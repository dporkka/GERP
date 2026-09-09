package crm

import "testing"

func TestDealLifecycle(t *testing.T) {
	if err := ValidateWorkflows(); err != nil {
		t.Fatalf("validate CRM workflows: %v", err)
	}

	state, err := DealLifecycle.Apply("open", "win")
	if err != nil {
		t.Fatalf("win deal: %v", err)
	}
	if state != "won" {
		t.Fatalf("unexpected deal state: %s", state)
	}

	if _, err := DealLifecycle.Apply("won", "reopen"); err == nil {
		t.Fatal("won deal should not reopen through the lost-deal transition")
	}
}
