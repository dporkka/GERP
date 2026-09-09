package goblcompliance

import (
	"errors"
	"testing"
)

func TestValidateRejectsNilEnvelope(t *testing.T) {
	service := New()
	if err := service.Validate(nil); !errors.Is(err, ErrNilEnvelope) {
		t.Fatalf("expected nil envelope error, got %v", err)
	}
}

func TestPrepareRejectsNilDocument(t *testing.T) {
	service := New()
	if _, err := service.Prepare(nil); err == nil {
		t.Fatal("expected nil document error")
	}
}
