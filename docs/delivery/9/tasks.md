# Tasks for PBI 9: Log Persistence & Historical Log Viewing

This document lists all tasks associated with PBI 9.

**Parent PBI**: [PBI 9: Log Persistence & Historical Log Viewing](./prd.md)

## Task Summary

| Task ID | Name | Status | Description |
| :------ | :--- | :----- | :---------- |
| 9-1 | [LogLine Model & Store Interface Extension](./9-1.md) | Proposed | Add LogLine struct to model package and extend Store interface with InsertLogLine/GetLogLines methods |
| 9-2 | [SQLite Log Persistence Implementation](./9-2.md) | Proposed | Add log_lines table migration and implement InsertLogLine/GetLogLines on SQLiteStore |
| 9-3 | [Engine Dual-Write to LogBroker & SQLite](./9-3.md) | Proposed | Modify LogWriter callback to persist log lines to SQLite alongside LogBroker publish |
| 9-4 | [Historical Logs API Endpoint](./9-4.md) | Proposed | Add GET /v1/workloads/:id/logs/history endpoint returning persisted log lines as JSON |
| 9-5 | [Frontend Historical Log Viewing](./9-5.md) | Proposed | Update useLogStream hook and LogViewer to fetch and display persisted logs for terminal workloads |
| 9-6 | [E2E CoS Test](./9-6.md) | Proposed | End-to-end tests verifying all PBI 9 acceptance criteria |

## Dependency Graph

```
9-1 (Model & Interface)
 │
 ▼
9-2 (SQLite Implementation)
 │
 ├──────────────┐
 ▼              ▼
9-3 (Engine)   9-4 (API Endpoint)
 │              │
 └──────┬───────┘
        ▼
       9-5 (Frontend)
        │
        ▼
       9-6 (E2E CoS Test)
```

## Implementation Order

1. **9-1** — LogLine Model & Store Interface Extension (foundation: types and interface contract)
2. **9-2** — SQLite Log Persistence Implementation (depends on 9-1: implements the interface)
3. **9-3** — Engine Dual-Write (depends on 9-2: uses InsertLogLine to persist during execution)
4. **9-4** — Historical Logs API Endpoint (depends on 9-2: uses GetLogLines to serve history)
5. **9-5** — Frontend Historical Log Viewing (depends on 9-3 + 9-4: needs backend producing & serving persisted logs)
6. **9-6** — E2E CoS Test (depends on all above: verifies the full stack)

Note: Tasks 9-3 and 9-4 could be implemented in parallel since they both depend only on 9-2, but sequential order is maintained for the one-task-at-a-time rule.

## Complexity Ratings

| Task ID | Complexity | External Packages |
|---------|-----------|-------------------|
| 9-1 | Simple | None |
| 9-2 | Medium | None |
| 9-3 | Medium | None |
| 9-4 | Medium | None |
| 9-5 | Complex | None |
| 9-6 | Complex | None |

## External Package Research Required

None — this PBI uses existing SQLite (modernc.org/sqlite), chi router, and Next.js/React infrastructure.
