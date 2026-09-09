# ADR-001: PostgreSQL-first modular monolith as the Gerp default

- Status: Accepted
- Date: 2026-09-08
- Scope: Core ERP transaction architecture

## Context

The original GERP prototype made Cloud Spanner, cross-domain soft references, GraphQL fan-out, and Temporal sagas foundational. That architecture demonstrates distributed systems patterns, but it imposes distributed consistency and operational costs on business transactions that normally benefit from one ACID boundary: journal posting, inventory movement, order confirmation, invoicing, purchasing, and settlement.

Gerp is intended to become a broadly deployable, low-maintenance, permissively licensed ERP. The default architecture therefore must optimize first for correctness, auditability, portability, developer productivity, and inexpensive operation. Horizontal distribution remains available when a measured requirement justifies it.

## Decision

Gerp will use a **modular Go monolith backed by PostgreSQL** as its primary architecture.

### Core rules

1. Domain packages own business invariants and do not import database or transport packages.
2. PostgreSQL is the default authoritative store for transactional ERP state.
3. One business command should use one database transaction whenever the affected state lives inside Gerp.
4. Cross-module foreign keys are allowed when they protect an invariant inside the same database. Package ownership remains explicit even when relational integrity spans modules.
5. Every tenant-owned record is tenant scoped. Repository methods require tenant context; PostgreSQL RLS may be used as defense in depth, not as a replacement for application predicates.
6. Financial postings are immutable after posting. Corrections use reversals and replacement entries.
7. Inventory quantities are derived from an immutable movement ledger rather than edited balance fields.
8. External side effects use a transactional outbox and idempotent consumers.
9. Durable workflow engines are optional adapters for truly long-running, cross-system processes. They are not required for ordinary ERP transactions.
10. HTMX + server-rendered HTML is the default UI model for forms, CRUD, workflow, tables, and administration. Alpine.js owns ephemeral browser state only. Rich maps/editors/canvases may use isolated client-side islands.

## Target topology

```text
Browser
  |
  | HTML / HTMX / JSON where appropriate
  v
Go HTTP adapters
  |
  v
Application services
  |
  +---------+---------+---------+---------+
  |         |         |         |         |
Finance   Sales      SCM       CRM     Workflow
  |         |         |         |         |
  +---------+---------+---------+---------+
                    |
              Domain ports
                    |
             PostgreSQL UoW
             /      |       \
          audit   outbox    jobs
                    |
             external adapters
```

## Why PostgreSQL

ERP workloads are relational, transactional, reporting-heavy, and constraint-rich. PostgreSQL provides mature ACID transactions, foreign keys, row-level security, JSONB for controlled extensibility, full-text search, materialized views, advisory locks, LISTEN/NOTIFY, extensions, and a broad operational ecosystem. PostGIS can serve geospatial ERP extensions without introducing a separate system.

## Persistence style

Prefer explicit SQL through `pgx` and either `sqlc` or small hand-written query packages. Reflection-heavy ORMs are not forbidden, but finance/inventory hot paths should keep SQL and transaction boundaries visible.

## Eventing and jobs

Use a transactional outbox for integration events. Begin with a PostgreSQL-backed job queue; River is a strong candidate. Introduce NATS/Kafka only when independent consumers or throughput justify them. Introduce Temporal/Restate only when workflows truly cross durable external boundaries and need replay/retry/compensation semantics.

## Service extraction criteria

A module should become a service only when at least one of the following is demonstrated:

- materially different scaling requirements;
- a compliance/security boundary requires isolation;
- independent deployment ownership provides concrete value;
- a different runtime is justified by the workload;
- failure isolation benefit exceeds the added consistency/operations cost.

Service extraction must preserve domain contracts and use versioned events/APIs.

## Migration from the prototype

The existing Spanner, Temporal, GraphQL, and MCP code remains usable during migration. The migration is strangler-style:

1. extract storage-independent invariants from current domain services;
2. add tenant-aware PostgreSQL schemas and repositories;
3. move ordinary transactions to PostgreSQL units of work;
4. move integration side effects to outbox workers;
5. retain Temporal only for workflows that still justify it;
6. treat GraphQL and MCP as adapters over application services rather than architectural centers;
7. remove legacy Spanner-only assumptions after parity tests pass.

## Consequences

### Positive

- lower deployment and cloud complexity;
- stronger local transaction guarantees;
- easier self-hosting and local development;
- fewer mandatory infrastructure dependencies;
- clearer finance/inventory correctness model;
- easier future use as a reusable Go ERP framework.

### Negative

- a single PostgreSQL cluster becomes an important scaling/failure domain;
- some eventual-service boundaries are postponed;
- large tenants may eventually require partitioning, read replicas, CDC, or selective extraction.

These are acceptable tradeoffs. They solve problems when measured rather than prepaying for them.
