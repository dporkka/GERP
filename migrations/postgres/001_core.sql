BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    base_currency char(3) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS memberships (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','invited','suspended')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS permissions (
    code text PRIMARY KEY,
    description text NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_code text NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_code)
);

CREATE TABLE IF NOT EXISTS membership_roles (
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, user_id, role_id),
    FOREIGN KEY (tenant_id, user_id)
        REFERENCES memberships(tenant_id, user_id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key text NOT NULL,
    request_hash bytea NOT NULL,
    status_code integer,
    response_headers jsonb,
    response_body bytea,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, key)
);
CREATE INDEX IF NOT EXISTS idempotency_expiry_idx ON idempotency_keys(expires_at);

CREATE TABLE IF NOT EXISTS audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_tenant_time_idx
    ON audit_events(tenant_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_resource_idx
    ON audit_events(tenant_id, resource_type, resource_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS outbox_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    topic text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error text
);
CREATE INDEX IF NOT EXISTS outbox_unpublished_idx
    ON outbox_events(next_attempt_at, occurred_at)
    WHERE published_at IS NULL;

COMMIT;
