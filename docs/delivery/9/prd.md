# PBI-9: Log Persistence & Historical Log Viewing

[View in Backlog](../backlog.md)

## Overview

Store workload log lines in SQLite as they are produced during execution, and expose them via a new API endpoint so the frontend can display logs for completed workloads. Currently, logs are only streamed in real-time via SSE through the in-memory LogBroker and are lost once a workload finishes. This PBI adds persistence to close that gap while keeping real-time SSE streaming for active workloads.

## Problem Statement

**Current state**: The LogBroker is an in-memory pub/sub system. Log lines are published during execution and streamed to connected SSE clients. Once a workload reaches a terminal state (completed, failed, killed), the log topic is closed and all lines are gone. If a user navigates to a completed workload's detail page, the log viewer shows "Disconnected" with no log content.

**Desired state**: Log lines are durably stored in SQLite alongside the workload record. A new API endpoint returns historical log lines for any workload. The frontend detail page detects whether a workload is active (use SSE) or terminal (fetch persisted logs), providing a seamless viewing experience regardless of when the user opens the page.

## User Stories

- As a user, I want to view workload logs after execution completes so that I can debug and review past workloads without needing to watch them in real time.
- As a user, I want the log viewer to show logs seamlessly whether I open the page during or after execution so that I don't have to worry about timing.

## Technical Approach

### Database

Add a `log_lines` table to SQLite:

```sql
CREATE TABLE IF NOT EXISTS log_lines (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workload_id TEXT NOT NULL REFERENCES workloads(id),
    seq         INTEGER NOT NULL,
    line        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_log_lines_workload ON log_lines(workload_id, seq);
```

### Store Layer

Extend the `Store` interface with:
- `InsertLogLine(ctx, workloadID, seq, line string) error` — insert a single log line
- `GetLogLines(ctx, workloadID string) ([]LogLine, error)` — retrieve all log lines for a workload, ordered by sequence number

### Engine

Modify the `LogWriter` callback in `engine.go` to dual-write: publish to LogBroker (for real-time SSE) AND persist to SQLite. Use a sequence counter per workload to maintain ordering.

### API

Add a new endpoint:
- `GET /v1/workloads/:id/logs/history` — returns persisted log lines as JSON array

Or alternatively, modify the existing `GET /v1/workloads/:id/logs` endpoint behavior:
- For active workloads: SSE stream (current behavior)
- For terminal workloads: return JSON array of persisted log lines (new behavior, based on Accept header or query parameter)

The simpler approach (new `/history` endpoint) is recommended to avoid breaking the existing SSE contract.

### Frontend

Update the workload detail page and `useLogStream` hook:
- For active workloads: continue using EventSource SSE (current behavior)
- For terminal workloads: fetch from the history endpoint and display in the same LogViewer component
- Handle the transition: if a workload becomes terminal while streaming, fetch the full history to ensure no lines were missed

## UX/UI Considerations

- The log viewer should look identical whether showing live or historical logs
- No "Disconnected" message for completed workloads — show the persisted logs instead
- For active workloads, the existing "Connected" / auto-scroll behavior continues unchanged
- A subtle indicator could differentiate live vs. historical mode (e.g., "Live" vs. "Complete — N log lines")

## Acceptance Criteria

1. Log lines are persisted to SQLite as they are produced during workload execution.
2. A new API endpoint (`GET /v1/workloads/:id/logs/history`) returns all persisted log lines for a workload as a JSON array.
3. The frontend detail page displays persisted logs for completed/failed/killed workloads.
4. Real-time SSE streaming continues to work for active (pending/running) workloads.
5. The log viewer seamlessly handles the transition from live streaming to historical display when a workload completes.
6. Persisted logs are ordered by sequence number and display in the correct order.

## Dependencies

- **Depends on**: PBI 2 (Done — LogBroker, SSE endpoint), PBI 3 (InProgress — frontend detail page, LogViewer component)
- **Blocks**: None
- **External**: None (uses existing SQLite, no new packages)

## Open Questions

- Should there be a retention policy for old log lines (e.g., auto-delete after N days or after workload count exceeds a threshold)? Likely not for MVP — SQLite handles moderate data volumes well.
- Should the history endpoint support pagination for workloads with very large log output? Start simple (return all lines), add pagination if needed.

## Related Tasks

_Tasks will be created when this PBI is approved via `/plan-pbi 9`._
