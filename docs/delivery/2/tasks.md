# Tasks for PBI 2: Backend Interface & Workload Lifecycle

This document lists all tasks associated with PBI 2.

**Parent PBI**: [PBI 2: Backend Interface & Workload Lifecycle](./prd.md)

## Task Summary

| Task ID | Name | Status | Description |
| :------ | :--- | :----- | :---------- |
| 2-1 | [Backend interface & domain types](./2-1.md) | Proposed | Define Backend Go interface, WorkloadSpec, WorkloadResult, BackendCapabilities types; add IsolationAuto constant |
| 2-2 | [Workload state machine](./2-2.md) | Proposed | Add state transition validation to model, enforce in store; invalid transitions return errors |
| 2-3 | [Store extensions for execution](./2-3.md) | Proposed | Extend Store interface with UpdateWorkload (full update) and GetWorkloadStats; implement in SQLite |
| 2-4 | [Backend registry & auto-routing](./2-4.md) | Proposed | Backend registry for registration/lookup, auto-mode resolution logic, GET /v1/backends endpoint |
| 2-5 | [Async execution engine](./2-5.md) | Proposed | Goroutine-based executor with timeout enforcement; POST /v1/workloads/async returning 202; real-time status updates |
| 2-6 | [Log streaming via SSE](./2-6.md) | Proposed | Log broker with per-workload channels; GET /v1/workloads/:id/logs SSE endpoint |
| 2-7 | [Stats endpoint](./2-7.md) | Proposed | GET /v1/stats returning aggregate execution statistics per backend |
| 2-8 | [E2E CoS Test](./2-8.md) | Proposed | End-to-end tests verifying all 9 acceptance criteria for PBI 2 |

## Dependency Graph

```
  2-1 (Backend interface)     2-2 (State machine)
       │                           │
       ├───────────┐               │
       │           │               │
       v           v               v
  2-4 (Registry)  2-3 (Store extensions)
       │           │
       │           │
       v           v
      2-5 (Async execution engine)
           │
           v
      2-6 (Log streaming SSE)
           │
           v
      2-7 (Stats endpoint)
           │
           v
      2-8 (E2E CoS Test)
```

## Implementation Order

1. **2-1** — Backend interface & domain types (no dependencies; foundational types all other tasks need)
2. **2-2** — Workload state machine (no dependencies; uses existing model constants)
3. **2-3** — Store extensions for execution (depends on 2-2 for transition validation)
4. **2-4** — Backend registry & auto-routing (depends on 2-1 for Backend interface)
5. **2-5** — Async execution engine (depends on 2-1, 2-3, 2-4; ties together backend, store, registry)
6. **2-6** — Log streaming via SSE (depends on 2-5; integrates with execution engine)
7. **2-7** — Stats endpoint (depends on 2-3; wires store stats to API)
8. **2-8** — E2E CoS Test (depends on all above; validates full acceptance criteria)

## Complexity Ratings

| Task ID | Complexity | External Packages |
|---------|-----------|-------------------|
| 2-1 | Simple | None |
| 2-2 | Medium | None |
| 2-3 | Medium | None |
| 2-4 | Medium | None |
| 2-5 | Complex | None |
| 2-6 | Complex | None |
| 2-7 | Simple | None |
| 2-8 | Complex | None |

## External Package Research Required

None — all functionality uses Go standard library and existing dependencies (chi, SQLite, Prometheus).
