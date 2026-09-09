package finance

import (
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
)

var (
	ErrJournalTooFewLines = errors.New("journal entry must contain at least two lines")
	ErrJournalZeroLine    = errors.New("journal line amount must be non-zero")
	ErrJournalUnbalanced  = errors.New("journal entry is not balanced")
	ErrJournalOverflow    = errors.New("journal entry amount overflow")
)

// ValidateJournal enforces the finance-domain invariants that must hold before
// persistence. It is intentionally storage-independent so every adapter
// (PostgreSQL, legacy Spanner, imports, tests, jobs) shares the same rules.
func ValidateJournal(lines []*LineItem) error {
	if len(lines) < 2 {
		return ErrJournalTooFewLines
	}

	seenLineIDs := make(map[uuid.UUID]struct{}, len(lines))
	var total int64

	for i, line := range lines {
		if line == nil {
			return fmt.Errorf("line %d: nil line", i+1)
		}
		if line.LineItemID == uuid.Nil {
			return fmt.Errorf("line %d: line item id is required", i+1)
		}
		if _, exists := seenLineIDs[line.LineItemID]; exists {
			return fmt.Errorf("line %d: duplicate line item id %s", i+1, line.LineItemID)
		}
		seenLineIDs[line.LineItemID] = struct{}{}

		if line.AccountID == uuid.Nil {
			return fmt.Errorf("line %d: account id is required", i+1)
		}
		if line.AmountCents == 0 {
			return fmt.Errorf("line %d: %w", i+1, ErrJournalZeroLine)
		}

		if line.AmountCents > 0 && total > math.MaxInt64-line.AmountCents {
			return ErrJournalOverflow
		}
		if line.AmountCents < 0 && total < math.MinInt64-line.AmountCents {
			return ErrJournalOverflow
		}
		total += line.AmountCents
	}

	if total != 0 {
		return fmt.Errorf("%w: sum=%d cents", ErrJournalUnbalanced, total)
	}
	return nil
}
