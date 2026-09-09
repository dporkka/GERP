package admin

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"gerp/internal/platform/resource"
)

func TestGenerateDerivesPermissionsMenuTablesAndSafeForms(t *testing.T) {
	resources := resource.CoreResources()
	model, err := Generate(resources)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Menu) != len(resources) {
		t.Fatalf("menu entries=%d resources=%d", len(model.Menu), len(resources))
	}
	if _, ok := model.Forms["contacts"]; !ok {
		t.Fatal("CRUD contacts should have a generated form")
	}
	for _, commandsOnly := range []string{"deals", "journal_entries", "sales_quotes", "sales_orders", "invoices"} {
		if _, ok := model.Forms[commandsOnly]; ok {
			t.Fatalf("commands-only resource %s must not have a generic form", commandsOnly)
		}
	}
	codes := strings.Join(PermissionCodes(model), "\n")
	for _, required := range []string{"crm.contact.create", "finance.journal.post", "sales.quote.create", "sales.invoice.issue"} {
		if !strings.Contains(codes, required) {
			t.Fatalf("missing generated permission %s", required)
		}
	}
}

func TestRendererEmitsHTMXActionsAndEscapesData(t *testing.T) {
	var deals resource.Resource
	for _, definition := range resource.CoreResources() {
		if definition.Name == "deals" {
			deals = definition
			break
		}
	}
	var output bytes.Buffer
	component := NewRenderer().ResourceTable(deals, []map[string]any{{"id": "deal-1", "name": `<script>alert(1)</script>`, "amount_minor": 100, "status": "open"}})
	if err := component.Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	html := output.String()
	if strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("row values were not escaped: %s", html)
	}
	if !strings.Contains(html, `hx-post="/admin/deals/deal-1/actions/archive"`) {
		t.Fatalf("missing HTMX archive action: %s", html)
	}
	if !strings.Contains(html, `hx-confirm="Are you sure?"`) {
		t.Fatalf("dangerous action lacks confirmation: %s", html)
	}
}

func TestRendererRejectsGenericFormForCommandResource(t *testing.T) {
	for _, definition := range resource.CoreResources() {
		if definition.Name != "journal_entries" {
			continue
		}
		var output bytes.Buffer
		err := NewRenderer().ResourceForm(definition, nil, nil).Render(context.Background(), &output)
		if err == nil {
			t.Fatal("expected generic journal form to be rejected")
		}
		return
	}
	t.Fatal("journal resource not found")
}
