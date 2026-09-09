BEGIN;

CREATE TABLE IF NOT EXISTS crm_companies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name text NOT NULL,
    domain text,
    phone text,
    website text,
    owner_id uuid REFERENCES users(id) ON DELETE SET NULL,
    lifecycle_stage text NOT NULL DEFAULT 'lead'
        CHECK (lifecycle_stage IN ('lead','prospect','customer','inactive')),
    custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS crm_companies_name_idx
    ON crm_companies(tenant_id, lower(name)) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS crm_companies_owner_idx
    ON crm_companies(tenant_id, owner_id) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS crm_contacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT '',
    display_name text NOT NULL,
    email text,
    phone text,
    job_title text,
    owner_id uuid REFERENCES users(id) ON DELETE SET NULL,
    lifecycle_stage text NOT NULL DEFAULT 'lead'
        CHECK (lifecycle_stage IN ('lead','prospect','customer','inactive')),
    custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS crm_contacts_name_idx
    ON crm_contacts(tenant_id, lower(display_name)) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS crm_contacts_email_idx
    ON crm_contacts(tenant_id, lower(email)) WHERE email IS NOT NULL AND archived_at IS NULL;
CREATE INDEX IF NOT EXISTS crm_contacts_owner_idx
    ON crm_contacts(tenant_id, owner_id) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS crm_contact_companies (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    contact_id uuid NOT NULL,
    company_id uuid NOT NULL,
    relationship text NOT NULL DEFAULT 'employee',
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, contact_id, company_id),
    FOREIGN KEY (tenant_id, contact_id)
        REFERENCES crm_contacts(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, company_id)
        REFERENCES crm_companies(tenant_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS crm_contact_primary_company_idx
    ON crm_contact_companies(tenant_id, contact_id)
    WHERE is_primary;

CREATE TABLE IF NOT EXISTS crm_pipelines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name text NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name)
);
CREATE UNIQUE INDEX IF NOT EXISTS crm_default_pipeline_idx
    ON crm_pipelines(tenant_id) WHERE is_default AND active;

CREATE TABLE IF NOT EXISTS crm_pipeline_stages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    pipeline_id uuid NOT NULL,
    name text NOT NULL,
    position integer NOT NULL CHECK (position >= 0),
    outcome text NOT NULL DEFAULT 'open' CHECK (outcome IN ('open','won','lost')),
    probability_bp integer NOT NULL DEFAULT 0 CHECK (probability_bp BETWEEN 0 AND 10000),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, pipeline_id, id),
    UNIQUE (tenant_id, pipeline_id, position),
    FOREIGN KEY (tenant_id, pipeline_id)
        REFERENCES crm_pipelines(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS crm_deals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    pipeline_id uuid NOT NULL,
    stage_id uuid NOT NULL,
    company_id uuid,
    primary_contact_id uuid,
    owner_id uuid REFERENCES users(id) ON DELETE SET NULL,
    name text NOT NULL,
    amount_minor bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','won','lost')),
    expected_close_on date,
    closed_at timestamptz,
    close_reason text,
    custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, pipeline_id)
        REFERENCES crm_pipelines(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, pipeline_id, stage_id)
        REFERENCES crm_pipeline_stages(tenant_id, pipeline_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, company_id)
        REFERENCES crm_companies(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, primary_contact_id)
        REFERENCES crm_contacts(tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS crm_deals_stage_idx
    ON crm_deals(tenant_id, pipeline_id, stage_id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS crm_deals_owner_idx
    ON crm_deals(tenant_id, owner_id, expected_close_on) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS crm_deals_company_idx
    ON crm_deals(tenant_id, company_id) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS crm_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    assigned_to uuid REFERENCES users(id) ON DELETE SET NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    subject text NOT NULL,
    description text NOT NULL DEFAULT '',
    due_at timestamptz,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','completed','cancelled')),
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
    related_type text,
    related_id uuid,
    completed_at timestamptz,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS crm_tasks_assignee_due_idx
    ON crm_tasks(tenant_id, assigned_to, due_at) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS crm_tasks_related_idx
    ON crm_tasks(tenant_id, related_type, related_id);

CREATE TABLE IF NOT EXISTS crm_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    author_id uuid REFERENCES users(id) ON DELETE SET NULL,
    related_type text NOT NULL,
    related_id uuid NOT NULL,
    body text NOT NULL,
    private boolean NOT NULL DEFAULT false,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS crm_notes_related_idx
    ON crm_notes(tenant_id, related_type, related_id, created_at DESC);

CREATE TABLE IF NOT EXISTS crm_activities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    activity_type text NOT NULL,
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    causal_type text,
    causal_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS crm_activities_subject_idx
    ON crm_activities(tenant_id, subject_type, subject_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS crm_followers (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, subject_type, subject_id)
);

CREATE TABLE IF NOT EXISTS crm_saved_views (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_name text NOT NULL,
    name text NOT NULL,
    definition jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id, resource_name, name)
);

CREATE OR REPLACE FUNCTION gerp_reject_crm_activity_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'CRM activity history is append-only';
END;
$$;

DROP TRIGGER IF EXISTS crm_activities_immutable ON crm_activities;
CREATE TRIGGER crm_activities_immutable
BEFORE UPDATE OR DELETE ON crm_activities
FOR EACH ROW EXECUTE FUNCTION gerp_reject_crm_activity_mutation();

COMMIT;
