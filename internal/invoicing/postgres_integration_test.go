package invoicing_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goblcompliance "gerp/internal/compliance/gobl"
	"gerp/internal/invoicing"
	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"
	"gerp/internal/platform/tenant"
	"gerp/internal/sales"

	"github.com/google/uuid"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
)

func TestIssuanceRequiresValidatedImmutableGOBLSnapshot(t *testing.T) {
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
	quoteID := uuid.New()
	orderID := uuid.New()
	invoiceID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, slug, name, base_currency) VALUES ($1, $2, 'Invoice test', 'USD')`, tenantID, "invoice-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'Invoice tester')`, actorID, fmt.Sprintf("%s@example.test", actorID)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sales_quotes (id, tenant_id, number, currency, status, total_minor, revision) VALUES ($1, $2, 'Q-SNAP', 'USD', 'accepted', 10000, 2)`, quoteID, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sales_orders (id, tenant_id, number, quote_id, currency, status, total_minor, revision) VALUES ($1, $2, 'SO-SNAP', $3, 'USD', 'confirmed', 10000, 1)`, orderID, tenantID, quoteID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ar_invoices (id, tenant_id, number, order_id, currency, status, total_minor, revision) VALUES ($1, $2, 'INV-SNAP', $3, 'USD', 'draft', 10000, 1)`, invoiceID, tenantID, orderID); err != nil {
		t.Fatal(err)
	}

	scoped, err := tenant.WithScope(ctx, tenant.Scope{TenantID: tenantID, ActorUserID: actorID})
	if err != nil {
		t.Fatal(err)
	}
	compliance := goblcompliance.New()
	service := invoicing.NewService(
		postgres.NewUnitOfWork(pool),
		sales.NewPostgresRepository(),
		goblcompliance.NewSnapshotStore(compliance),
		platformevents.NewRecorder(),
	)

	if _, _, err := service.Issue(scoped, invoiceID, 1, nil); err == nil {
		t.Fatal("expected nil GOBL envelope to reject issuance")
	}
	var status string
	var snapshots int
	if err := pool.QueryRow(ctx, `SELECT status FROM ar_invoices WHERE tenant_id = $1 AND id = $2`, tenantID, invoiceID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM invoice_document_snapshots WHERE tenant_id = $1 AND invoice_id = $2`, tenantID, invoiceID).Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || snapshots != 0 {
		t.Fatalf("failed issuance committed state: status=%s snapshots=%d", status, snapshots)
	}

	price := num.MakeAmount(10000, 2)
	document := &bill.Invoice{
		Code:      "INV-SNAP",
		IssueDate: cal.MakeDate(2026, time.September, 9),
		Currency:  "USD",
		Supplier:  &org.Party{Name: "GERP Test Supplier"},
		Customer:  &org.Party{Name: "GERP Test Customer"},
		Lines: []*bill.Line{
			{
				Quantity: num.MakeAmount(1, 0),
				Item: &org.Item{
					Name:  "ERP implementation",
					Price: &price,
				},
			},
		},
	}
	envelope, err := compliance.Prepare(document)
	if err != nil {
		t.Fatalf("prepare valid GOBL invoice: %v", err)
	}
	revision, snapshot, err := service.Issue(scoped, invoiceID, 1, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if revision != 2 || snapshot.ID == uuid.Nil || len(snapshot.Payload) == 0 {
		t.Fatalf("unexpected issuance result: revision=%d snapshot=%#v", revision, snapshot)
	}
	if _, err := pool.Exec(ctx, `UPDATE invoice_document_snapshots SET media_type = 'text/plain' WHERE id = $1`, snapshot.ID); err == nil {
		t.Fatal("expected PostgreSQL to reject invoice snapshot mutation")
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM ar_invoices WHERE tenant_id = $1 AND id = $2`, tenantID, invoiceID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "issued" {
		t.Fatalf("status=%s, want issued", status)
	}
}
