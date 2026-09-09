BEGIN;

CREATE TABLE IF NOT EXISTS gl_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code text NOT NULL,
    name text NOT NULL,
    account_type text NOT NULL CHECK (account_type IN ('asset','liability','equity','revenue','expense')),
    normal_balance text NOT NULL CHECK (normal_balance IN ('debit','credit')),
    currency char(3),
    active boolean NOT NULL DEFAULT true,
    parent_id uuid REFERENCES gl_accounts(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS fiscal_periods (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name text NOT NULL,
    starts_on date NOT NULL,
    ends_on date NOT NULL,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','soft_closed','closed')),
    closed_at timestamptz,
    closed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    CHECK (starts_on <= ends_on),
    UNIQUE (tenant_id, starts_on, ends_on)
);

CREATE TABLE IF NOT EXISTS journal_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number text NOT NULL,
    effective_date date NOT NULL,
    currency char(3) NOT NULL,
    description text NOT NULL DEFAULT '',
    source_type text,
    source_id text,
    posted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    posted_at timestamptz NOT NULL DEFAULT now(),
    reversal_of uuid REFERENCES journal_entries(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, number),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS journal_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    journal_entry_id uuid NOT NULL,
    account_id uuid NOT NULL,
    debit_minor bigint NOT NULL DEFAULT 0 CHECK (debit_minor >= 0),
    credit_minor bigint NOT NULL DEFAULT 0 CHECK (credit_minor >= 0),
    memo text NOT NULL DEFAULT '',
    dimension_values jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((debit_minor > 0 AND credit_minor = 0) OR (credit_minor > 0 AND debit_minor = 0)),
    FOREIGN KEY (tenant_id, journal_entry_id)
        REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, account_id)
        REFERENCES gl_accounts(tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS journal_lines_entry_idx
    ON journal_lines(tenant_id, journal_entry_id);
CREATE INDEX IF NOT EXISTS journal_lines_account_idx
    ON journal_lines(tenant_id, account_id, journal_entry_id);

CREATE OR REPLACE FUNCTION gerp_reject_posted_journal_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'posted journals are immutable; create a reversal instead';
END;
$$;

DROP TRIGGER IF EXISTS journal_entries_immutable ON journal_entries;
CREATE TRIGGER journal_entries_immutable
BEFORE UPDATE OR DELETE ON journal_entries
FOR EACH ROW EXECUTE FUNCTION gerp_reject_posted_journal_mutation();

DROP TRIGGER IF EXISTS journal_lines_immutable ON journal_lines;
CREATE TRIGGER journal_lines_immutable
BEFORE UPDATE OR DELETE ON journal_lines
FOR EACH ROW EXECUTE FUNCTION gerp_reject_posted_journal_mutation();

-- Reversal state is represented by a separate reversing journal linked through
-- reversal_of; the original posted journal is never mutated.
--
-- Balance validation is performed by the Go finance domain before insertion and
-- should be repeated by the posting transaction before COMMIT. Deferring a
-- statement-level database assertion is intentionally left to the repository
-- implementation because multi-row balance checks cannot be expressed as a
-- simple row CHECK constraint.

COMMIT;
