# Tasks for PBI 10: Real-Time Log Streaming Fixes

This document lists all tasks associated with PBI 10.

**Parent PBI**: [PBI 10: Real-Time Log Streaming Fixes](./prd.md)

## Task Summary

| Task ID | Name | Status | Description |
| :------ | :--- | :----- | :---------- |
| 10-1 | [Add `event: done` SSE termination to Go backend](./10-1.md) | Review | Send explicit `event: done` SSE event before closing the log stream when a workload reaches terminal state; update API spec |
| 10-2 | [Create Next.js streaming API route for SSE proxy](./10-2.md) | Review | Add a Next.js App Router route handler that pipes SSE from the Go backend to the browser without buffering |
| 10-3 | [Update useLogStream hook for clean shutdown](./10-3.md) | Review | Handle `done` event to close EventSource cleanly; remove blind onerror reconnect logic |
| 10-4 | [E2E CoS Test — Real-time streaming and clean shutdown](./10-4.md) | Review | Verify real-time log delivery, done event, clean connection teardown, and no regressions |

## Dependency Graph

```
10-1 (Go: done event) ──┐
                         ├──→ 10-3 (Frontend: hook update) ──→ 10-4 (E2E CoS Test)
10-2 (Next.js: SSE route)┘
```

## Implementation Order

1. **10-1** — Backend `done` event (no dependencies; foundational for frontend and tests)
2. **10-2** — Next.js streaming API route (no dependencies; independent of 10-1)
3. **10-3** — Frontend hook update (depends on 10-1 for `done` event; depends on 10-2 for streaming route)
4. **10-4** — E2E CoS Test (depends on all prior tasks)

Tasks 10-1 and 10-2 are independent and could be implemented in parallel.

## Complexity Ratings

| Task ID | Complexity | External Packages |
|---------|------------|-------------------|
| 10-1 | Simple | None |
| 10-2 | Medium | None |
| 10-3 | Simple | None |
| 10-4 | Medium | None |

## External Package Research Required

None — all changes use existing packages and standard Web APIs (ReadableStream, EventSource).
