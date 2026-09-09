# Gerp upstream influences and reuse policy

Gerp deliberately learns from strong permissively licensed Go business-software projects without becoming a source-level amalgamation of them.

## Reuse rule

There are two allowed integration modes:

1. **Dependency** — import a maintained upstream Go module through `go.mod`, preserve its license obligations, and keep a narrow adapter owned by Gerp.
2. **Clean-room pattern adoption** — study public behavior and architecture, then implement a Gerp-native design using Gerp naming, contracts, tests, schemas, and invariants. Do not copy source text, generated files, migrations, templates, or internal identifiers.

Any future source-level copy must be explicit in a pull request, preserve copyright/license notices, identify the exact upstream commit, and pass a license review.

## Influence matrix

| Upstream | License | Gerp adoption | Mode |
| --- | --- | --- | --- |
| `iota-uz/iota-sdk` | Apache-2.0 | module descriptors, composition root, explicit dependency graph, module-local UI/application/infrastructure boundaries | clean-room pattern |
| `HMB-research/open-accounting` | MIT | immutable accounting history, reversal-first corrections, accounting workflow boundaries, multi-tenant finance concepts | clean-room pattern |
| `aeml/open_crm` | MIT | bounded CRM operations, revision-aware writes, pipelines/stages, activity history, archive/recovery semantics, saved views | clean-room pattern |
| `invopop/gobl` | Apache-2.0 | invoice/business-document calculation, validation, tax/regime rules, electronic-document interoperability | direct dependency |
| `qor/qor` | MIT | resource metadata, explicit actions, state-machine-driven workflows, admin projection concepts | clean-room pattern |
| `go-admin-team/go-admin` | MIT | RBAC vocabulary, admin resource/code-generation metadata, menu/form/table generation ideas | clean-room pattern |

## What Gerp intentionally does differently

### Domain commands outrank generic CRUD

Metadata-driven administration is useful for low-risk master data. It is dangerous when applied blindly to ERP aggregates.

Every resource declares one of three mutation policies:

- `crud`: generated create/update/archive operations are allowed after authorization and validation.
- `commands_only`: all writes must route through named domain commands.
- `read_only`: metadata may generate browse/search/detail surfaces only.

Examples of `commands_only` resources include posted journals, deals with lifecycle side effects, stock movements, invoice issuance, payments, approvals, payroll runs, and period close.

### Modules are compile-time composition, not microservices

Gerp modules form an explicit dependency graph, but by default they execute inside one Go process and one PostgreSQL transaction boundary. A module boundary is an ownership boundary first. It becomes a network boundary only after measured operational need justifies extraction.

### State machines carry business intent

Lifecycle transitions are named operations such as `send`, `approve`, `post`, `reverse`, `win`, or `close`, not arbitrary status-field updates. Transition definitions carry permission and command metadata so the HTTP, CLI, admin, MCP, and automation surfaces can expose the same business capability without duplicating policy.

### CRM history is evidence, not mutable presentation state

Activities and audit events are append-only. Mutable projections such as a deal's current stage or a contact's owner may change, but the causal history remains independently queryable.

### GOBL is a compliance boundary

Gerp owns commercial lifecycle and accounting truth. GOBL owns the representation and validation of portable business documents. Gerp therefore stores both:

- normalized ERP records used for orders, invoices, receivables, payments, and reporting; and
- validated immutable GOBL envelope snapshots for external document exchange and compliance evidence.

The two are linked by stable IDs and hashes. A GOBL document never directly mutates the ledger.

## Current implementation

The first convergence slice introduces:

- `internal/platform/module`: deterministic module registration and dependency resolution.
- `internal/platform/resource`: metadata for fields, actions, permissions, tenant scope, auditing, and safe mutation policy.
- `internal/platform/workflow`: declarative state-machine transitions.
- `migrations/postgres/003_crm.sql`: tenant-safe CRM operational schema with revision counters and append-only activity history.
- `internal/compliance/gobl`: a narrow adapter around the upstream GOBL module.

## Next convergence slices

1. PostgreSQL unit-of-work and repository contracts shared by finance and CRM.
2. Finance posting/reversal repository with audit and outbox committed atomically.
3. CRM command service for pipeline movement, win/loss, owner changes, archive/restore, and activity evidence.
4. Sales quote/order state machines and quote-to-cash orchestration.
5. Invoice model plus GOBL snapshot generation, hashing, delivery, and jurisdiction-specific addons.
6. Admin renderer/code generator consuming Gerp resource metadata for HTMX/templ screens and APIs.
7. Permission catalog generation from module/resource/action metadata.

The long-term goal is not to reproduce six upstream projects. It is to converge their strongest ideas into one coherent ERP kernel with fewer architectural seams.
