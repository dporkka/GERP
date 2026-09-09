package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Audit struct {
	ID           uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     any
	OccurredAt   time.Time
}

type Outbox struct {
	ID            uuid.UUID
	Topic         string
	AggregateType string
	AggregateID   string
	Payload       any
	OccurredAt    time.Time
}

type Recorder struct{}

func NewRecorder() Recorder { return Recorder{} }

func (Recorder) Record(ctx context.Context, tx pgx.Tx, audit Audit, outbox ...Outbox) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	if audit.Action == "" || audit.ResourceType == "" || audit.ResourceID == "" {
		return fmt.Errorf("audit action, resource type and resource id are required")
	}
	if audit.ID == uuid.Nil {
		audit.ID = uuid.New()
	}
	if audit.OccurredAt.IsZero() {
		audit.OccurredAt = time.Now().UTC()
	}
	metadata, err := marshalObject(audit.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	var actor any
	if scope.ActorUserID != uuid.Nil {
		actor = scope.ActorUserID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, tenant_id, actor_user_id, action, resource_type, resource_id, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)`,
		audit.ID, scope.TenantID, actor, audit.Action, audit.ResourceType, audit.ResourceID, string(metadata), audit.OccurredAt,
	); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}

	for i := range outbox {
		event := &outbox[i]
		if event.Topic == "" || event.AggregateType == "" || event.AggregateID == "" {
			return fmt.Errorf("outbox topic, aggregate type and aggregate id are required")
		}
		if event.ID == uuid.Nil {
			event.ID = uuid.New()
		}
		if event.OccurredAt.IsZero() {
			event.OccurredAt = audit.OccurredAt
		}
		payload, err := marshalObject(event.Payload)
		if err != nil {
			return fmt.Errorf("marshal outbox payload: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox_events
				(id, tenant_id, topic, aggregate_type, aggregate_id, payload, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`,
			event.ID, scope.TenantID, event.Topic, event.AggregateType, event.AggregateID, string(payload), event.OccurredAt,
		); err != nil {
			return fmt.Errorf("insert outbox event: %w", err)
		}
	}
	return nil
}

func marshalObject(value any) ([]byte, error) {
	if value == nil {
		return []byte(`{}`), nil
	}
	return json.Marshal(value)
}
