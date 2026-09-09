package postgres

import (
	"context"
	"fmt"

	"gerp/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UnitOfWork struct {
	pool *pgxpool.Pool
}

func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

func (u *UnitOfWork) Pool() *pgxpool.Pool { return u.pool }

type TxFunc func(context.Context, pgx.Tx) error

func (u *UnitOfWork) Within(ctx context.Context, opts pgx.TxOptions, fn TxFunc) error {
	if u == nil || u.pool == nil {
		return fmt.Errorf("postgres unit of work is not configured")
	}
	if fn == nil {
		return fmt.Errorf("transaction function is required")
	}
	scope, err := tenant.Require(ctx)
	if err != nil {
		return err
	}

	tx, err := u.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin postgres transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	actor := ""
	if scope.ActorUserID != [16]byte{} {
		actor = scope.ActorUserID.String()
	}
	if _, err := tx.Exec(ctx,
		"SELECT set_config('gerp.tenant_id', $1, true), set_config('gerp.actor_user_id', $2, true)",
		scope.TenantID.String(), actor,
	); err != nil {
		return fmt.Errorf("set transaction tenant context: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit postgres transaction: %w", err)
	}
	return nil
}
