package crm

import (
	"context"

	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CommandService struct {
	uow      *postgres.UnitOfWork
	repo     PostgresRepository
	recorder platformevents.Recorder
}

func NewCommandService(uow *postgres.UnitOfWork, repo PostgresRepository, recorder platformevents.Recorder) *CommandService {
	return &CommandService{uow: uow, repo: repo, recorder: recorder}
}

func (s *CommandService) MoveDeal(ctx context.Context, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID) (int64, error) {
	return s.runDealCommand(ctx, dealID, "move", "moved", func(ctx context.Context, tx pgx.Tx) (int64, error) {
		return s.repo.MoveDeal(ctx, tx, dealID, expectedRevision, stageID)
	})
}

func (s *CommandService) WinDeal(ctx context.Context, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID, reason string) (int64, error) {
	return s.runDealCommand(ctx, dealID, "win", "won", func(ctx context.Context, tx pgx.Tx) (int64, error) {
		return s.repo.CloseDeal(ctx, tx, dealID, expectedRevision, stageID, "won", reason)
	})
}

func (s *CommandService) LoseDeal(ctx context.Context, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID, reason string) (int64, error) {
	return s.runDealCommand(ctx, dealID, "lose", "lost", func(ctx context.Context, tx pgx.Tx) (int64, error) {
		return s.repo.CloseDeal(ctx, tx, dealID, expectedRevision, stageID, "lost", reason)
	})
}

func (s *CommandService) ReopenDeal(ctx context.Context, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID) (int64, error) {
	return s.runDealCommand(ctx, dealID, "reopen", "reopened", func(ctx context.Context, tx pgx.Tx) (int64, error) {
		return s.repo.ReopenDeal(ctx, tx, dealID, expectedRevision, stageID)
	})
}

func (s *CommandService) ArchiveDeal(ctx context.Context, dealID uuid.UUID, expectedRevision int64) (int64, error) {
	return s.runDealCommand(ctx, dealID, "archive", "archived", func(ctx context.Context, tx pgx.Tx) (int64, error) {
		return s.repo.SetArchived(ctx, tx, dealID, expectedRevision, true)
	})
}

func (s *CommandService) RestoreDeal(ctx context.Context, dealID uuid.UUID, expectedRevision int64) (int64, error) {
	return s.runDealCommand(ctx, dealID, "restore", "restored", func(ctx context.Context, tx pgx.Tx) (int64, error) {
		return s.repo.SetArchived(ctx, tx, dealID, expectedRevision, false)
	})
}

type dealCommand func(context.Context, pgx.Tx) (int64, error)

func (s *CommandService) runDealCommand(ctx context.Context, dealID uuid.UUID, action, topicSuffix string, command dealCommand) (int64, error) {
	var revision int64
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		revision, err = command(ctx, tx)
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{
				Action:       "crm.deal." + action,
				ResourceType: "deal",
				ResourceID:   dealID.String(),
				Metadata:     map[string]any{"revision": revision},
			},
			platformevents.Outbox{
				Topic:         "crm.deal." + topicSuffix,
				AggregateType: "deal",
				AggregateID:   dealID.String(),
				Payload:       map[string]any{"revision": revision},
			},
		)
	})
	return revision, err
}
