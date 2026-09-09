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
    parent_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, parent_id)
        REFERENCES gl_accounts(tenant_id, id) ON DELETE RESTRICT
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
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','posted')),
    posted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    posted_at timestamptz,
    reversal_of uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, number),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, reversal_of)
        REFERENCES journal_entries(tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS journal_entries_one_reversal_idx
    ON journal_entries(tenant_id, reversal_of)
    WHERE reversal_of IS NOT NULL;

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

CREATE OR REPLACE FUNCTION gerp_guard_journal_entry_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    line_count bigint;
    debit_total numeric;
    credit_total numeric;
BEGIN
    IF OLD.status = 'posted' THEN
        RAISE EXCEPTION 'posted journals are immutable; create a reversal instead';
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;

    IF NEW.status = 'posted' THEN
        SELECT count(*), COALESCE(sum(debit_minor), 0), COALESCE(sum(credit_minor), 0)
          INTO line_count, debit_total, credit_total
          FROM journal_lines
         WHERE tenant_id = NEW.tenant_id
           AND journal_entry_id = NEW.id;

        IF line_count < 2 THEN
            RAISE EXCEPTION 'journal % requires at least two lines before posting', NEW.id;
        END IF;
        IF debit_total <> credit_total THEN
            RAISE EXCEPTION 'journal % is unbalanced: debits %, credits %', NEW.id, debit_total, credit_total;
        END IF;

        NEW.posted_at := COALESCE(NEW.posted_at, now());
    ELSE
        NEW.posted_at := NULL;
        NEW.posted_by := NULL;
    END IF;

    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS journal_entries_guard ON journal_entries;
CREATE TRIGGER journal_entries_guard
BEFORE UPDATE OR DELETE ON journal_entries
FOR EACH ROW EXECUTE FUNCTION gerp_guard_journal_entry_mutation();

CREATE OR REPLACE FUNCTION gerp_guard_journal_line_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    target_tenant uuid;
    target_entry uuid;
    entry_status text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_tenant := OLD.tenant_id;
        target_entry := OLD.journal_entry_id;
    ELSE
        target_tenant := NEW.tenant_id;
        target_entry := NEW.journal_entry_id;
    END IF;

    IF TG_OP = 'UPDATE' AND
       (NEW.tenant_id <> OLD.tenant_id OR NEW.journal_entry_id <> OLD.journal_entry_id) THEN
        RAISE EXCEPTION 'journal lines cannot be moved between journals';
    END IF;

    SELECT status
      INTO entry_status
      FROM journal_entries
     WHERE tenant_id = target_tenant
       AND id = target_entry;

    IF entry_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'journal lines are mutable only while the journal is draft';
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS journal_lines_guard ON journal_lines;
CREATE TRIGGER journal_lines_guard
BEFORE INSERT OR UPDATE OR DELETE ON journal_lines
FOR EACH ROW EXECUTE FUNCTION gerp_guard_journal_line_mutation();

-- Posting is the accounting boundary: a draft may be edited freely, but the
-- draft -> posted transition is rejected by PostgreSQL unless it has at least
-- two lines and total debits equal total credits. Once posted, neither the
-- journal header nor its lines may be changed. Corrections are new reversing
-- journals linked through reversal_of.

COMMIT;
