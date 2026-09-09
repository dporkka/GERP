package crm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gerp/internal/platform/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrDealNotFound         = errors.New("deal not found")
	ErrDealRevisionConflict = errors.New("deal revision conflict")
	ErrDealState            = errors.New("deal is not in the required state")
)

type PostgresRepository struct{}

func NewPostgresRepository() PostgresRepository { return PostgresRepository{} }

func (repository PostgresRepository) MoveDeal(ctx context.Context, tx pgx.Tx, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID) (int64, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, err
	}
	var revision int64
	err = tx.QueryRow(ctx, `
		UPDATE crm_deals AS d
		   SET stage_id = $4,
		       revision = d.revision + 1,
		       updated_at = now()
		 WHERE d.tenant_id = $1
		   AND d.id = $2
		   AND d.revision = $3
		   AND d.status = 'open'
		   AND d.archived_at IS NULL
		   AND EXISTS (
		       SELECT 1
		         FROM crm_pipeline_stages AS s
		        WHERE s.tenant_id = d.tenant_id
		          AND s.pipeline_id = d.pipeline_id
		          AND s.id = $4
		          AND s.active
		          AND s.outcome = 'open'
		   )
		RETURNING d.revision`, scope.TenantID, dealID, expectedRevision, stageID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, repository.classifyDealWriteFailure(ctx, tx, dealID, expectedRevision)
	}
	if err != nil {
		return 0, fmt.Errorf("move deal: %w", err)
	}
	if err := repository.recordActivity(ctx, tx, dealID, "deal.moved", map[string]any{"stage_id": stageID, "revision": revision}); err != nil {
		return 0, err
	}
	return revision, nil
}

func (repository PostgresRepository) CloseDeal(ctx context.Context, tx pgx.Tx, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID, outcome, reason string) (int64, error) {
	if outcome != "won" && outcome != "lost" {
		return 0, fmt.Errorf("deal outcome must be won or lost")
	}
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, err
	}
	var revision int64
	err = tx.QueryRow(ctx, `
		UPDATE crm_deals AS d
		   SET stage_id = $4,
		       status = $5,
		       closed_at = now(),
		       close_reason = NULLIF($6, ''),
		       revision = d.revision + 1,
		       updated_at = now()
		 WHERE d.tenant_id = $1
		   AND d.id = $2
		   AND d.revision = $3
		   AND d.status = 'open'
		   AND d.archived_at IS NULL
		   AND EXISTS (
		       SELECT 1
		         FROM crm_pipeline_stages AS s
		        WHERE s.tenant_id = d.tenant_id
		          AND s.pipeline_id = d.pipeline_id
		          AND s.id = $4
		          AND s.active
		          AND s.outcome = $5
		   )
		RETURNING d.revision`, scope.TenantID, dealID, expectedRevision, stageID, outcome, reason).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, repository.classifyDealWriteFailure(ctx, tx, dealID, expectedRevision)
	}
	if err != nil {
		return 0, fmt.Errorf("close deal: %w", err)
	}
	if err := repository.recordActivity(ctx, tx, dealID, "deal."+outcome, map[string]any{"stage_id": stageID, "reason": reason, "revision": revision}); err != nil {
		return 0, err
	}
	return revision, nil
}

func (repository PostgresRepository) ReopenDeal(ctx context.Context, tx pgx.Tx, dealID uuid.UUID, expectedRevision int64, stageID uuid.UUID) (int64, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, err
	}
	var revision int64
	err = tx.QueryRow(ctx, `
		UPDATE crm_deals AS d
		   SET stage_id = $4,
		       status = 'open',
		       closed_at = NULL,
		       close_reason = NULL,
		       revision = d.revision + 1,
		       updated_at = now()
		 WHERE d.tenant_id = $1
		   AND d.id = $2
		   AND d.revision = $3
		   AND d.status IN ('won', 'lost')
		   AND d.archived_at IS NULL
		   AND EXISTS (
		       SELECT 1
		         FROM crm_pipeline_stages AS s
		        WHERE s.tenant_id = d.tenant_id
		          AND s.pipeline_id = d.pipeline_id
		          AND s.id = $4
		          AND s.active
		          AND s.outcome = 'open'
		   )
		RETURNING d.revision`, scope.TenantID, dealID, expectedRevision, stageID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, repository.classifyDealWriteFailure(ctx, tx, dealID, expectedRevision)
	}
	if err != nil {
		return 0, fmt.Errorf("reopen deal: %w", err)
	}
	if err := repository.recordActivity(ctx, tx, dealID, "deal.reopened", map[string]any{"stage_id": stageID, "revision": revision}); err != nil {
		return 0, err
	}
	return revision, nil
}

func (repository PostgresRepository) SetArchived(ctx context.Context, tx pgx.Tx, dealID uuid.UUID, expectedRevision int64, archived bool) (int64, error) {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return 0, err
	}
	var revision int64
	var activity string
	var query string
	if archived {
		activity = "deal.archived"
		query = `
			UPDATE crm_deals
			   SET archived_at = now(), revision = revision + 1, updated_at = now()
			 WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND archived_at IS NULL
			RETURNING revision`
	} else {
		activity = "deal.restored"
		query = `
			UPDATE crm_deals
			   SET archived_at = NULL, revision = revision + 1, updated_at = now()
			 WHERE tenant_id = $1 AND id = $2 AND revision = $3 AND archived_at IS NOT NULL
			RETURNING revision`
	}
	err = tx.QueryRow(ctx, query, scope.TenantID, dealID, expectedRevision).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, repository.classifyDealWriteFailure(ctx, tx, dealID, expectedRevision)
	}
	if err != nil {
		return 0, fmt.Errorf("change deal archive state: %w", err)
	}
	if err := repository.recordActivity(ctx, tx, dealID, activity, map[string]any{"revision": revision}); err != nil {
		return 0, err
	}
	return revision, nil
}

func (PostgresRepository) classifyDealWriteFailure(ctx context.Context, tx pgx.Tx, dealID uuid.UUID, expectedRevision int64) error {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	var currentRevision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM crm_deals WHERE tenant_id = $1 AND id = $2`, scope.TenantID, dealID).Scan(&currentRevision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrDealNotFound
		}
		return fmt.Errorf("classify deal write failure: %w", err)
	}
	if currentRevision != expectedRevision {
		return fmt.Errorf("%w: expected %d, current %d", ErrDealRevisionConflict, expectedRevision, currentRevision)
	}
	return ErrDealState
}

func (PostgresRepository) recordActivity(ctx context.Context, tx pgx.Tx, dealID uuid.UUID, activityType string, payload any) error {
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal CRM activity payload: %w", err)
	}
	var actor any
	if scope.ActorUserID != uuid.Nil {
		actor = scope.ActorUserID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO crm_activities
			(id, tenant_id, actor_user_id, activity_type, subject_type, subject_id, causal_type, causal_id, payload)
		VALUES ($1, $2, $3, $4, 'deal', $5, 'command', $6, $7::jsonb)`,
		uuid.New(), scope.TenantID, actor, activityType, dealID, uuid.New(), string(encoded)); err != nil {
		return fmt.Errorf("insert CRM activity: %w", err)
	}
	return nil
}
