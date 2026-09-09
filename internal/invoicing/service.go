package invoicing

import (
	"context"

	goblcompliance "gerp/internal/compliance/gobl"
	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"
	"gerp/internal/sales"

	"github.com/google/uuid"
	"github.com/invopop/gobl"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	uow       *postgres.UnitOfWork
	invoices  sales.PostgresRepository
	snapshots goblcompliance.SnapshotStore
	recorder  platformevents.Recorder
}

func NewService(uow *postgres.UnitOfWork, invoices sales.PostgresRepository, snapshots goblcompliance.SnapshotStore, recorder platformevents.Recorder) *Service {
	return &Service{uow: uow, invoices: invoices, snapshots: snapshots, recorder: recorder}
}

func (s *Service) Issue(ctx context.Context, invoiceID uuid.UUID, expectedRevision int64, envelope *gobl.Envelope) (int64, goblcompliance.Snapshot, error) {
	var revision int64
	var snapshot goblcompliance.Snapshot
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		snapshot, err = s.snapshots.SaveValidated(ctx, tx, invoiceID, envelope)
		if err != nil {
			return err
		}
		revision, err = s.invoices.TransitionInvoice(ctx, tx, invoiceID, expectedRevision, "draft", "issued")
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{
				Action:       "sales.invoice.issue",
				ResourceType: "invoice",
				ResourceID:   invoiceID.String(),
				Metadata:     map[string]any{"revision": revision, "snapshot_id": snapshot.ID, "sha256": snapshot.SHA256},
			},
			platformevents.Outbox{
				Topic:         "sales.invoice.issued",
				AggregateType: "invoice",
				AggregateID:   invoiceID.String(),
				Payload:       map[string]any{"revision": revision, "snapshot_id": snapshot.ID},
			},
		)
	})
	return revision, snapshot, err
}

func (s *Service) MarkPaid(ctx context.Context, invoiceID uuid.UUID, expectedRevision int64) (int64, error) {
	return s.transition(ctx, invoiceID, expectedRevision, "issued", "paid", "mark_paid")
}

func (s *Service) Void(ctx context.Context, invoiceID uuid.UUID, expectedRevision int64) (int64, error) {
	return s.transition(ctx, invoiceID, expectedRevision, "issued", "void", "void")
}

func (s *Service) transition(ctx context.Context, invoiceID uuid.UUID, expectedRevision int64, fromState, toState, action string) (int64, error) {
	var revision int64
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		revision, err = s.invoices.TransitionInvoice(ctx, tx, invoiceID, expectedRevision, fromState, toState)
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "sales.invoice." + action, ResourceType: "invoice", ResourceID: invoiceID.String(), Metadata: map[string]any{"revision": revision}},
			platformevents.Outbox{Topic: "sales.invoice." + toState, AggregateType: "invoice", AggregateID: invoiceID.String(), Payload: map[string]any{"revision": revision}},
		)
	})
	return revision, err
}
