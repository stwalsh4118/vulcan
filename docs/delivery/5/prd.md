# PBI-5: V8 Isolate Backend

[View in Backlog](../backlog.md)

## Overview

Implement the V8 isolate backend using a long-running Deno subprocess. JS/TS workloads execute as Web Workers inside Deno, achieving sub-millisecond cold starts with V8-level isolation. After this PBI, users can submit JavaScript or TypeScript code from the frontend and see it execute almost instantly.

## Problem Statement

Firecracker provides strong isolation but ~125ms cold starts. For lightweight JS/TS functions, that overhead is unnecessary. V8 isolates offer process-level isolation with near-zero cold starts — the same model Cloudflare Workers uses. This gives Vulcan its second isolation tier and enables the first real backend comparison.

## User Stories

- As a user, I want to run JS/TS code in V8 isolates so that I get near-instant execution for lightweight functions.
- As a user, I want to submit inline JavaScript from the frontend and see the result in milliseconds.
- As a learner, I want to compare isolate execution times against Firecracker microVM times for the same workload.

## Technical Approach

- **vulcan-isolate-host**: a Deno TypeScript program that runs as a long-lived subprocess managed by the Go node agent. Exposes an HTTP server on localhost.
- **Endpoint**: `POST /execute` accepts `{ code, input, timeout_ms }`, creates a Deno Web Worker, executes the code in an isolated V8 heap, returns `{ output, duration_ms }`.
- **Worker isolation**: each workload is a separate Web Worker with its own V8 isolate. No shared state, no filesystem access, no network access (`--deny-all`).
- **Process management**: Go backend starts Deno subprocess on init, monitors health (periodic ping), restarts on crash. Graceful shutdown on backend stop.
- **Timeout enforcement**: Deno host kills the Worker if it exceeds `timeout_ms`.
- **Memory limits**: per-Worker memory cap via Deno flags.
- **Frontend updates**: ensure the code editor provides JS/TS syntax highlighting; show `isolate` as available in the isolation selector.

## UX/UI Considerations

- Frontend code editor should default to JavaScript syntax highlighting when `isolate` isolation is selected.
- Workload detail view should show isolate-specific metadata (worker creation time, heap size).
- The near-instant execution should feel responsive in the UI — no unnecessary loading spinners for sub-10ms workloads.

## Acceptance Criteria

1. Isolate backend implements the `Backend` interface from PBI-2.
2. Deno host process starts automatically when the isolate backend initializes.
3. JS/TS code submitted as a string executes in an isolated Web Worker and returns results.
4. Each workload runs in a separate V8 isolate with no access to filesystem or network.
5. Workloads exceeding the timeout are killed and marked as failed.
6. Memory limits are enforced per isolate.
7. Deno host process recovers automatically from crashes.
8. A user can submit a JS/TS workload from the frontend, see it execute in milliseconds, and view the result.
9. Prometheus metrics include isolate-specific stats (execution time, active workers, host process restarts).
10. `auto` mode correctly routes JS/TS workloads to the isolate backend.

## Dependencies

- **Depends on**: PBI-2 (Backend interface), PBI-3 (Frontend for manual testing)
- **External**: Deno runtime (installed on host)

## Open Questions

- Should Wasm workloads also route to the isolate backend, or is that deferred?
- What's the maximum number of concurrent Workers the Deno host should support before rejecting requests?
- Should the Deno host process be periodically recycled to prevent memory leaks, or only on crash?

## Related Tasks

_Tasks will be created when this PBI moves to Agreed via `/plan-pbi 5`._
