package sales_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"
	"gerp/internal/platform/tenant"
	"gerp/internal/sales"

	"github.com/google/uuid"
)

func TestQuoteToOrderToInvoiceConversion(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, slug, name, base_currency) VALUES ($1, $2, 'Sales test', 'USD')`, tenantID, "sales-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'Sales tester')`, actorID, fmt.Sprintf("%s@example.test", actorID)); err != nil {
		t.Fatal(err)
	}
	scoped, err := tenant.WithScope(ctx, tenant.Scope{TenantID: tenantID, ActorUserID: actorID})
	if err != nil {
		t.Fatal(err)
	}
	service := sales.NewService(postgres.NewUnitOfWork(pool), sales.NewPostgresRepository(), platformevents.NewRecorder())

	quoteID := uuid.New()
	quote := sales.QuoteDraft{
		ID:       quoteID,
		Number:   "Q-1001",
		Currency: "USD",
		Lines: []sales.QuoteLine{
			{ID: uuid.New(), Position: 1, Description: "Implementation", Quantity: "2", UnitPriceMinor: 50000, LineTotalMinor: 100000},
			{ID: uuid.New(), Position: 2, Description: "Support", Quantity: "1", UnitPriceMinor: 25000, LineTotalMinor: 25000},
		},
	}
	revision, err := service.CreateQuote(scoped, quote)
	if err != nil {
		t.Fatal(err)
	}
	revision, state, err := service.TransitionQuote(scoped, quoteID, revision, "send")
	if err != nil {
		t.Fatal(err)
	}
	if state != "sent" {
		t.Fatalf("state=%s, want sent", state)
	}
	revision, state, err = service.TransitionQuote(scoped, quoteID, revision, "accept")
	if err != nil {
		t.Fatal(err)
	}
	if state != "accepted" {
		t.Fatalf("state=%s, want accepted", state)
	}

	order, err := service.CreateOrderFromQuote(scoped, quoteID, revision, uuid.New(), "SO-1001")
	if err != nil {
		t.Fatal(err)
	}
	if order.TotalMinor != 125000 || order.Status != "confirmed" {
		t.Fatalf("unexpected order: %#v", order)
	}
	invoice, err := service.CreateInvoiceFromOrder(scoped, order.ID, uuid.New(), "INV-1001")
	if err != nil {
		t.Fatal(err)
	}
	if invoice.TotalMinor != order.TotalMinor || invoice.Status != "draft" {
		t.Fatalf("unexpected invoice: %#v", invoice)
	}

	var quoteLines, orderLines, invoiceLines int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sales_quote_lines WHERE tenant_id = $1 AND quote_id = $2`, tenantID, quoteID).Scan(&quoteLines); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sales_order_lines WHERE tenant_id = $1 AND order_id = $2`, tenantID, order.ID).Scan(&orderLines); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ar_invoice_lines WHERE tenant_id = $1 AND invoice_id = $2`, tenantID, invoice.ID).Scan(&invoiceLines); err != nil {
		t.Fatal(err)
	}
	if quoteLines != 2 || orderLines != 2 || invoiceLines != 2 {
		t.Fatalf("line history not preserved: quote=%d order=%d invoice=%d", quoteLines, orderLines, invoiceLines)
	}
}
