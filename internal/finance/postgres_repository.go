package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrJournalNotFound = errors.New("journal not found")
	ErrJournalState    = errors.New("journal is not in the required state")
)

type PostgresRepository struct{}

func NewPostgresRepository() PostgresRepository { return PostgresRepository{} }

func (PostgresRepository) CreateDraft(ctx context.Context, tx pgx.Tx, draft DraftJournal) error {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	if err := draft.Validate(); err != nil {
		return err
	}

	var reversal any
	if draft.ReversalOf != nil {
		reversal = *draft.ReversalOf
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO journal_entries
			(id, tenant_id, number, effective_date, currency, description, source_type, source_id, status, reversal_of)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), 'draft', $9)`,
		draft.ID, scope.TenantID, draft.Number, draft.EffectiveDate, draft.Currency,
		draft.Description, draft.SourceType, draft.SourceID, reversal,
	); err != nil {
		return fmt.Errorf("insert journal draft: %w", err)
	}

	for _, line := range draft.Lines {
		dimensions, err := json.Marshal(line.DimensionValues)
		if err != nil {
			return fmt.Errorf("marshal journal dimensions: %w", err)
		}
		if line.DimensionValues == nil {
			dimensions = []byte(`{}`)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO journal_lines
				(id, tenant_id, journal_entry_id, account_id, debit_minor, credit_minor, memo, dimension_values)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)`,
			line.ID, scope.TenantID, draft.ID, line.AccountID, line.DebitMinor, line.CreditMinor,
			line.Memo, string(dimensions),
		); err != nil {
			return fmt.Errorf("insert journal line: %w", err)
		}
	}
	return nil
}

func (PostgresRepository) Post(ctx context.Context, tx pgx.Tx, journalID uuid.UUID) error {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	var actor any
	if scope.ActorUserID != uuid.Nil {
		actor = scope.ActorUserID
	}
	command, err := tx.Exec(ctx, `
		UPDATE journal_entries
		   SET status = 'posted', posted_by = $3
		 WHERE tenant_id = $1 AND id = $2 AND status = 'draft'`,
		scope.TenantID, journalID, actor,
	)
	if err != nil {
		return fmt.Errorf("post journal: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrJournalState
	}
	return nil
}

func (repository PostgresRepository) Reverse(
	ctx context.Context,
	tx pgx.Tx,
	journalID uuid.UUID,
	reversalID uuid.UUID,
	reversalNumber string,
	effectiveDate time.Time,
) error {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	if reversalID == uuid.Nil || reversalNumber == "" || effectiveDate.IsZero() {
		return fmt.Errorf("reversal id, number and effective date are required")
	}

	var currency string
	var description string
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT currency, description, status
		  FROM journal_entries
		 WHERE tenant_id = $1 AND id = $2
		 FOR UPDATE`, scope.TenantID, journalID).Scan(&currency, &description, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJournalNotFound
		}
		return fmt.Errorf("load journal for reversal: %w", err)
	}
	if status != "posted" {
		return ErrJournalState
	}

	rows, err := tx.Query(ctx, `
		SELECT id, account_id, debit_minor, credit_minor, memo, dimension_values
		  FROM journal_lines
		 WHERE tenant_id = $1 AND journal_entry_id = $2
		 ORDER BY id`, scope.TenantID, journalID)
	if err != nil {
		return fmt.Errorf("load journal lines for reversal: %w", err)
	}
	defer rows.Close()

	lines := make([]JournalLine, 0)
	for rows.Next() {
		var originalID uuid.UUID
		var line JournalLine
		var dimensions []byte
		if err := rows.Scan(&originalID, &line.AccountID, &line.DebitMinor, &line.CreditMinor, &line.Memo, &dimensions); err != nil {
			return fmt.Errorf("scan journal line for reversal: %w", err)
		}
		line.ID = uuid.New()
		line.DebitMinor, line.CreditMinor = line.CreditMinor, line.DebitMinor
		if err := json.Unmarshal(dimensions, &line.DimensionValues); err != nil {
			return fmt.Errorf("decode journal dimensions: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate journal lines for reversal: %w", err)
	}

	reversalOf := journalID
	draft := DraftJournal{
		ID:            reversalID,
		Number:        reversalNumber,
		EffectiveDate: effectiveDate,
		Currency:      currency,
		Description:   "Reversal: " + description,
		SourceType:    "journal_reversal",
		SourceID:      journalID.String(),
		ReversalOf:    &reversalOf,
		Lines:         lines,
	}
	if err := repository.CreateDraft(ctx, tx, draft); err != nil {
		return err
	}
	return repository.Post(ctx, tx, reversalID)
}

func (PostgresRepository) Get(ctx context.Context, tx pgx.Tx, journalID uuid.UUID) (PostedJournal, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return PostedJournal{}, err
	}

	var journal PostedJournal
	var postedAt *time.Time
	var reversalText string
	if err := tx.QueryRow(ctx, `
		SELECT id, tenant_id, number, effective_date, currency, description, status,
		       posted_at, COALESCE(reversal_of::text, '')
		  FROM journal_entries
		 WHERE tenant_id = $1 AND id = $2`, scope.TenantID, journalID).Scan(
		&journal.ID, &journal.TenantID, &journal.Number, &journal.EffectiveDate, &journal.Currency,
		&journal.Description, &journal.Status, &postedAt, &reversalText,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PostedJournal{}, ErrJournalNotFound
		}
		return PostedJournal{}, fmt.Errorf("get journal: %w", err)
	}
	journal.PostedAt = postedAt
	if reversalText != "" {
		id, err := uuid.Parse(reversalText)
		if err != nil {
			return PostedJournal{}, fmt.Errorf("parse reversal id: %w", err)
		}
		journal.ReversalOf = &id
	}

	rows, err := tx.Query(ctx, `
		SELECT id, account_id, debit_minor, credit_minor, memo, dimension_values
		  FROM journal_lines
		 WHERE tenant_id = $1 AND journal_entry_id = $2
		 ORDER BY id`, scope.TenantID, journalID)
	if err != nil {
		return PostedJournal{}, fmt.Errorf("get journal lines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var line JournalLine
		var dimensions []byte
		if err := rows.Scan(&line.ID, &line.AccountID, &line.DebitMinor, &line.CreditMinor, &line.Memo, &dimensions); err != nil {
			return PostedJournal{}, fmt.Errorf("scan journal line: %w", err)
		}
		if err := json.Unmarshal(dimensions, &line.DimensionValues); err != nil {
			return PostedJournal{}, fmt.Errorf("decode journal dimensions: %w", err)
		}
		journal.Lines = append(journal.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return PostedJournal{}, fmt.Errorf("iterate journal lines: %w", err)
	}
	return journal, nil
}
