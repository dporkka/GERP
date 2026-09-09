package finance_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"gerp/internal/finance"
	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"
	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestJournalPostingAndReversalPostgres(t *testing.T) {
	url := os.Getenv("GERP_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("GERP_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := postgres.Open(ctx, postgres.Config{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	tenantID := uuid.New()
	actorID := uuid.New()
	debitAccount := uuid.New()
	creditAccount := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, slug, name, base_currency) VALUES ($1, $2, 'Finance test', 'USD')`, tenantID, "finance-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'Finance tester')`, actorID, fmt.Sprintf("%s@example.test", actorID)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO gl_accounts (id, tenant_id, code, name, account_type, normal_balance)
		VALUES ($1, $2, '1000', 'Cash', 'asset', 'debit'),
		       ($3, $2, '4000', 'Revenue', 'revenue', 'credit')`,
		debitAccount, tenantID, creditAccount,
	); err != nil {
		t.Fatal(err)
	}

	scoped, err := tenant.WithScope(ctx, tenant.Scope{TenantID: tenantID, ActorUserID: actorID})
	if err != nil {
		t.Fatal(err)
	}
	uow := postgres.NewUnitOfWork(pool)
	repo := finance.NewPostgresRepository()
	service := finance.NewJournalService(uow, repo, platformevents.NewRecorder())

	journalID := uuid.New()
	draft := finance.DraftJournal{
		ID:            journalID,
		Number:        "JE-1001",
		EffectiveDate: time.Now().UTC(),
		Currency:      "USD",
		Description:   "Integration posting",
		Lines: []finance.JournalLine{
			{ID: uuid.New(), AccountID: debitAccount, DebitMinor: 12500},
			{ID: uuid.New(), AccountID: creditAccount, CreditMinor: 12500},
		},
	}
	if err := service.CreateDraft(scoped, draft); err != nil {
		t.Fatal(err)
	}
	if err := service.Post(scoped, journalID); err != nil {
		t.Fatal(err)
	}

	var posted finance.PostedJournal
	if err := uow.Within(scoped, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		posted, err = repo.Get(ctx, tx, journalID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if posted.Status != "posted" || posted.PostedAt == nil {
		t.Fatalf("journal not posted: %#v", posted)
	}

	if _, err := pool.Exec(ctx, `UPDATE journal_lines SET memo = 'mutated' WHERE tenant_id = $1 AND journal_entry_id = $2`, tenantID, journalID); err == nil {
		t.Fatal("expected PostgreSQL to reject mutation of a posted journal line")
	}

	reversalID := uuid.New()
	if err := service.Reverse(scoped, journalID, reversalID, "JE-1001-R", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := uow.Within(scoped, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(ctx context.Context, tx pgx.Tx) error {
		reversal, err := repo.Get(ctx, tx, reversalID)
		if err != nil {
			return err
		}
		if reversal.Status != "posted" || reversal.ReversalOf == nil || *reversal.ReversalOf != journalID {
			return fmt.Errorf("invalid reversal: %#v", reversal)
		}
		if len(reversal.Lines) != 2 || reversal.Lines[0].DebitMinor+reversal.Lines[1].DebitMinor != 12500 || reversal.Lines[0].CreditMinor+reversal.Lines[1].CreditMinor != 12500 {
			return fmt.Errorf("reversal lines do not invert original: %#v", reversal.Lines)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var audits int
	var outbox int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE tenant_id = $1 AND resource_type = 'journal_entry'`, tenantID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE tenant_id = $1 AND aggregate_type = 'journal_entry'`, tenantID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if audits != 3 || outbox != 3 {
		t.Fatalf("expected 3 atomic audit/outbox records, got audits=%d outbox=%d", audits, outbox)
	}
}
