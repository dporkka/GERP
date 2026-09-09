# Gerp

**Gerp** is a Go-first, permissively licensed enterprise resource planning platform designed to deliver broad ERP capability without making distributed systems infrastructure mandatory.

The project is converging on a **modular Go monolith + PostgreSQL** core with server-rendered/HTMX-first business UI, explicit domain invariants, transactional audit/outbox primitives, and optional adapters for GraphQL, MCP, durable workflow engines, cloud databases, and rich client-side application surfaces.

> Status: active architecture convergence. The repository contains an earlier Spanner/Temporal distributed prototype alongside the new PostgreSQL-first foundation. Legacy adapters remain during the migration so working domain code can be preserved while transaction boundaries are simplified.

## Principles

- **Correctness before distribution.** Finance, inventory, sales, purchasing, and settlement use ACID transactions when they live inside Gerp.
- **Domain rules are storage-independent.** Business invariants live in Go domain packages, not handlers, GraphQL resolvers, Temporal workflows, or database adapters.
- **PostgreSQL is the default system of record.** Relational integrity is a feature for ERP workloads, not a limitation to remove prematurely.
- **Multi-tenant by construction.** Tenant identity belongs at application and repository boundaries, with database policies available as defense in depth.
- **Immutable business facts.** Posted journals and stock movements are append/reversal oriented and auditable.
- **Integrations are reliable.** Transactional outbox + idempotency are baseline primitives.
- **Low operational burden.** A normal Gerp installation should not require Kubernetes, Kafka, Temporal, Redis, or a Node.js application server.
- **Agent-ready, not agent-bypassed.** MCP/AI actions use the same authorization and domain commands as human operators.
- **Permissive ecosystem.** Prefer MIT/Apache/BSD-style dependencies and small, well-maintained building blocks.

## Target architecture

```text
Browser
  |
  | HTML + HTMX; JSON/API where useful
  v
Go transport adapters
  |
  v
Application services
  |
  +-----------+-----------+-----------+-----------+
  |           |           |           |           |
Finance      CRM        Sales        SCM       Workflow
  |           |           |           |           |
  +-----------+-----------+-----------+-----------+
                          |
                     Domain ports
                          |
                  PostgreSQL unit of work
                   /        |         \
                audit     outbox      jobs
                          |
                  external adapters
```

Ordinary ERP screens should default to Go-rendered HTML + HTMX, with Alpine.js for small ephemeral UI state. Rich maps, planners, visual editors, offline applications, and collaborative canvases can be isolated Svelte/React/Web Component islands without moving authoritative business rules into the browser.

See [`docs/architecture/ADR-001-modular-monolith-postgres.md`](docs/architecture/ADR-001-modular-monolith-postgres.md).

## ERP coverage

The repository already contains domain packages covering or prototyping:

- Finance
- Human capital management
- Supply chain
- Enterprise asset management
- Legal/compliance
- Revenue/CRM
- Learning/compliance training
- Master data
- Content/knowledge management (COAMS)
- IAM
- Pipeline/workflow
- MCP/agent interfaces

The implementation roadmap expands and normalizes those domains into a coherent ERP kernel covering:

- general ledger, AR/AP, banking, close and reporting
- CRM and opportunity management
- quote-to-cash
- procure-to-pay
- inventory/WMS
- manufacturing/MRP
- projects/services
- assets/maintenance
- HCM/payroll integrations
- workflow/approvals
- enterprise SSO/SCIM/localization/consolidation

See [`docs/ULTIMATE_ROADMAP.md`](docs/ULTIMATE_ROADMAP.md).

## Finance integrity

The finance package now has storage-independent journal validation. A journal must:

- contain at least two lines;
- contain no nil/zero-amount lines;
- reference explicit line/account IDs;
- contain no duplicate line IDs;
- avoid integer overflow;
- balance exactly to zero before persistence.

The legacy Spanner service calls the same invariant function that future PostgreSQL adapters will call. This is the migration pattern for the rest of the ERP: **extract invariants first, then replace infrastructure behind stable domain contracts**.

## PostgreSQL foundation

New PostgreSQL migrations live under [`migrations/postgres`](migrations/postgres):

- `001_core.sql` — tenants, users, memberships, RBAC, idempotency, audit and transactional outbox
- `002_finance.sql` — chart of accounts, fiscal periods, immutable journals and ledger lines

The next implementation milestone is the `pgx` unit-of-work/repository layer, followed by a complete finance vertical slice on PostgreSQL.

## Existing distributed adapters

The original prototype uses:

- Cloud Spanner
- Temporal
- GraphQL/gqlgen
- MCP
- Google APIs

These are no longer architectural requirements. They remain useful where justified:

- Spanner can remain an optional scale adapter until removed or proven necessary for specific installations.
- Temporal can remain for truly durable, cross-system workflows.
- GraphQL can remain a client/API adapter.
- MCP remains a first-class agent interface over permissioned application services.

## Development

The current module requires the Go version declared in `go.mod`.

```bash
go test ./...
go vet ./...
go build ./...
```

The repository CI runs formatting, vet, tests, and build checks on pull requests.

### Legacy local matrix

Existing prototype tooling remains available while migration proceeds:

```bash
make up
make init-db
go run ./cmd/seed/main.go
make run-worker
make run-gateway
```

Do not add new baseline features that require the legacy distributed stack unless an ADR documents why a normal PostgreSQL transaction/job/outbox cannot satisfy the requirement.

## Migration sequence

1. Extract business invariants from infrastructure-coupled services.
2. Add tenant-aware PostgreSQL schemas and repository contracts.
3. Move finance to PostgreSQL with parity/invariant tests.
4. Move inventory and commercial order flows into the same transactional core.
5. Add outbox-backed integrations and a PostgreSQL job queue.
6. Build the server-rendered/HTMX application shell.
7. Retain Temporal/GraphQL/MCP/cloud adapters where they add measurable value.
8. Remove Spanner-only and cross-domain-saga assumptions after parity gates pass.

## License

MIT. See [`LICENSE`](LICENSE).
