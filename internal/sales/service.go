package sales

import (
	"context"

	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	uow      *postgres.UnitOfWork
	repo     PostgresRepository
	recorder platformevents.Recorder
}

func NewService(uow *postgres.UnitOfWork, repo PostgresRepository, recorder platformevents.Recorder) *Service {
	return &Service{uow: uow, repo: repo, recorder: recorder}
}

func (s *Service) CreateQuote(ctx context.Context, quote QuoteDraft) (int64, error) {
	var revision int64
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		revision, err = s.repo.CreateQuote(ctx, tx, quote)
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "sales.quote.create", ResourceType: "sales_quote", ResourceID: quote.ID.String()},
			platformevents.Outbox{Topic: "sales.quote.created", AggregateType: "sales_quote", AggregateID: quote.ID.String(), Payload: map[string]any{"revision": revision}},
		)
	})
	return revision, err
}

func (s *Service) TransitionQuote(ctx context.Context, quoteID uuid.UUID, expectedRevision int64, transition string) (int64, string, error) {
	var revision int64
	var state string
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		revision, state, err = s.repo.TransitionQuote(ctx, tx, quoteID, expectedRevision, transition)
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "sales.quote." + transition, ResourceType: "sales_quote", ResourceID: quoteID.String(), Metadata: map[string]any{"revision": revision, "state": state}},
			platformevents.Outbox{Topic: "sales.quote." + state, AggregateType: "sales_quote", AggregateID: quoteID.String(), Payload: map[string]any{"revision": revision}},
		)
	})
	return revision, state, err
}

func (s *Service) CreateOrderFromQuote(ctx context.Context, quoteID uuid.UUID, expectedQuoteRevision int64, orderID uuid.UUID, orderNumber string) (Order, error) {
	var order Order
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		order, err = s.repo.CreateOrderFromAcceptedQuote(ctx, tx, quoteID, expectedQuoteRevision, orderID, orderNumber)
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "sales.order.create_from_quote", ResourceType: "sales_order", ResourceID: order.ID.String(), Metadata: map[string]any{"quote_id": quoteID}},
			platformevents.Outbox{Topic: "sales.order.confirmed", AggregateType: "sales_order", AggregateID: order.ID.String(), Payload: map[string]any{"quote_id": quoteID}},
		)
	})
	return order, err
}

func (s *Service) CreateInvoiceFromOrder(ctx context.Context, orderID, invoiceID uuid.UUID, invoiceNumber string) (Invoice, error) {
	var invoice Invoice
	err := s.uow.Within(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		invoice, err = s.repo.CreateInvoiceFromOrder(ctx, tx, orderID, invoiceID, invoiceNumber)
		if err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx,
			platformevents.Audit{Action: "sales.invoice.create_from_order", ResourceType: "invoice", ResourceID: invoice.ID.String(), Metadata: map[string]any{"order_id": orderID}},
			platformevents.Outbox{Topic: "sales.invoice.draft_created", AggregateType: "invoice", AggregateID: invoice.ID.String(), Payload: map[string]any{"order_id": orderID}},
		)
	})
	return invoice, err
}
