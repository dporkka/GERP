BEGIN;

CREATE TABLE IF NOT EXISTS sales_quotes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number text NOT NULL,
    company_id uuid,
    currency char(3) NOT NULL,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','sent','accepted','rejected','expired')),
    valid_until date,
    total_minor bigint NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    sent_at timestamptz,
    accepted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    FOREIGN KEY (tenant_id, company_id)
        REFERENCES crm_companies(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS sales_quote_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    quote_id uuid NOT NULL,
    position integer NOT NULL CHECK (position > 0),
    description text NOT NULL,
    quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    line_total_minor bigint NOT NULL CHECK (line_total_minor >= 0),
    tax_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, quote_id, position),
    FOREIGN KEY (tenant_id, quote_id)
        REFERENCES sales_quotes(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS sales_orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number text NOT NULL,
    quote_id uuid NOT NULL,
    currency char(3) NOT NULL,
    status text NOT NULL DEFAULT 'confirmed'
        CHECK (status IN ('confirmed','fulfilled','cancelled')),
    total_minor bigint NOT NULL CHECK (total_minor >= 0),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    confirmed_at timestamptz NOT NULL DEFAULT now(),
    fulfilled_at timestamptz,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    UNIQUE (tenant_id, quote_id),
    FOREIGN KEY (tenant_id, quote_id)
        REFERENCES sales_quotes(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS sales_order_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id uuid NOT NULL,
    source_quote_line_id uuid,
    position integer NOT NULL CHECK (position > 0),
    description text NOT NULL,
    quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    line_total_minor bigint NOT NULL CHECK (line_total_minor >= 0),
    tax_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, order_id, position),
    FOREIGN KEY (tenant_id, order_id)
        REFERENCES sales_orders(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS ar_invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    number text NOT NULL,
    order_id uuid NOT NULL,
    currency char(3) NOT NULL,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','issued','paid','void')),
    issue_date date,
    due_date date,
    total_minor bigint NOT NULL CHECK (total_minor >= 0),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    issued_at timestamptz,
    paid_at timestamptz,
    voided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number),
    UNIQUE (tenant_id, order_id),
    FOREIGN KEY (tenant_id, order_id)
        REFERENCES sales_orders(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS ar_invoice_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    invoice_id uuid NOT NULL,
    source_order_line_id uuid,
    position integer NOT NULL CHECK (position > 0),
    description text NOT NULL,
    quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    line_total_minor bigint NOT NULL CHECK (line_total_minor >= 0),
    tax_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, invoice_id, position),
    FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES ar_invoices(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS invoice_document_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    invoice_id uuid NOT NULL,
    format text NOT NULL DEFAULT 'gobl-json',
    media_type text NOT NULL DEFAULT 'application/json',
    sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, invoice_id, sha256),
    FOREIGN KEY (tenant_id, invoice_id)
        REFERENCES ar_invoices(tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS invoice_document_snapshots_invoice_idx
    ON invoice_document_snapshots(tenant_id, invoice_id, created_at DESC);

CREATE OR REPLACE FUNCTION gerp_reject_invoice_snapshot_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'invoice document snapshots are immutable';
END;
$$;

DROP TRIGGER IF EXISTS invoice_document_snapshots_immutable ON invoice_document_snapshots;
CREATE TRIGGER invoice_document_snapshots_immutable
BEFORE UPDATE OR DELETE ON invoice_document_snapshots
FOR EACH ROW EXECUTE FUNCTION gerp_reject_invoice_snapshot_mutation();

CREATE OR REPLACE FUNCTION gerp_guard_quote_line_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    target_tenant uuid;
    target_quote uuid;
    quote_status text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_tenant := OLD.tenant_id;
        target_quote := OLD.quote_id;
    ELSE
        target_tenant := NEW.tenant_id;
        target_quote := NEW.quote_id;
    END IF;

    SELECT status INTO quote_status
      FROM sales_quotes
     WHERE tenant_id = target_tenant AND id = target_quote;

    IF quote_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'quote lines are mutable only while the quote is draft';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS sales_quote_lines_guard ON sales_quote_lines;
CREATE TRIGGER sales_quote_lines_guard
BEFORE INSERT OR UPDATE OR DELETE ON sales_quote_lines
FOR EACH ROW EXECUTE FUNCTION gerp_guard_quote_line_mutation();

COMMIT;
