# PBI-1: Project Foundation & Core API

[View in Backlog](../backlog.md)

## Overview

Establish the Go project structure, HTTP server, persistence layer, and core API endpoints that every subsequent PBI builds upon. This is the foundation — a running server that accepts requests, stores workload records, and exposes health/metrics.

## Problem Statement

No code exists yet. Every feature in Vulcan — backends, frontend, CLI — depends on a working HTTP API server with persistence. Without this foundation, nothing else can be built or tested.

## User Stories

- As a developer, I want a well-structured Go project so that I can add features incrementally without refactoring the base.
- As a developer, I want HTTP endpoints for workload CRUD so that frontends and CLIs have a stable API to target.
- As an operator, I want health and metrics endpoints so that I can verify the server is running and monitor it.

## Technical Approach

- **Go module** with `go 1.23+`, organized by domain (`cmd/`, `internal/api/`, `internal/store/`, `internal/config/`).
- **chi router** for HTTP with middleware (request ID, logging, recovery, CORS).
- **SQLite** via `modernc.org/sqlite` (pure Go, no CGO) for workload persistence.
- **Workload table** per the PRD data model (id, status, isolation, runtime, node_id, input_hash, output, exit_code, error, resource limits, timestamps).
- **ULID** generation for workload IDs.
- **Prometheus** metrics via `client_golang` at `/metrics`.
- **Structured JSON logging** to stdout.
- **Configuration** via environment variables and/or config file (listen address, DB path, log level).

## UX/UI Considerations

N/A — backend/infrastructure PBI. API responses follow the JSON format defined in the PRD.

## Acceptance Criteria

1. `go build ./cmd/vulcan` produces a single binary that starts an HTTP server.
2. `GET /healthz` returns 200 with server status.
3. `GET /metrics` returns Prometheus-formatted metrics.
4. `POST /v1/workloads` accepts a workload request body and stores it in SQLite (returns 201 with workload record — status will be `pending` since no backends exist yet).
5. `GET /v1/workloads/:id` retrieves a stored workload by ID.
6. `GET /v1/workloads` lists workloads with pagination.
7. `DELETE /v1/workloads/:id` marks a workload as killed.
8. Structured JSON logs are written to stdout on every request.
9. Server is configurable via environment variables (at minimum: listen address, DB path).

## Dependencies

- **Depends on**: None
- **External**: `github.com/go-chi/chi/v5`, `modernc.org/sqlite`, `github.com/prometheus/client_golang`, `github.com/oklog/ulid/v2`

## Open Questions

- Should configuration support a YAML/TOML config file in addition to env vars, or is env-only sufficient for MVP?

## Related Tasks

_Tasks will be created when this PBI moves to Agreed via `/plan-pbi 1`._
