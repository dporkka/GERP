package resource

import (
	"errors"
	"testing"
)

func TestResourceCommandsOnlyRequiresActions(t *testing.T) {
	resource := Resource{
		Name:         "journal_entries",
		Module:       "finance",
		MutationMode: MutationModeCommandsOnly,
	}
	if err := resource.Validate(); err == nil {
		t.Fatal("expected commands-only resource without actions to fail")
	}
}

func TestResourceCommandsOnlyWithExplicitAction(t *testing.T) {
	resource := Resource{
		Name:         "journal_entries",
		Module:       "finance",
		MutationMode: MutationModeCommandsOnly,
		Actions: []Action{
			{Name: "post", Command: "finance.post_journal", Permission: "finance.journal.post"},
		},
	}
	if err := resource.Validate(); err != nil {
		t.Fatalf("validate resource: %v", err)
	}
}

func TestRegistryRejectsDuplicateResource(t *testing.T) {
	registry := NewRegistry()
	resource := Resource{Name: "contacts", Module: "crm", MutationMode: MutationModeCRUD}
	if err := registry.Register(resource); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := registry.Register(resource); !errors.Is(err, ErrDuplicateResource) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}
