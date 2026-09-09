package goblcompliance

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/invopop/gobl"
)

var ErrNilEnvelope = errors.New("GOBL envelope is nil")

// Service is Gerp's boundary to the GOBL business-document engine. Gerp keeps
// invoice lifecycle, authorization and persistence in its own domain; GOBL is
// responsible for document calculation, tax/regime rules and validation.
type Service struct{}

func New() Service { return Service{} }

// Prepare calculates a GOBL document, wraps it in an envelope and validates the
// resulting business document before it may be persisted or delivered.
func (Service) Prepare(document any) (*gobl.Envelope, error) {
	if document == nil {
		return nil, errors.New("business document is nil")
	}

	envelope, err := gobl.Envelop(document)
	if err != nil {
		return nil, fmt.Errorf("prepare GOBL document: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return nil, fmt.Errorf("validate GOBL document: %w", err)
	}
	return envelope, nil
}

func (Service) Validate(envelope *gobl.Envelope) error {
	if envelope == nil {
		return ErrNilEnvelope
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("validate GOBL envelope: %w", err)
	}
	return nil
}

// MarshalValidated serializes only a document that passes GOBL validation.
// The resulting JSON is suitable for immutable document snapshots and hashes.
func (service Service) MarshalValidated(envelope *gobl.Envelope) ([]byte, error) {
	if err := service.Validate(envelope); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal GOBL envelope: %w", err)
	}
	return payload, nil
}
