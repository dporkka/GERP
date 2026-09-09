package finance

import (
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
)

func line(amount int64) *LineItem {
	return &LineItem{
		LineItemID:  uuid.New(),
		AccountID:   uuid.New(),
		AmountCents: amount,
	}
}

func TestValidateJournalAcceptsBalancedEntry(t *testing.T) {
	if err := ValidateJournal([]*LineItem{line(12_500), line(-12_500)}); err != nil {
		t.Fatalf("ValidateJournal() error = %v", err)
	}
}

func TestValidateJournalRejectsUnbalancedEntry(t *testing.T) {
	err := ValidateJournal([]*LineItem{line(12_500), line(-12_000)})
	if !errors.Is(err, ErrJournalUnbalanced) {
		t.Fatalf("expected ErrJournalUnbalanced, got %v", err)
	}
}

func TestValidateJournalRejectsDegenerateEntries(t *testing.T) {
	t.Run("too few lines", func(t *testing.T) {
		if err := ValidateJournal([]*LineItem{line(100)}); !errors.Is(err, ErrJournalTooFewLines) {
			t.Fatalf("expected ErrJournalTooFewLines, got %v", err)
		}
	})

	t.Run("zero line", func(t *testing.T) {
		if err := ValidateJournal([]*LineItem{line(0), line(1), line(-1)}); !errors.Is(err, ErrJournalZeroLine) {
			t.Fatalf("expected ErrJournalZeroLine, got %v", err)
		}
	})

	t.Run("nil line", func(t *testing.T) {
		if err := ValidateJournal([]*LineItem{nil, line(1)}); err == nil {
			t.Fatal("expected nil line to fail")
		}
	})
}

func TestValidateJournalRejectsDuplicateLineIDs(t *testing.T) {
	id := uuid.New()
	first := line(100)
	second := line(-100)
	first.LineItemID = id
	second.LineItemID = id
	if err := ValidateJournal([]*LineItem{first, second}); err == nil {
		t.Fatal("expected duplicate line ID to fail")
	}
}

func TestValidateJournalRejectsOverflow(t *testing.T) {
	if err := ValidateJournal([]*LineItem{line(math.MaxInt64), line(1), line(-1)}); !errors.Is(err, ErrJournalOverflow) {
		t.Fatalf("expected ErrJournalOverflow, got %v", err)
	}
}
