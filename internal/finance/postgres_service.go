package finance

import (
	"context"
	"time"

	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type JournalService struct {
	uow      *postgres.UnitOfWork
	repo     PostgresRepository
	recorder platformevents.Recorder
}

func NewJournalService(uow *postgres.UnitOfWork, repo PostgresRepository, recorder platformevents.Recorder) *JournalService {
	return &JournalService{uow: uow, repo: repo, recorder: recorder}
}

func (s *JournalService) CreateDraft(ctx context.Context, draft DraftJournal) error {
	return s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.CreateDraft(ctx, tx, draft); err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "finance.journal.create_draft", ResourceType: "journal_entry", ResourceID: draft.ID.String()},
			platformevents.Outbox{Topic: "finance.journal.draft_created", AggregateType: "journal_entry", AggregateID: draft.ID.String(), Payload: map[string]any{"number": draft.Number}},
		)
	})
}

func (s *JournalService) Post(ctx context.Context, journalID uuid.UUID) error {
	return s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.Post(ctx, tx, journalID); err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "finance.journal.post", ResourceType: "journal_entry", ResourceID: journalID.String()},
			platformevents.Outbox{Topic: "finance.journal.posted", AggregateType: "journal_entry", AggregateID: journalID.String()},
		)
	})
}

func (s *JournalService) Reverse(ctx context.Context, journalID, reversalID uuid.UUID, reversalNumber string, effectiveDate time.Time) error {
	return s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.Reverse(ctx, tx, journalID, reversalID, reversalNumber, effectiveDate); err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{
				Action:       "finance.journal.reverse",
				ResourceType: "journal_entry",
				ResourceID:   reversalID.String(),
				Metadata:     map[string]any{"reversal_of": journalID.String()},
			},
			platformevents.Outbox{
				Topic:         "finance.journal.reversed",
				AggregateType: "journal_entry",
				AggregateID:   reversalID.String(),
				Payload:       map[string]any{"reversal_of": journalID.String()},
			},
		)
	})
}
