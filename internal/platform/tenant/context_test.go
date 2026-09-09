package tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestRequireRejectsMissingScope(t *testing.T) {
	_, err := Require(context.Background())
	if !errors.Is(err, ErrMissingScope) {
		t.Fatalf("expected missing scope, got %v", err)
	}
}

func TestWithScopeRequiresTenant(t *testing.T) {
	_, err := WithScope(context.Background(), Scope{})
	if !errors.Is(err, ErrMissingTenant) {
		t.Fatalf("expected missing tenant, got %v", err)
	}
}

func TestScopeRoundTrip(t *testing.T) {
	scope := Scope{TenantID: uuid.New(), ActorUserID: uuid.New()}
	ctx, err := WithScope(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Require(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != scope {
		t.Fatalf("got %#v, want %#v", got, scope)
	}
}
