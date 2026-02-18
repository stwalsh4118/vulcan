# Tasks for PBI 1: Project Foundation & Core API

This document lists all tasks associated with PBI 1.

**Parent PBI**: [PBI 1: Project Foundation & Core API](./prd.md)

## Task Summary

| Task ID | Name | Status | Description |
| :------ | :--- | :----- | :---------- |
| 1-1 | [Go module & project structure](./1-1.md) | Proposed | Initialize Go module, create directory layout, and produce a compilable main.go |
| 1-2 | [Configuration & structured logging](./1-2.md) | Proposed | Config struct loaded from env vars, structured JSON logger to stdout |
| 1-3 | [Workload domain types](./1-3.md) | Proposed | Workload struct, status/isolation/runtime constants, ULID ID generation |
| 1-4 | [SQLite persistence layer](./1-4.md) | Proposed | SQLite connection, workloads table migration, and full CRUD operations |
| 1-5 | [HTTP server with chi & middleware](./1-5.md) | Proposed | chi router setup, middleware stack, graceful shutdown |
| 1-6 | [Health & metrics endpoints](./1-6.md) | Proposed | GET /healthz and GET /metrics with Prometheus |
| 1-7 | [Workload API endpoints](./1-7.md) | Proposed | POST, GET (single + list), DELETE workload endpoints with validation |
| 1-8 | [E2E CoS Test](./1-8.md) | Proposed | End-to-end verification of all PBI 1 acceptance criteria |

## Dependency Graph

```
  1-1 (Go module & project structure)
   ├──→ 1-2 (Configuration & structured logging)
   │      └──→ 1-5 (HTTP server with chi & middleware)
   │             ├──→ 1-6 (Health & metrics endpoints)
   │             └──→ 1-7 (Workload API endpoints) ←── 1-4
   └──→ 1-3 (Workload domain types)
          └──→ 1-4 (SQLite persistence layer)

  1-6 ──→ 1-8 (E2E CoS Test) ←── 1-7
```

## Implementation Order

1. **1-1** — Go module & project structure (no dependencies — foundational scaffolding)
2. **1-2** — Configuration & structured logging (depends on 1-1 — project must exist)
3. **1-3** — Workload domain types (depends on 1-1 — project must exist)
4. **1-4** — SQLite persistence layer (depends on 1-3 — needs domain types for CRUD)
5. **1-5** — HTTP server with chi & middleware (depends on 1-2 — needs config for listen addr)
6. **1-6** — Health & metrics endpoints (depends on 1-5 — needs router to register on)
7. **1-7** — Workload API endpoints (depends on 1-4 + 1-5 — needs store + router)
8. **1-8** — E2E CoS Test (depends on 1-6 + 1-7 — needs all endpoints functional)

## Complexity Ratings

| Task ID | Complexity | External Packages |
| :------ | :--------- | :---------------- |
| 1-1 | Simple | None (go mod init only) |
| 1-2 | Simple | `log/slog` (stdlib) |
| 1-3 | Simple | `github.com/oklog/ulid/v2` |
| 1-4 | Medium | `modernc.org/sqlite` |
| 1-5 | Medium | `github.com/go-chi/chi/v5`, `github.com/go-chi/cors` |
| 1-6 | Simple | `github.com/prometheus/client_golang` |
| 1-7 | Complex | None (uses packages from earlier tasks) |
| 1-8 | Medium | `net/http` (stdlib, test client) |

## External Package Research Required

| Task ID | Package | Guide Document |
| :------ | :------ | :------------- |
| 1-3 | `github.com/oklog/ulid/v2` | `1-3-ulid-guide.md` |
| 1-4 | `modernc.org/sqlite` | `1-4-sqlite-guide.md` |
| 1-5 | `github.com/go-chi/chi/v5` | `1-5-chi-guide.md` |
| 1-6 | `github.com/prometheus/client_golang` | `1-6-prometheus-guide.md` |
