# Product Backlog

**PRD**: [View PRD](../prd.md)

## Backlog Items

| ID | Actor | User Story | Status | Conditions of Satisfaction (CoS) |
|----|-------|-----------|--------|----------------------------------|
| 1 | Developer | As a developer, I want a foundational Go project with HTTP API and persistence so that all subsequent features have a stable base to build on | Done | Go module builds; chi server starts and serves health/metrics; SQLite stores and retrieves workloads; all CRUD endpoints return correct responses |
| 2 | Developer | As a developer, I want a common backend interface and workload lifecycle so that isolation backends are interchangeable and workloads progress through well-defined states | Done | Backend Go interface defined; workloads transition through pending→running→completed/failed/killed; auto-routing resolves isolation mode; async execution returns immediately with pollable status; log streaming works via SSE |
| 3 | User | As a user, I want a web dashboard to submit workloads and monitor their execution so that I can test each backend visually as it comes online | Done | Next.js app runs and connects to API; workload submission form supports all input types; workload list is paginated and filterable; detail view shows status/output/metadata; log streaming displays in real time |
| 4 | User | As a user, I want to run workloads in Firecracker microVMs so that I get hardware-level isolation for arbitrary runtimes and can test this from the frontend | Proposed | Firecracker microVMs boot and execute workloads; CNI networking provides connectivity; vsock transfers code and results; guest agent runs inside VM; rootfs images for Go/Node/Python work; frontend can submit and monitor microVM workloads |
| 5 | User | As a user, I want to run JS/TS workloads in V8 isolates so that I get sub-millisecond cold starts for lightweight functions and can test this from the frontend | Proposed | Deno host process runs and accepts workloads; Web Workers execute in isolated V8 heaps; timeouts and memory limits enforced; frontend can submit inline JS/TS code and see results |
| 6 | User | As a user, I want to run OCI container workloads in gVisor so that I get syscall-level isolation for any containerized application and can test this from the frontend | Proposed | containerd pulls and caches OCI images; runsc executes containers with gVisor sandboxing; resource limits enforced; frontend supports OCI image reference input and shows results |
| 7 | User | As a user, I want a CLI tool to submit and manage workloads from the terminal so that I can script and automate compute tasks | Proposed | `vulcan run` executes workloads (inline, file, OCI); `vulcan status` shows workload state; `vulcan logs -f` streams logs; `vulcan ls` lists workloads; `vulcan nodes` shows cluster; `vulcan benchmark` runs comparison suite |
| 8 | User | As a user, I want a monitoring and benchmark dashboard so that I can compare backend performance and view system health visually | Proposed | Dashboard shows node status and capacity; backend capabilities displayed; aggregate stats visualized; benchmark suite runnable from UI; comparison charts show cold start, throughput, memory per backend |
| 9 | User | As a user, I want to view workload logs after execution completes so that I can debug and review past workloads | Done | Log lines persisted to SQLite during execution; API endpoint returns historical logs; frontend shows persisted logs for completed workloads; SSE still works for active workloads; seamless transition from live to historical |

_Items are ordered by priority (highest first)._

## PBI Details

| ID | Title | Detail Document |
|----|-------|----------------|
| 1 | Project Foundation & Core API | [View Details](./1/prd.md) |
| 2 | Backend Interface & Workload Lifecycle | [View Details](./2/prd.md) |
| 3 | Frontend Foundation & Workload Dashboard | [View Details](./3/prd.md) |
| 4 | Firecracker microVM Backend | [View Details](./4/prd.md) |
| 5 | V8 Isolate Backend | [View Details](./5/prd.md) |
| 6 | gVisor Backend | [View Details](./6/prd.md) |
| 7 | CLI Tool | [View Details](./7/prd.md) |
| 8 | Monitoring & Benchmark Dashboard | [View Details](./8/prd.md) |
| 9 | Log Persistence & Historical Log Viewing | [View Details](./9/prd.md) |

## History

| Timestamp | PBI_ID | Event_Type | Details | User |
|-----------|--------|------------|---------|------|
| 20260218-110701 | ALL | Created | Initial backlog created from PRD decomposition | AI_Agent |
| 20260218-111242 | 1 | Status Change | Proposed → Agreed. Auto-approved for planning. | AI_Agent |
| 2026-02-18 11:19:56 | 1 | Status Change | Agreed → InProgress. Started implementation. | AI_Agent |
| 2026-02-18 11:58:00 | 1 | Status Change | InProgress → Done. All 8 tasks completed and verified. | AI_Agent |
| 2026-02-18 13:14:02 | 2 | Status Change | Proposed → Agreed. Auto-approved for planning. | AI_Agent |
| 2026-02-18 13:35:12 | 2 | Status Change | Agreed → InProgress. Started implementation. | AI_Agent |
| 2026-02-19 04:49:27 | 2 | Status Change | InProgress → Done. All 8 tasks completed and verified. | AI_Agent |
| 2026-02-19 05:13:20 | 3 | Status Change | Proposed → Agreed. Auto-approved for planning. | AI_Agent |
| 2026-02-19 05:23:12 | 3 | Status Change | Agreed → InProgress. Started implementation. | AI_Agent |
| 2026-02-19 06:18:27 | 3 | Status Change | InProgress → Done. All 9 tasks completed and verified. | AI_Agent |
| 20260219-061510 | 9 | Created | PBI created from feature request: Log persistence — store workload log lines in SQLite so they can be viewed after completion. | AI_Agent |
| 2026-02-19 07:13:19 | 9 | Status Change | Proposed → Agreed. Auto-approved for planning. | AI_Agent |
| 2026-02-19 07:20:40 | 9 | Status Change | Agreed → InProgress. Started implementation. | AI_Agent |
| 2026-02-19 07:57:42 | 9 | Status Change | InProgress → Done. All 6 tasks completed and verified. | AI_Agent |
