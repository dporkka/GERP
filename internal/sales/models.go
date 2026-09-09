package sales

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type QuoteLine struct {
	ID             uuid.UUID
	Position       int
	Description    string
	Quantity       string
	UnitPriceMinor int64
	LineTotalMinor int64
	TaxCode        string
}

type QuoteDraft struct {
	ID         uuid.UUID
	Number     string
	CompanyID  *uuid.UUID
	Currency   string
	ValidUntil *time.Time
	Lines      []QuoteLine
}

func (quote QuoteDraft) Validate() error {
	if quote.ID == uuid.Nil || quote.Number == "" {
		return fmt.Errorf("quote id and number are required")
	}
	if len(quote.Currency) != 3 {
		return fmt.Errorf("quote currency must be ISO-4217 alpha-3")
	}
	if len(quote.Lines) == 0 {
		return fmt.Errorf("quote requires at least one line")
	}
	for i, line := range quote.Lines {
		if line.ID == uuid.Nil || line.Position <= 0 || line.Description == "" || line.Quantity == "" {
			return fmt.Errorf("quote line %d is incomplete", i+1)
		}
		if line.UnitPriceMinor < 0 || line.LineTotalMinor < 0 {
			return fmt.Errorf("quote line %d has negative amount", i+1)
		}
	}
	return nil
}

type Order struct {
	ID         uuid.UUID
	QuoteID    uuid.UUID
	Number     string
	Currency   string
	Status     string
	TotalMinor int64
	Revision   int64
}

type Invoice struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	Number     string
	Currency   string
	Status     string
	TotalMinor int64
	Revision   int64
}
