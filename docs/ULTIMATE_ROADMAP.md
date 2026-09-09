# Gerp — Ultimate ERP roadmap

This roadmap converts the existing GERP prototype into a production-oriented, permissively licensed Go ERP without discarding working domain code unnecessarily.

## Product thesis

Gerp should compete on five axes:

1. **Correctness:** finance, inventory, purchasing, sales, payroll, and asset operations enforce explicit invariants.
2. **Low maintenance:** one Go application and PostgreSQL are enough for the default deployment.
3. **Extensibility:** modules, typed custom fields, workflow policies, webhooks, and stable APIs let vertical products extend Gerp without forking the kernel.
4. **Interoperability:** accounting/tax documents, payments, identity, storage, messaging, and productivity-suite integrations are adapters rather than hard dependencies.
5. **Agent-ready operation:** MCP/AI features operate through the same permissioned application services as human UIs. Agents never bypass authorization or domain rules.

## Phase 0 — architectural convergence

Goal: establish the durable platform contract before expanding feature count.

- [x] Accept PostgreSQL-first modular-monolith ADR.
- [x] Extract storage-independent finance journal validation.
- [x] Add PostgreSQL tenant/RBAC/audit/outbox schema.
- [x] Add PostgreSQL immutable GL schema.
- [ ] Add `pgx` connection/pool adapter and transaction unit-of-work abstraction.
- [ ] Add migration runner and repeatable local Postgres dev environment.
- [ ] Add mandatory tenant context at HTTP/application/repository boundaries.
- [ ] Add CSRF and idempotency middleware.
- [ ] Persist audit events and outbox messages in the same transaction as business changes.
- [ ] Add structured logging, OpenTelemetry hooks, health/readiness, and metrics.
- [ ] Keep Spanner behind a legacy adapter until parity tests pass.

Exit gate: finance kernel and one commercial flow run entirely on PostgreSQL with invariant, tenant-isolation, and retry tests.

## Phase 1 — accounting kernel

### General ledger

- chart of accounts + templates
- journals, reversals, recurring journals
- fiscal years/periods and hard/soft close
- dimensions: cost center, department, project, location
- trial balance, general ledger, balance sheet, income statement, cash-flow statement
- multi-currency with explicit FX-rate sources and realized/unrealized gain/loss
- audit trail for every posting action

### Accounts receivable

- customers, terms, credit limits
- quotes -> orders -> invoices
- credit/debit notes
- receipts, allocations, refunds
- aging and statements
- dunning hooks

### Accounts payable

- vendors, bills, payment terms
- three-way match hooks
- approvals
- payment runs
- AP aging

### Banking

- bank/cash accounts
- statement import
- reconciliation engine
- payment-provider adapters

Exit gate: double-entry accounting passes property/invariant tests and all posted entries are immutable/reversible.

## Phase 2 — commercial ERP

### CRM

- companies, people, leads, opportunities
- pipelines/stages
- tasks, notes, activity history
- territories and ownership
- quoting handoff

### Sales

- price lists
- quotes
- sales orders
- reservations
- fulfillment
- returns/RMAs
- invoicing integration
- commissions hooks

### Procurement

- vendor catalogs
- purchase requisitions
- RFQ/comparison
- purchase orders
- receipts
- vendor bills
- three-way matching
- returns

### Inventory / WMS

- products/SKUs/variants
- warehouses, locations and bins
- immutable stock movement ledger
- reservations and allocations
- lots/serials
- cycle counts
- transfers
- FIFO and moving-average costing policies
- landed cost

Exit gate: quote-to-cash and procure-to-pay operate end-to-end using one transactional core.

## Phase 3 — configurable platform

- typed custom fields and field groups
- module registry/manifests
- saved views, filters and columns
- report/query framework with tenant-safe query definitions
- import/export jobs
- S3/R2 document storage adapter
- comments, mentions and notifications
- webhook subscriptions + signed delivery
- PostgreSQL-backed durable job queue
- approval policy engine
- scheduled automation
- email/calendar integrations
- GOBL-compatible invoice/document adapter

UI default:

- Go server-rendered pages for ordinary ERP screens
- HTMX for navigation/actions/fragments
- Alpine.js for ephemeral UI state only
- isolated rich-client islands for planners, maps, editors, canvases, or offline-heavy surfaces

## Phase 4 — operations ERP

### Manufacturing

- BOMs and versions
- routings/work centers
- MRP
- work orders
- consumption/output
- quality inspections
- subcontracting

### Projects / services

- projects/tasks/milestones
- time tracking
- expenses
- budgets
- project billing
- profitability

### Enterprise assets

- asset registry
- capitalization
- depreciation books
- maintenance plans/work orders
- inspections
- disposal

### HCM

- employee directory
- positions/teams
- leave/time
- expenses
- payroll interfaces
- onboarding/offboarding workflows

## Phase 5 — enterprise capabilities

- OIDC + SAML SSO
- SCIM
- delegated administration
- fine-grained authorization adapter if RBAC becomes insufficient
- intercompany transactions
- consolidation/eliminations
- multiple books / local GAAP overlays
- localization packs
- tax packs
- retention/legal hold
- tenant-level backup/restore
- CDC/data warehouse feeds
- read replicas/partitioning where measured

## Phase 6 — ecosystem and agent layer

- stable module SDK
- plugin permission manifest
- CLI/admin automation SDK
- MCP server mapped to permissioned application commands/queries
- agent audit identity
- human approval gates for high-impact agent actions
- workflow copilots
- module marketplace governance

## Dependency policy

Prefer permissively licensed dependencies. Every new dependency must justify:

- capability not reasonably implemented in the standard library/core stack;
- active maintenance/security posture;
- license compatibility;
- operational cost;
- transitive dependency burden.

High-value candidates include PostgreSQL, pgx, sqlc, HTMX, Alpine.js, River, GOBL, and PostGIS where applicable. Temporal/Restate/NATS/Kafka remain optional infrastructure, not baseline requirements.

## Explicit anti-goals

Gerp will not:

- split domains into microservices merely to demonstrate DDD;
- use distributed sagas where one ACID transaction is sufficient;
- use binary floating point for money;
- let clients or AI agents enforce authoritative business rules;
- use arbitrary JSON custom fields for invariants that deserve typed columns/models;
- require Kubernetes, Kafka, Temporal, Redis, or a JS server to run a normal installation;
- sacrifice relational integrity for hypothetical scale.
