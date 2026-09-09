package crm_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"gerp/internal/crm"
	platformevents "gerp/internal/platform/events"
	"gerp/internal/platform/postgres"
	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
)

func TestDealMoveUsesOptimisticRevisionAndAtomicEvidence(t *testing.T) {
	url := os.Getenv("GERP_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("GERP_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := postgres.Open(ctx, postgres.Config{URL: url, MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	tenantID := uuid.New()
	actorID := uuid.New()
	pipelineID := uuid.New()
	stageOneID := uuid.New()
	stageTwoID := uuid.New()
	dealID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO tenants (id, slug, name, base_currency) VALUES ($1, $2, 'CRM test', 'USD')`, tenantID, "crm-"+uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'CRM tester')`, actorID, fmt.Sprintf("%s@example.test", actorID)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO crm_pipelines (id, tenant_id, name, is_default) VALUES ($1, $2, 'Sales', true)`, pipelineID, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO crm_pipeline_stages (id, tenant_id, pipeline_id, name, position, outcome, probability_bp)
		VALUES ($1, $2, $3, 'Discovery', 1, 'open', 1000),
		       ($4, $2, $3, 'Proposal', 2, 'open', 5000)`, stageOneID, tenantID, pipelineID, stageTwoID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO crm_deals (id, tenant_id, pipeline_id, stage_id, name, currency, revision)
		VALUES ($1, $2, $3, $4, 'Revision test', 'USD', 1)`, dealID, tenantID, pipelineID, stageOneID); err != nil {
		t.Fatal(err)
	}

	scoped, err := tenant.WithScope(ctx, tenant.Scope{TenantID: tenantID, ActorUserID: actorID})
	if err != nil {
		t.Fatal(err)
	}
	service := crm.NewCommandService(postgres.NewUnitOfWork(pool), crm.NewPostgresRepository(), platformevents.NewRecorder())

	revision, err := service.MoveDeal(scoped, dealID, 1, stageTwoID)
	if err != nil {
		t.Fatal(err)
	}
	if revision != 2 {
		t.Fatalf("revision=%d, want 2", revision)
	}

	if _, err := service.MoveDeal(scoped, dealID, 1, stageOneID); !errors.Is(err, crm.ErrDealRevisionConflict) {
		t.Fatalf("expected stale revision conflict, got %v", err)
	}

	var currentRevision int64
	var currentStage uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT revision, stage_id FROM crm_deals WHERE tenant_id = $1 AND id = $2`, tenantID, dealID).Scan(&currentRevision, &currentStage); err != nil {
		t.Fatal(err)
	}
	if currentRevision != 2 || currentStage != stageTwoID {
		t.Fatalf("stale command mutated deal: revision=%d stage=%s", currentRevision, currentStage)
	}

	var activities int
	var audits int
	var outbox int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM crm_activities WHERE tenant_id = $1 AND subject_type = 'deal' AND subject_id = $2`, tenantID, dealID).Scan(&activities); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE tenant_id = $1 AND resource_type = 'deal' AND resource_id = $2`, tenantID, dealID.String()).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE tenant_id = $1 AND aggregate_type = 'deal' AND aggregate_id = $2`, tenantID, dealID.String()).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if activities != 1 || audits != 1 || outbox != 1 {
		t.Fatalf("expected one committed evidence set, got activities=%d audits=%d outbox=%d", activities, audits, outbox)
	}
}
