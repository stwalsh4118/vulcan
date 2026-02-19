# Tasks for PBI 3: Frontend Foundation & Workload Dashboard

This document lists all tasks associated with PBI 3.

**Parent PBI**: [PBI 3: Frontend Foundation & Workload Dashboard](./prd.md)

## Task Summary

| Task ID | Name | Status | Description |
| :------ | :--- | :----- | :---------- |
| 3-1 | [Next.js project scaffolding & Tailwind dark theme](./3-1.md) | Done | Initialize Next.js 14+ with App Router, TypeScript, Tailwind CSS dark theme; verify pnpm dev starts |
| 3-2 | [API client layer & TypeScript types](./3-2.md) | Done | Typed fetch wrapper for all backend endpoints; TypeScript types matching Go models; React Query for data fetching |
| 3-3 | [App shell layout & navigation](./3-3.md) | Done | Root layout with dark theme, sidebar navigation, status badge components, desktop-first responsive shell |
| 3-4 | [Dashboard home page](./3-4.md) | Done | Route `/` showing recent workloads and aggregate stats from the API |
| 3-5 | [Workload list page with pagination & filtering](./3-5.md) | Done | Route `/workloads` with paginated workload table, status filtering, and color-coded status badges |
| 3-6 | [Workload submission form with code editor](./3-6.md) | Done | Route `/workloads/new` with runtime/isolation selectors, code editor, JSON input, resource limits, submit to API |
| 3-7 | [Workload detail page](./3-7.md) | Done | Route `/workloads/[id]` showing status, output, error, duration, isolation, timestamps with auto-polling |
| 3-8 | [Real-time log streaming](./3-8.md) | Done | EventSource SSE integration on detail page with auto-scroll and pause/resume |
| 3-9 | [E2E CoS Test](./3-9.md) | Done | End-to-end tests verifying all 8 acceptance criteria for PBI 3 |

## Dependency Graph

```
  3-1 (Next.js scaffolding)
       │
       ├──────────────────┐
       │                  │
       v                  v
  3-2 (API client)   3-3 (App shell)
       │                  │
       ├──────┬───────┬───┤
       │      │       │   │
       v      v       v   v
  3-4 (Home) 3-5 (List) 3-6 (Form)  3-7 (Detail)
                                          │
                                          v
                                     3-8 (Log streaming)
                                          │
                                          v
                                     3-9 (E2E CoS Test)
```

## Implementation Order

1. **3-1** — Next.js project scaffolding & Tailwind dark theme (no dependencies; foundation for all frontend work)
2. **3-2** — API client layer & TypeScript types (depends on 3-1; provides data layer for all pages)
3. **3-3** — App shell layout & navigation (depends on 3-1; provides layout shell for all pages)
4. **3-4** — Dashboard home page (depends on 3-2, 3-3; first page consuming API data)
5. **3-5** — Workload list page (depends on 3-2, 3-3; parallel with 3-4, 3-6, 3-7)
6. **3-6** — Workload submission form (depends on 3-2, 3-3; requires code editor package research)
7. **3-7** — Workload detail page (depends on 3-2, 3-3; parallel with 3-4, 3-5, 3-6)
8. **3-8** — Real-time log streaming (depends on 3-7; integrates SSE into detail page)
9. **3-9** — E2E CoS Test (depends on all above; validates full acceptance criteria)

## Complexity Ratings

| Task ID | Complexity | External Packages |
|---------|-----------|-------------------|
| 3-1 | Simple | next, react, tailwindcss (via create-next-app) |
| 3-2 | Medium | @tanstack/react-query |
| 3-3 | Simple | None |
| 3-4 | Medium | None |
| 3-5 | Medium | None |
| 3-6 | Complex | Code editor (CodeMirror recommended) |
| 3-7 | Medium | None |
| 3-8 | Complex | None (uses browser EventSource API) |
| 3-9 | Complex | @playwright/test |

## External Package Research Required

| Task ID | Package | Reason |
|---------|---------|--------|
| 3-2 | @tanstack/react-query | React data fetching with polling, revalidation, caching |
| 3-6 | CodeMirror (or Monaco) | Inline code editor with syntax highlighting for JS/TS, Python, Go |
| 3-9 | @playwright/test | Browser-based E2E testing |
