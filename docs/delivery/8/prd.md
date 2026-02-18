# PBI-8: Monitoring & Benchmark Dashboard

[View in Backlog](../backlog.md)

## Overview

Extend the frontend with monitoring views and a benchmark visualization dashboard. Node status, backend capabilities, aggregate statistics, and a UI-driven benchmark runner that compares cold start times, throughput, and memory usage across all three backends with charts.

## Problem Statement

All three backends are operational (PBIs 4-6) and the frontend can submit workloads (PBI-3), but there's no way to visualize system health or compare backend performance at a glance. The PRD emphasizes benchmarking as a core value — the user wants to see real numbers from their hardware, side by side, in a visual format.

## User Stories

- As an operator, I want to see all nodes, their capacity, and running workloads in one place.
- As a learner, I want to run the same workload across all three backends and compare cold start, memory overhead, and throughput side by side.
- As an operator, I want to see aggregate stats (workload counts, avg latency per backend) on a dashboard.
- As a user, I want to kick off a benchmark suite from the UI and see the results as charts.

## Technical Approach

- **Node status page** (`/nodes`): table of nodes with hostname, address, capacity (CPU/mem total vs used), available backends, status. Consumes `GET /v1/nodes`.
- **Backend capabilities page** or section: what each backend supports, current status (online/offline), runtime support matrix.
- **Stats dashboard**: aggregate numbers from `GET /v1/stats` — total workloads, success/failure rate, avg duration per backend, workloads over time.
- **Benchmark runner** (`/benchmarks`): UI to trigger `GET /v1/stats/benchmark`. Displays results as:
  - Comparison table (backend × metric).
  - Bar charts for cold start time, warm start time, throughput.
  - Memory overhead comparison.
- **Charting library**: Recharts, Chart.js, or similar React-compatible library.
- **Auto-refresh**: stats and node status poll on an interval (configurable, default 5s).

## UX/UI Considerations

- Benchmark results should be visually compelling — this is the showcase feature for comparing isolation technologies.
- Charts should be responsive and render well on large monitors.
- Node status should use clear visual indicators (green/yellow/red) for health.
- Benchmark runner should show progress while running (workloads submitted, completed, remaining).
- Results should be downloadable as JSON for external analysis.

## Acceptance Criteria

1. Node status page displays all nodes with capacity, backends, and health status.
2. Stats dashboard shows aggregate workload counts, success rate, and average duration per backend.
3. Benchmark runner can be triggered from the UI and shows progress during execution.
4. Benchmark results display as a comparison table (backend × metric: cold start, throughput, memory).
5. Bar/line charts visualize benchmark results for at-a-glance comparison.
6. Stats auto-refresh on a configurable interval.
7. Benchmark results can be exported as JSON.
8. All views handle loading, empty, and error states gracefully.

## Dependencies

- **Depends on**: PBI-3 (Frontend foundation), PBI-4 (Firecracker), PBI-5 (V8 Isolates), PBI-6 (gVisor)
- **External**: Charting library (Recharts, Chart.js, or similar)

## Open Questions

- Should benchmark history be persisted (run benchmarks over time, compare across dates)?
- Should there be Grafana dashboard templates in the repo that consume the Prometheus metrics, in addition to the built-in dashboard?

## Related Tasks

_Tasks will be created when this PBI moves to Agreed via `/plan-pbi 8`._
