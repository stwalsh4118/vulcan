# PBI-2: Backend Interface & Workload Lifecycle

[View in Backlog](../backlog.md)

## Overview

Define the common Go interface that all isolation backends implement, build the workload lifecycle state machine, and implement the local scheduler with auto-routing. This is the abstraction layer that makes Firecracker, V8, and gVisor interchangeable.

## Problem Statement

PBI-1 stores workload records but can't execute anything. Before adding real backends, the system needs a clear contract (Go interface) that backends must satisfy, a state machine that manages workload lifecycle, and a scheduler that picks the right backend. Without this, each backend would be a one-off integration.

## User Stories

- As a developer, I want a Backend interface so that I can implement new isolation backends without changing the API layer.
- As a user, I want async workload execution so that I can submit long-running tasks and poll for results.
- As a user, I want to stream logs from running workloads so that I can debug in real time.
- As a user, I want `auto` isolation mode so that the system picks the best backend for my workload.

## Technical Approach

- **Backend interface** in Go: `Execute(ctx, WorkloadSpec) (WorkloadResult, error)`, `Capabilities() BackendCapabilities`, `Cleanup(workloadID)`.
- **Workload lifecycle**: pending → running → completed | failed | killed. State transitions enforced in the store layer.
- **Local scheduler**: resolves `auto` mode (JS/TS → isolate, OCI ref → gVisor, archive → microVM), validates requested isolation is available, dispatches to backend.
- **Async execution**: `POST /v1/workloads/async` returns 202 with workload ID immediately. Execution runs in a goroutine. Client polls `GET /v1/workloads/:id`.
- **Log streaming**: `GET /v1/workloads/:id/logs` as SSE. Backends write log lines to a channel; the SSE handler reads and pushes them.
- **Timeout enforcement**: context deadline per workload, cancelled on expiry.
- **`GET /v1/backends`**: returns registered backends and their capabilities.
- **`GET /v1/stats`**: aggregate stats (counts, avg latency per backend).

## UX/UI Considerations

N/A — backend/infrastructure PBI. Adds new API endpoints that the frontend (PBI-3) will consume.

## Acceptance Criteria

1. `Backend` Go interface is defined and documented.
2. Workload status transitions are enforced (invalid transitions return errors).
3. `POST /v1/workloads/async` returns 202 and the workload executes asynchronously.
4. `GET /v1/workloads/:id` reflects real-time status updates during execution.
5. `GET /v1/workloads/:id/logs` streams log lines via SSE while a workload is running.
6. `auto` isolation mode correctly resolves to the appropriate backend based on runtime/format.
7. Workloads that exceed their timeout are killed and marked as `failed`.
8. `GET /v1/backends` returns the list of registered backends and their capabilities.
9. `GET /v1/stats` returns aggregate execution statistics.

## Dependencies

- **Depends on**: PBI-1 (Core API and persistence)
- **External**: None beyond PBI-1's dependencies

## Open Questions

- Should the scheduler support backend preference hints beyond `auto` (e.g., "prefer isolate but fall back to microvm")?
- Should log lines be persisted in SQLite or only streamed in real time?

## Related Tasks

[View Tasks](./tasks.md)
