# PBI-10: Real-Time Log Streaming Fixes

[View in Backlog](../backlog.md)

## Overview

Fix two related SSE log streaming bugs that prevent real-time log delivery to the browser and cause connection leaks after workload completion. The Next.js `rewrites()` proxy buffers the entire SSE response so log lines arrive all at once, and the frontend `useLogStream` hook's reconnect logic creates connection accumulation when the backend closes streams without an explicit termination event.

## Problem Statement

**Current state**: Log lines don't stream in real time to the browser — they all arrive at once when the workload finishes. Additionally, after running multiple workloads, stale `/logs` SSE connections accumulate because the `onerror` reconnect logic fires when the backend closes the connection on workload completion, creating reconnect loops before the 2-second status poll detects the terminal state.

**Desired state**: Log lines appear in the browser immediately as they are produced. SSE connections close cleanly when a workload reaches terminal state with no reconnect loops or leaked connections.

**Root causes**:
1. The Next.js `rewrites()` proxy (`next.config.ts`) buffers the upstream SSE response instead of streaming it through. Confirmed: `curl -N` directly to `:8080` streams lines correctly.
2. The Go SSE handler (`api/internal/api/logs.go`) closes the stream without sending a termination event, so the browser `EventSource` API fires `onerror` (indistinguishable from a real error), triggering up to 3 reconnect attempts before the status poll catches up.

## User Stories

- As a user, I want to see workload log lines appear in real time so that I can monitor execution progress as it happens.
- As a user, I want log connections to clean up automatically when a workload finishes so that the browser doesn't accumulate stale connections.

## Technical Approach

### Fix 1: Eliminate proxy buffering (follow-up #12)

Replace the `rewrites()` proxy for the SSE endpoint with a **custom Next.js API route handler** that fetches the upstream SSE stream from the Go backend and pipes it to the client as a `ReadableStream`. This keeps the single-origin architecture (no direct browser-to-`:8080` connection) while giving full control over response streaming.

**Why not direct connection to :8080?** Couples the frontend to the backend's deployment port. Breaks in any non-localhost deployment and exposes the Go server directly.

**Why not WebSocket?** Overkill for one-way log streaming. Requires significant refactoring on both backend (new WebSocket handler) and frontend (replace `EventSource` with WebSocket client). SSE is the right tool for server-to-client streaming.

### Fix 2: Explicit stream termination (follow-up #13)

Add a `done` SSE event type to the Go backend that is sent before closing the stream when a workload reaches terminal state. Update the frontend hook to listen for this event and close the connection gracefully instead of treating the stream closure as an error.

### Components affected

| Component | File(s) | Change |
|-----------|---------|--------|
| Go SSE handler | `api/internal/api/logs.go` | Send `event: done` before closing |
| Next.js config | `web/next.config.ts` | Remove SSE path from rewrites (or keep non-SSE paths) |
| Next.js API route | `web/src/app/api/v1/workloads/[id]/logs/route.ts` (new) | Streaming proxy for SSE |
| Frontend hook | `web/src/hooks/useLogStream.ts` | Handle `done` event, remove blind reconnect |
| API specs | `docs/api-specs/core/core-api.md` | Document `done` event |

## UX/UI Considerations

No visual changes. The log viewer already renders lines as they arrive — this PBI fixes the delivery mechanism so lines actually arrive in real time instead of batched.

## Acceptance Criteria

1. Log lines from a running workload appear in the browser within 1 second of being produced by the backend.
2. The Go SSE endpoint sends an `event: done` SSE event before closing the stream when a workload reaches terminal state.
3. The frontend `useLogStream` hook handles the `done` event by closing the `EventSource` cleanly (no reconnect attempt).
4. After a workload completes, no stale `/logs` SSE connections remain open in the browser's network tab.
5. Running 5+ workloads sequentially does not accumulate leaked `EventSource` connections.
6. The Next.js streaming API route correctly pipes SSE data from the Go backend to the browser without buffering.
7. Historical log fallback (PBI-9) continues to work for terminal workloads.
8. All existing E2E tests continue to pass.

## Dependencies

- **Depends on**: PBI-2 (Done), PBI-3 (Done), PBI-9 (Done)
- **Blocks**: None
- **External**: None

## Open Questions

- None — root causes are confirmed and the approach is straightforward.

## Related Tasks

[View Tasks](./tasks.md)
