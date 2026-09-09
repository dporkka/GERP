package finance

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type JournalLine struct {
	ID              uuid.UUID
	AccountID       uuid.UUID
	DebitMinor      int64
	CreditMinor     int64
	Memo            string
	DimensionValues map[string]any
}

type DraftJournal struct {
	ID            uuid.UUID
	Number        string
	EffectiveDate time.Time
	Currency      string
	Description   string
	SourceType    string
	SourceID      string
	ReversalOf    *uuid.UUID
	Lines         []JournalLine
}

type PostedJournal struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Number        string
	EffectiveDate time.Time
	Currency      string
	Description   string
	Status        string
	PostedAt      *time.Time
	ReversalOf    *uuid.UUID
	Lines         []JournalLine
}

func (draft DraftJournal) Validate() error {
	if draft.ID == uuid.Nil {
		return fmt.Errorf("journal id is required")
	}
	if draft.Number == "" {
		return fmt.Errorf("journal number is required")
	}
	if draft.EffectiveDate.IsZero() {
		return fmt.Errorf("journal effective date is required")
	}
	if len(draft.Currency) != 3 {
		return fmt.Errorf("journal currency must be ISO-4217 alpha-3")
	}

	legacy := make([]*LineItem, 0, len(draft.Lines))
	for i, line := range draft.Lines {
		if line.ID == uuid.Nil {
			return fmt.Errorf("line %d: id is required", i+1)
		}
		if line.AccountID == uuid.Nil {
			return fmt.Errorf("line %d: account id is required", i+1)
		}
		if line.DebitMinor < 0 || line.CreditMinor < 0 {
			return fmt.Errorf("line %d: debit and credit must be non-negative", i+1)
		}
		if (line.DebitMinor > 0) == (line.CreditMinor > 0) {
			return fmt.Errorf("line %d: exactly one of debit or credit must be positive", i+1)
		}
		amount := line.DebitMinor
		if line.CreditMinor > 0 {
			amount = -line.CreditMinor
		}
		legacy = append(legacy, &LineItem{
			LineItemID:  line.ID,
			AccountID:   line.AccountID,
			AmountCents: amount,
		})
	}
	return ValidateJournal(legacy)
}
