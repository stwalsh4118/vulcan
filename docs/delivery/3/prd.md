# PBI-3: Frontend Foundation & Workload Dashboard

[View in Backlog](../backlog.md)

## Overview

Build the Next.js web application with a workload submission form, workload list, workload detail view, and real-time log streaming. This UI must be ready before any backend lands so that each backend can be manually tested through the browser as it comes online.

## Problem Statement

The API exists (PBI-1) and will soon support execution (PBI-2), but there's no visual way to interact with Vulcan. The user needs a dashboard to submit workloads with different runtimes and isolation modes, monitor execution, view results, and stream logs — all from the browser. This is the primary test interface for validating backends as they're added.

## User Stories

- As a user, I want a web dashboard so that I can interact with Vulcan visually instead of only via curl/CLI.
- As a user, I want to submit workloads from a form so that I can quickly test different runtimes and isolation modes.
- As a user, I want to see a list of all workloads so that I can track what I've run.
- As a user, I want to see workload details (status, output, errors, duration) so that I can verify execution.
- As a user, I want to stream logs from the dashboard so that I can debug running workloads in real time.

## Technical Approach

- **Next.js 14+ (App Router)** with TypeScript, React Server Components where appropriate.
- **Styling**: Tailwind CSS with a dark-themed dashboard aesthetic (this is infra tooling, not a consumer app).
- **API client**: typed fetch wrapper or React Query / SWR for data fetching with polling for workload status updates.
- **Code editor**: Monaco Editor or CodeMirror for inline code input (JS/TS, Python, Go).
- **Log streaming**: EventSource API consuming the SSE `/v1/workloads/:id/logs` endpoint.
- **Pages**:
  - `/` — Dashboard home: recent workloads, quick stats.
  - `/workloads` — Paginated workload list with status filters.
  - `/workloads/new` — Workload submission form.
  - `/workloads/[id]` — Workload detail: status, output, logs, metadata.
- **Workload form fields**: runtime selector, isolation mode selector (auto/microvm/isolate/gvisor), code input (editor or file upload), input JSON, resource limits (CPU, memory, timeout).

## UX/UI Considerations

- Dark theme — this is a systems/infra tool, visual consistency with terminal aesthetics.
- Status indicators with color coding (pending=yellow, running=blue, completed=green, failed=red, killed=gray).
- Responsive but desktop-first — primary use is on a monitor.
- Code editor should support syntax highlighting for JS/TS, Python, Go.
- Log view should auto-scroll and support pause/resume.
- Form should clearly indicate which isolation modes are available (disabled states for backends not yet deployed).

## Acceptance Criteria

1. `pnpm dev` starts the Next.js development server and loads the dashboard.
2. Dashboard home page shows recent workloads and basic stats.
3. Workload submission form allows selecting runtime, isolation mode, entering code (with syntax highlighting), providing JSON input, and setting resource limits.
4. Submitting a workload calls the API and navigates to the workload detail page.
5. Workload list page shows all workloads with pagination and status filtering.
6. Workload detail page shows status, output, error, duration, isolation used, and metadata.
7. Workload detail page streams logs in real time via SSE when the workload is running.
8. The UI gracefully handles backends that aren't available yet (disabled options, clear messaging).

## Dependencies

- **Depends on**: PBI-1 (API endpoints to consume)
- **External**: `next`, `react`, `tailwindcss`, `@tailwindcss/typography`, code editor library (Monaco or CodeMirror), `swr` or `@tanstack/react-query`

## Open Questions

- Monaco Editor vs CodeMirror for the code input? Monaco is heavier but more VS Code-like; CodeMirror is lighter.
- Should the frontend be served from the Go binary (embedded) or run as a separate process? Separate process is simpler for development; embedded is simpler for deployment.

## Related Tasks

[View Tasks](./tasks.md)
