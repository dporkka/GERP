package workflow

import (
	"errors"
	"testing"
)

func TestDefinitionApply(t *testing.T) {
	definition := Definition{
		Name:    "quote",
		Initial: "draft",
		States:  []string{"draft", "sent", "accepted", "void"},
		Transitions: []Transition{
			{Name: "send", From: "draft", To: "sent"},
			{Name: "accept", From: "sent", To: "accepted"},
			{Name: "void", From: "draft", To: "void"},
			{Name: "void", From: "sent", To: "void"},
		},
	}
	if err := definition.Validate(); err != nil {
		t.Fatalf("validate workflow: %v", err)
	}

	state, err := definition.Apply("draft", "send")
	if err != nil {
		t.Fatalf("apply transition: %v", err)
	}
	if state != "sent" {
		t.Fatalf("unexpected state %q", state)
	}

	state, err = definition.Apply("sent", "send")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got state=%q err=%v", state, err)
	}
}

func TestDefinitionRejectsUnknownState(t *testing.T) {
	definition := Definition{
		Name:    "deal",
		Initial: "open",
		States:  []string{"open", "won"},
		Transitions: []Transition{
			{Name: "lose", From: "open", To: "lost"},
		},
	}
	if err := definition.Validate(); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected invalid state error, got %v", err)
	}
}
