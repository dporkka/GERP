package tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrMissingScope = errors.New("tenant scope is required")
	ErrMissingTenant = errors.New("tenant id is required")
)

type Scope struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
}

func (s Scope) Validate() error {
	if s.TenantID == uuid.Nil {
		return ErrMissingTenant
	}
	return nil
}

type contextKey struct{}

func WithScope(ctx context.Context, scope Scope) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, contextKey{}, scope), nil
}

func Require(ctx context.Context) (Scope, error) {
	if ctx == nil {
		return Scope{}, ErrMissingScope
	}
	scope, ok := ctx.Value(contextKey{}).(Scope)
	if !ok {
		return Scope{}, ErrMissingScope
	}
	if err := scope.Validate(); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

func TenantID(ctx context.Context) (uuid.UUID, error) {
	scope, err := Require(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return scope.TenantID, nil
}
