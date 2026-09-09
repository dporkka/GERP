package sales

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrQuoteNotFound         = errors.New("quote not found")
	ErrQuoteRevisionConflict = errors.New("quote revision conflict")
	ErrQuoteState            = errors.New("quote is not in the required state")
	ErrOrderState            = errors.New("order is not in the required state")
	ErrInvoiceState          = errors.New("invoice is not in the required state")
)

type PostgresRepository struct{}

func NewPostgresRepository() PostgresRepository { return PostgresRepository{} }

func (PostgresRepository) CreateQuote(ctx context.Context, tx pgx.Tx, quote QuoteDraft) (int64, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, err
	}
	if err := quote.Validate(); err != nil {
		return 0, err
	}
	var company any
	if quote.CompanyID != nil {
		company = *quote.CompanyID
	}
	var validUntil any
	if quote.ValidUntil != nil {
		validUntil = quote.ValidUntil.UTC()
	}
	var total int64
	for _, line := range quote.Lines {
		if line.LineTotalMinor > 0 && total > (1<<63-1)-line.LineTotalMinor {
			return 0, fmt.Errorf("quote total overflow")
		}
		total += line.LineTotalMinor
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sales_quotes
			(id, tenant_id, number, company_id, currency, valid_until, total_minor, status, revision)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', 1)`,
		quote.ID, scope.TenantID, quote.Number, company, quote.Currency, validUntil, total); err != nil {
		return 0, fmt.Errorf("insert quote: %w", err)
	}
	for _, line := range quote.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sales_quote_lines
				(id, tenant_id, quote_id, position, description, quantity, unit_price_minor, line_total_minor, tax_code)
			VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8, NULLIF($9, ''))`,
			line.ID, scope.TenantID, quote.ID, line.Position, line.Description, line.Quantity,
			line.UnitPriceMinor, line.LineTotalMinor, line.TaxCode); err != nil {
			return 0, fmt.Errorf("insert quote line: %w", err)
		}
	}
	return 1, nil
}

func (repository PostgresRepository) TransitionQuote(ctx context.Context, tx pgx.Tx, quoteID uuid.UUID, expectedRevision int64, transition string) (int64, string, error) {
	definition := QuoteWorkflow()
	var currentState string
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, "", err
	}
	if err := tx.QueryRow(ctx, `SELECT status FROM sales_quotes WHERE tenant_id = $1 AND id = $2`, scope.TenantID, quoteID).Scan(&currentState); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrQuoteNotFound
		}
		return 0, "", fmt.Errorf("load quote state: %w", err)
	}
	nextState, err := definition.Apply(currentState, transition)
	if err != nil {
		return 0, currentState, ErrQuoteState
	}

	var revision int64
	var query string
	switch nextState {
	case "sent":
		query = `UPDATE sales_quotes SET status = $4, revision = revision + 1, sent_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND status = $5 RETURNING revision`
	case "accepted":
		query = `UPDATE sales_quotes SET status = $4, revision = revision + 1, accepted_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND status = $5 RETURNING revision`
	default:
		query = `UPDATE sales_quotes SET status = $4, revision = revision + 1, updated_at = now() WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND status = $5 RETURNING revision`
	}
	if err := tx.QueryRow(ctx, query, scope.TenantID, quoteID, expectedRevision, nextState, currentState).Scan(&revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, currentState, repository.classifyQuoteWriteFailure(ctx, tx, quoteID, expectedRevision)
		}
		return 0, currentState, fmt.Errorf("transition quote: %w", err)
	}
	return revision, nextState, nil
}

func (PostgresRepository) CreateOrderFromAcceptedQuote(ctx context.Context, tx pgx.Tx, quoteID uuid.UUID, expectedQuoteRevision int64, orderID uuid.UUID, orderNumber string) (Order, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return Order{}, err
	}
	if orderID == uuid.Nil || orderNumber == "" {
		return Order{}, fmt.Errorf("order id and number are required")
	}
	var currency, status string
	var total int64
	var revision int64
	if err := tx.QueryRow(ctx, `
		SELECT currency, status, total_minor, revision
		  FROM sales_quotes
		 WHERE tenant_id = $1 AND id = $2
		 FOR UPDATE`, scope.TenantID, quoteID).Scan(&currency, &status, &total, &revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrQuoteNotFound
		}
		return Order{}, fmt.Errorf("load quote for order: %w", err)
	}
	if revision != expectedQuoteRevision {
		return Order{}, fmt.Errorf("%w: expected %d, current %d", ErrQuoteRevisionConflict, expectedQuoteRevision, revision)
	}
	if status != "accepted" {
		return Order{}, ErrQuoteState
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sales_orders (id, tenant_id, number, quote_id, currency, status, total_minor, revision)
		VALUES ($1, $2, $3, $4, $5, 'confirmed', $6, 1)`, orderID, scope.TenantID, orderNumber, quoteID, currency, total); err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sales_order_lines
			(id, tenant_id, order_id, source_quote_line_id, position, description, quantity, unit_price_minor, line_total_minor, tax_code)
		SELECT gen_random_uuid(), tenant_id, $3, id, position, description, quantity, unit_price_minor, line_total_minor, tax_code
		  FROM sales_quote_lines
		 WHERE tenant_id = $1 AND quote_id = $2
		 ORDER BY position`, scope.TenantID, quoteID, orderID); err != nil {
		return Order{}, fmt.Errorf("copy quote lines to order: %w", err)
	}
	return Order{ID: orderID, QuoteID: quoteID, Number: orderNumber, Currency: currency, Status: "confirmed", TotalMinor: total, Revision: 1}, nil
}

func (PostgresRepository) CreateInvoiceFromOrder(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, invoiceID uuid.UUID, invoiceNumber string) (Invoice, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return Invoice{}, err
	}
	if invoiceID == uuid.Nil || invoiceNumber == "" {
		return Invoice{}, fmt.Errorf("invoice id and number are required")
	}
	var currency, status string
	var total int64
	if err := tx.QueryRow(ctx, `
		SELECT currency, status, total_minor
		  FROM sales_orders
		 WHERE tenant_id = $1 AND id = $2
		 FOR UPDATE`, scope.TenantID, orderID).Scan(&currency, &status, &total); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invoice{}, ErrOrderState
		}
		return Invoice{}, fmt.Errorf("load order for invoice: %w", err)
	}
	if status != "confirmed" && status != "fulfilled" {
		return Invoice{}, ErrOrderState
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ar_invoices (id, tenant_id, number, order_id, currency, status, total_minor, revision)
		VALUES ($1, $2, $3, $4, $5, 'draft', $6, 1)`, invoiceID, scope.TenantID, invoiceNumber, orderID, currency, total); err != nil {
		return Invoice{}, fmt.Errorf("insert invoice: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ar_invoice_lines
			(id, tenant_id, invoice_id, source_order_line_id, position, description, quantity, unit_price_minor, line_total_minor, tax_code)
		SELECT gen_random_uuid(), tenant_id, $3, id, position, description, quantity, unit_price_minor, line_total_minor, tax_code
		  FROM sales_order_lines
		 WHERE tenant_id = $1 AND order_id = $2
		 ORDER BY position`, scope.TenantID, orderID, invoiceID); err != nil {
		return Invoice{}, fmt.Errorf("copy order lines to invoice: %w", err)
	}
	return Invoice{ID: invoiceID, OrderID: orderID, Number: invoiceNumber, Currency: currency, Status: "draft", TotalMinor: total, Revision: 1}, nil
}

func (PostgresRepository) TransitionInvoice(ctx context.Context, tx pgx.Tx, invoiceID uuid.UUID, expectedRevision int64, fromState, toState string) (int64, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, err
	}
	if next, err := InvoiceWorkflow().Apply(fromState, transitionName(fromState, toState)); err != nil || next != toState {
		return 0, ErrInvoiceState
	}
	var revision int64
	var query string
	switch toState {
	case "issued":
		query = `UPDATE ar_invoices SET status = 'issued', revision = revision + 1, issue_date = COALESCE(issue_date, CURRENT_DATE), issued_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND status = $4 RETURNING revision`
	case "paid":
		query = `UPDATE ar_invoices SET status = 'paid', revision = revision + 1, paid_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND status = $4 RETURNING revision`
	case "void":
		query = `UPDATE ar_invoices SET status = 'void', revision = revision + 1, voided_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND status = $4 RETURNING revision`
	default:
		return 0, ErrInvoiceState
	}
	if err := tx.QueryRow(ctx, query, scope.TenantID, invoiceID, expectedRevision, fromState).Scan(&revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrInvoiceState
		}
		return 0, fmt.Errorf("transition invoice: %w", err)
	}
	return revision, nil
}

func (PostgresRepository) classifyQuoteWriteFailure(ctx context.Context, tx pgx.Tx, quoteID uuid.UUID, expectedRevision int64) error {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	var current int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM sales_quotes WHERE tenant_id = $1 AND id = $2`, scope.TenantID, quoteID).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrQuoteNotFound
		}
		return fmt.Errorf("classify quote write failure: %w", err)
	}
	if current != expectedRevision {
		return fmt.Errorf("%w: expected %d, current %d", ErrQuoteRevisionConflict, expectedRevision, current)
	}
	return ErrQuoteState
}

func transitionName(fromState, toState string) string {
	switch fromState + ">" + toState {
	case "draft>issued":
		return "issue"
	case "issued>paid":
		return "mark_paid"
	case "issued>void":
		return "void"
	default:
		return ""
	}
}

var _ = time.Time{}
