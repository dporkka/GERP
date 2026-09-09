package goblcompliance

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
	"github.com/invopop/gobl"
	"github.com/jackc/pgx/v5"
)

type Snapshot struct {
	ID        uuid.UUID
	InvoiceID uuid.UUID
	SHA256    [sha256.Size]byte
	Payload   []byte
	CreatedAt time.Time
}

type SnapshotStore struct {
	service Service
}

func NewSnapshotStore(service Service) SnapshotStore {
	return SnapshotStore{service: service}
}

func (store SnapshotStore) SaveValidated(ctx context.Context, tx pgx.Tx, invoiceID uuid.UUID, envelope *gobl.Envelope) (Snapshot, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if invoiceID == uuid.Nil {
		return Snapshot{}, fmt.Errorf("invoice id is required")
	}
	payload, err := store.service.MarshalValidated(envelope)
	if err != nil {
		return Snapshot{}, err
	}
	digest := sha256.Sum256(payload)
	snapshot := Snapshot{
		ID:        uuid.New(),
		InvoiceID: invoiceID,
		SHA256:    digest,
		Payload:   append([]byte(nil), payload...),
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO invoice_document_snapshots
			(id, tenant_id, invoice_id, format, media_type, sha256, payload)
		VALUES ($1, $2, $3, 'gobl-json', 'application/json', $4, $5::jsonb)
		RETURNING created_at`, snapshot.ID, scope.TenantID, invoiceID, digest[:], string(payload)).Scan(&snapshot.CreatedAt); err != nil {
		return Snapshot{}, fmt.Errorf("persist immutable GOBL invoice snapshot: %w", err)
	}
	return snapshot, nil
}
