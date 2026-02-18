# Vulcan — Unified Compute Platform with Pluggable Isolation

## Overview
A compute platform that abstracts over multiple isolation backends — Firecracker microVMs, V8 isolates, gVisor containers — behind a single API. Submit a workload, specify (or let Vulcan choose) the isolation level, and get a result back. Built in Go, designed to run inside VMs on commodity hardware, and scale across multiple nodes. Equal parts learning project and usable tool.

## Problem Statement
Cloud providers have made serverless compute a black box. You `deploy` and it runs — but the isolation, scheduling, networking, and lifecycle management underneath are opaque. There's no way to deeply understand how Lambda, Workers, or Cloud Run work without building the layers yourself. At the same time, existing open-source FaaS platforms (OpenFaaS, Knative, OpenWhisk) are complex Kubernetes-native systems that abstract *over* the interesting parts rather than exposing them.

The second problem is practical: homelab servers with 36 threads and 128GB of RAM sit idle. There's no lightweight, self-hosted compute platform that lets you throw arbitrary workloads at your own hardware with proper isolation — something between "SSH in and run it" and "set up a full Kubernetes cluster."

## Target Users
1. **Primary: Sean** — solo dev who wants to understand cloud infrastructure deeply by building it, and who has beefy homelab hardware to run it on.
2. **Secondary: Homelab enthusiasts** — people with spare hardware who want a self-hosted compute platform without the complexity of Kubernetes.
3. **Tertiary: Infrastructure learners** — developers studying for cloud/systems roles who want a hands-on playground for isolation technologies.

## Core Value Proposition
The only platform that lets you compare and use Firecracker microVMs, V8 isolates, and gVisor containers through a single API on your own hardware. It's a tool you use *and* a system you learn from — every layer is yours to inspect, benchmark, and modify.

## User Stories / Use Cases

### Core Workflows
- As a user, I want to submit a function (code + runtime) via HTTP and get the result back, so I can run arbitrary compute without SSH-ing into a server.
- As a user, I want to specify the isolation level (`microvm`, `isolate`, `gvisor`, or `auto`) so I can trade off between cold start speed and isolation strength.
- As a user, I want to run async workloads and poll for results, so I can kick off long-running tasks without blocking.
- As a user, I want to stream logs from a running workload, so I can debug issues in real time.

### Operational
- As an operator, I want to see all nodes, their capacity, and running workloads in one place.
- As an operator, I want Prometheus metrics for every backend so I can build dashboards comparing cold start times, throughput, and resource usage.
- As an operator, I want to add a new node by pointing it at the cluster, with no manual configuration.

### Learning / Benchmarking
- As a learner, I want to run the same workload across all three backends and compare cold start, memory overhead, and throughput side by side.
- As a learner, I want to inspect how each backend sets up isolation (TAP devices, V8 heaps, syscall interception) through logs and metrics.

## Features & Scope

### MVP (Phase 1-3)
- HTTP API for synchronous and async workload execution
- Firecracker microVM backend (Go, Node, Python runtimes via pre-built rootfs images)
- V8 isolate backend via Deno (JS/TS/Wasm)
- gVisor backend via runsc (any language via OCI images)
- Backend interface abstraction — all three implement the same Go interface
- `auto` isolation mode: JS/TS → isolate, OCI image → gVisor, everything else → microVM
- Single-node deployment inside a Proxmox VM
- Per-workload resource limits (CPU, memory, timeout)
- Prometheus /metrics endpoint with per-backend stats
- CLI tool: `vulcan run`, `vulcan logs`, `vulcan status`
- Built-in benchmark suite comparing all backends

### Future (Phase 4-5)
- Multi-node: NATS-based placement, node agents, distributed scheduler
- Warm pools with configurable size per backend/runtime
- Firecracker snapshot/restore for faster warm starts
- Function registry (named functions that persist across invocations)
- Request routing (HTTP proxy that dispatches to warm workloads by path)
- Web dashboard for node/workload management
- Workload-to-workload networking (private mesh between microVMs)
- OCI image builder (Dockerfile → rootfs for Firecracker)

---

## System Design

### Architecture Overview

```
                    ┌─────────────┐
                    │  Vulcan API  │  ← HTTP API + CLI target
                    │   (Go)      │
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    │  Scheduler   │  ← Local-only (MVP), NATS-based (multi-node)
                    │              │     Picks backend + node for workload
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    │  Node Agent  │  ← Manages local backends, reports capacity
                    │              │     Source of truth for its own workloads
                    └──────┬──────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
    ┌─────┴─────┐   ┌─────┴─────┐   ┌─────┴─────┐
    │ Firecracker│   │  Isolate  │   │   gVisor   │
    │  Backend   │   │  Backend  │   │  Backend   │
    │            │   │           │   │            │
    │ KVM microVM│   │ Deno V8   │   │ runsc OCI  │
    │ TAP + NAT  │   │ subprocess│   │ containerd │
    └───────────┘   └───────────┘   └────────────┘
```

In MVP, the API server, scheduler, and node agent are a single binary. The scheduler is just "run it locally on whichever backend matches." Multi-node splits the API server from node agents, with NATS for communication.

### Key Technical Decisions

**Single binary for MVP**
API server, scheduler, and node agent compile into one Go binary. No microservices overhead for a single-node deployment. Multi-node adds a separate `vulcan-agent` binary that node agents run.

**Firecracker networking: CNI with tc-redirect-tap**
Rather than managing TAP devices and nftables rules manually per microVM, use CNI plugins. The firecracker-go-sdk has native CNI support. `tc-redirect-tap` adapts standard CNI plugins (bridge, ptp) for Firecracker's TAP requirement. This is the same approach Kata Containers uses.

**Code injection via vsock**
Firecracker supports virtio-vsock — a socket interface between host and guest that doesn't require network setup. The node agent sends function code to the guest agent over vsock, the guest agent executes it, and returns the result over the same channel. Faster and simpler than MMDS or drive mounting.

**Deno as isolate runtime**
A long-running Deno process managed by the node agent. Workloads are submitted as Deno Workers (Web Workers API). Communication via HTTP — the Deno process runs a small server that accepts code + input and returns output. This avoids subprocess-per-request overhead while maintaining isolate-level isolation.

**Function packaging**
Three formats, matching the three backends:
- **Raw code** (string) — for isolates. JS/TS submitted directly.
- **Archive** (tar.gz) — for microVMs. Contains entrypoint + dependencies, extracted into rootfs.
- **OCI image reference** — for gVisor. Standard container images.

The `auto` mode infers from the format: raw code → isolate, OCI ref → gVisor, archive → microVM.

**Local state: SQLite**
Each node agent stores workload state in SQLite. Simple, reliable, queryable. Fly.io uses BoltDB but SQLite is more flexible for debugging and ad-hoc queries. No distributed database — each node owns its state.

### Data Flow: Synchronous Workload

```
Client → POST /v1/workloads { code, runtime: "node", isolation: "auto" }
  → API Server receives request
  → Scheduler resolves "auto" → "isolate" (because runtime is node/JS)
  → Scheduler picks node (local in MVP)
  → Node Agent dispatches to Isolate Backend
  → Isolate Backend sends code to Deno worker process
  → Deno executes in V8 isolate, returns result
  → Node Agent records execution metadata in SQLite
  → API Server returns { output, duration_ms: 3, isolation_used: "isolate", node: "newt" }
```

### Data Flow: Firecracker Workload

```
Client → POST /v1/workloads { code: <tar.gz>, runtime: "python", isolation: "microvm" }
  → Scheduler → Firecracker Backend
  → Backend checks warm pool for python microVM
    → If warm: grab VM from pool
    → If cold: create TAP (via CNI), boot microVM (kernel + python rootfs, ~125ms)
  → Send code to guest agent via vsock
  → Guest agent extracts archive, runs entrypoint
  → Guest agent returns stdout/stderr via vsock
  → Backend returns result (or destroys/recycles VM)
  → Node Agent records metadata
  → API returns result
```

### Infrastructure

**Deployment:**
- Single Proxmox VM per node (Debian 12, nested KVM enabled)
- `vulcan` binary runs as systemd service
- Base rootfs images (Alpine + Go/Node/Python) stored locally
- Firecracker binary + kernel image bundled or fetched on first run

**Monitoring:**
- Prometheus metrics at `/metrics` — per-backend cold starts, execution times, memory usage, pool sizes
- Structured JSON logging to stdout (journald captures it)
- Future: Grafana dashboard template in the repo

**CI/CD:**
- GitHub Actions: build, test, lint
- Release: single binary per platform via GoReleaser
- Rootfs images: built in CI, published as GitHub release assets

---

## Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| Language | Go 1.23+ | Firecracker Go SDK, infra ecosystem is Go, strong concurrency model |
| HTTP framework | net/http + chi | Lightweight, stdlib-compatible, good middleware ecosystem |
| Firecracker | firecracker-go-sdk | Official SDK, builder patterns, CNI support built in |
| V8 Isolates | Deno (long-running subprocess) | Avoids embedding V8, excellent isolate model, TS-native |
| gVisor | runsc via containerd Go client | Standard OCI integration, containerd is the industry default |
| Messaging | NATS (phase 4+) | Lightweight, Go-native, proven at Fly.io scale |
| Local state | SQLite via modernc.org/sqlite | Pure Go SQLite, no CGO, single-file database |
| Metrics | Prometheus client_golang | Standard, Grafana-compatible |
| CLI | cobra | Standard Go CLI framework |
| Networking | CNI + tc-redirect-tap | Managed TAP creation, standard plugin ecosystem |
| Container images | containerd Go client | Pull/manage OCI images programmatically |

---

## Data Model

### Workloads

```sql
CREATE TABLE workloads (
    id          TEXT PRIMARY KEY,    -- ulid
    status      TEXT NOT NULL,       -- pending, running, completed, failed, killed
    isolation   TEXT NOT NULL,       -- microvm, isolate, gvisor
    runtime     TEXT NOT NULL,       -- go, node, python, wasm, oci
    node_id     TEXT NOT NULL,       -- which node executed this
    input_hash  TEXT,                -- hash of input for dedup/caching
    output      BLOB,               -- result bytes (null until complete)
    exit_code   INTEGER,
    error       TEXT,                -- error message if failed
    cpu_limit   INTEGER,            -- vcpus allocated
    mem_limit   INTEGER,            -- MB allocated
    timeout_s   INTEGER,            -- max execution time
    duration_ms INTEGER,            -- actual execution time
    created_at  DATETIME NOT NULL,
    started_at  DATETIME,
    finished_at DATETIME
);
```

### Nodes (multi-node, phase 4)

```sql
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,
    hostname    TEXT NOT NULL,
    address     TEXT NOT NULL,       -- ip:port
    cpus_total  INTEGER NOT NULL,
    cpus_used   INTEGER NOT NULL DEFAULT 0,
    mem_total   INTEGER NOT NULL,    -- MB
    mem_used    INTEGER NOT NULL DEFAULT 0,
    backends    TEXT NOT NULL,       -- json array: ["microvm", "isolate", "gvisor"]
    last_seen   DATETIME NOT NULL,
    status      TEXT NOT NULL        -- online, offline, draining
);
```

### Warm Pool (phase 5)

```sql
CREATE TABLE warm_pool (
    id          TEXT PRIMARY KEY,
    backend     TEXT NOT NULL,       -- microvm, gvisor
    runtime     TEXT NOT NULL,       -- go, node, python
    vm_id       TEXT,                -- firecracker VM ID or container ID
    status      TEXT NOT NULL,       -- ready, in_use, recycling
    created_at  DATETIME NOT NULL,
    last_used   DATETIME
);
```

---

## API Design

### Endpoints

```
POST   /v1/workloads              Create and execute (sync — blocks until complete)
POST   /v1/workloads/async        Create and execute (async — returns ID immediately)
GET    /v1/workloads/:id          Get workload status and result
GET    /v1/workloads/:id/logs     Stream workload logs (SSE)
DELETE /v1/workloads/:id          Kill a running workload
GET    /v1/workloads              List recent workloads (paginated)

GET    /v1/nodes                  List cluster nodes and capacity
GET    /v1/nodes/:id              Node detail with running workloads

GET    /v1/backends               List available backends and capabilities
GET    /v1/stats                  Aggregate stats (counts, avg latency per backend)
GET    /v1/stats/benchmark        Run built-in benchmark suite, return comparison

GET    /metrics                   Prometheus metrics
GET    /healthz                   Health check
```

### Request/Response Examples

**Sync execution:**
```json
// POST /v1/workloads
{
  "runtime": "node",
  "isolation": "auto",
  "code": "export default (input) => ({ sum: input.a + input.b });",
  "input": { "a": 1, "b": 2 },
  "resources": { "cpus": 1, "mem_mb": 128, "timeout_s": 30 }
}

// Response 200
{
  "id": "01JKQX7V8MZRN3B4HTGFWDCS9E",
  "status": "completed",
  "isolation_used": "isolate",
  "node": "newt",
  "output": { "sum": 3 },
  "duration_ms": 3
}
```

**Async execution:**
```json
// POST /v1/workloads/async
{
  "runtime": "python",
  "isolation": "microvm",
  "code_archive": "<base64 tar.gz>",
  "input": { "dataset": "large" },
  "resources": { "cpus": 4, "mem_mb": 2048, "timeout_s": 300 }
}

// Response 202
{
  "id": "01JKQX9A2KPTN5C6JMREVBYF4W",
  "status": "running"
}
```

### CLI

```bash
# Run inline code
vulcan run --runtime node --code 'export default () => "hello"'

# Run a file
vulcan run --runtime python --file ./my_script.py --input '{"n": 42}'

# Run with specific isolation
vulcan run --runtime go --file ./main.go --isolation microvm

# Run OCI image
vulcan run --image alpine:latest --cmd "echo hello"

# Check status
vulcan status 01JKQX9A2KPTN5C6JMREVBYF4W

# Stream logs
vulcan logs -f 01JKQX9A2KPTN5C6JMREVBYF4W

# List recent workloads
vulcan ls

# Cluster overview
vulcan nodes

# Benchmark all backends
vulcan benchmark
```

---

## Backend Details

### Firecracker (Phase 1)

**MicroVM lifecycle:**
1. Check warm pool for matching runtime
2. If cold: allocate TAP device via CNI (`tc-redirect-tap`), boot microVM via Go SDK
3. Guest agent (small static Go binary baked into rootfs) listens on vsock
4. Host sends workload spec + code over vsock
5. Guest agent extracts code, executes entrypoint, captures stdout/stderr
6. Guest agent sends result back over vsock
7. Host records result, returns VM to warm pool or destroys

**Base rootfs images (pre-built):**
- `vulcan-base-go.ext4` — Alpine + Go toolchain (~150MB)
- `vulcan-base-node.ext4` — Alpine + Node.js (~100MB)
- `vulcan-base-python.ext4` — Alpine + Python 3 (~80MB)
- `vulcan-base-minimal.ext4` — Alpine only (~20MB)

**Guest agent (`vulcan-guest`):**
A ~5MB static Go binary that:
- Starts as PID 1 (or spawned by a minimal init)
- Listens on vsock for workload specs
- Extracts code archive to /work
- Executes entrypoint with resource limits (via cgroups inside guest)
- Streams stdout/stderr back over vsock
- Reports exit code and execution time

**Networking:**
- CNI bridge plugin creates bridge + TAP per VM
- `tc-redirect-tap` redirects traffic from veth to TAP
- Outbound NAT via nftables masquerade on the bridge
- Guest gets IP via kernel command line args (no DHCP needed)

**Security:**
- Firecracker jailer: chroot + namespace + seccomp + cgroup isolation per VM
- Each VM runs under a dedicated unprivileged UID
- Guest has no access to host filesystem

### V8 Isolates (Phase 2)

**Architecture:**
- Long-running Deno process (`vulcan-isolate-host`) managed by node agent
- HTTP server inside Deno accepts workload requests
- Each workload runs as a Deno Web Worker (separate V8 isolate)
- Worker has: own heap, own event loop, no filesystem access, no network access
- Communication: HTTP request in → HTTP response out

**`vulcan-isolate-host` (Deno/TypeScript):**
```
POST /execute
  { code: string, input: any, timeout_ms: number }
  → Creates Worker, passes code + input
  → Worker executes, returns result
  → Host returns { output, duration_ms }
```

**Security:**
- Deno's `--deny-all` permissions: no filesystem, no network, no env
- Each Worker is a separate V8 isolate with isolated heap
- Timeout enforced by the host — kills Worker if exceeded
- Memory limit per Worker (Deno `--max-old-space-size` equivalent)

**Limitations (by design):**
- JS/TS/Wasm only
- No filesystem, no raw TCP/UDP
- 128MB memory per isolate
- Single-threaded per isolate

### gVisor (Phase 3)

**Architecture:**
- Uses containerd Go client to pull and manage OCI images
- Runs containers with `runsc` (gVisor) as the OCI runtime
- Each workload = one container with gVisor sandboxing

**Lifecycle:**
1. Pull OCI image (cached locally after first pull)
2. Create container via containerd with runsc runtime
3. Set resource limits via OCI spec (CPU, memory, timeout)
4. Execute, capture stdout/stderr
5. Destroy container

**Security:**
- gVisor intercepts all syscalls in user-space (Sentry kernel)
- Container has no direct kernel access
- Resource limits via OCI spec + cgroup enforcement

---

## Milestones

| Phase | Scope | Estimated Effort |
|---|---|---|
| **Phase 1: Firecracker Core** | Single binary. HTTP API (sync + async). Firecracker backend with CNI networking, vsock communication, guest agent. Pre-built rootfs images for Go/Node/Python. CLI (`run`, `status`, `logs`, `ls`). SQLite state. Prometheus metrics. Runs in Proxmox VM with nested KVM. | 2-3 weeks |
| **Phase 2: V8 Isolates** | Isolate backend via Deno host process. `auto` mode routing (JS/TS → isolate). Same API surface, caller picks isolation. Side-by-side benchmarks: Firecracker vs isolates. | 1-2 weeks |
| **Phase 3: gVisor + Benchmarks** | gVisor backend via containerd + runsc. OCI image support. All three backends behind the same interface. `vulcan benchmark` command. Comprehensive benchmark suite. Blog post #1: "Building a Serverless Platform From Scratch." | 1-2 weeks |
| **Phase 4: Multi-Node** | Separate `vulcan-agent` binary. NATS-based placement (broadcast → offer → accept, Fly.io style). Node registration and heartbeat. Scheduler with capacity-aware placement. Run across newt + gecko. | 2-3 weeks |
| **Phase 5: Warm Pools + Polish** | Warm pool management per backend/runtime. Firecracker snapshot/restore. Function registry (named persistent functions). HTTP request routing to warm workloads. Web dashboard. Blog post #2: "Scaling a DIY Compute Platform Across Nodes." | 2-3 weeks |

---

## Benchmarks to Capture

| Metric | Firecracker | V8 Isolate | gVisor |
|---|---|---|---|
| Cold start time | ~125ms expected | ~0-5ms expected | ~50-100ms expected |
| Warm start time | ? (snapshot/restore) | ~0ms (reuse isolate) | ? (cached image) |
| Memory overhead per workload | ~5MB + guest allocation | KBs | Container overhead |
| Max concurrent (on 64GB node) | ? | ? (thousands expected) | ? |
| Throughput (hello-world req/s) | ? | ? | ? |
| Throughput (CPU-bound 1s task) | ? | ? | ? |
| Isolation strength | Hardware (KVM) | Process (V8 sandbox) | Syscall (user-space kernel) |
| Supported runtimes | Any (via rootfs) | JS/TS/Wasm | Any (via OCI image) |
| Networking | Full (TAP/NAT) | None | Full (gVisor netstack) |
| Filesystem | Full (ext4 rootfs) | None | Full (OCI layers) |

Run all benchmarks on same Xeon E5-2660 v3 (10c/20t, 64GB). Publish real numbers from commodity hardware, not cloud instances.

---

## Risks & Open Questions

### Technical Risks
- **Firecracker networking under load** — Managing hundreds of TAP devices + NAT rules could degrade. Mitigation: test at scale early in phase 1, benchmark with 50+ concurrent VMs.
- **Nested KVM stability** — 10-20% perf hit is fine, but stability under sustained load with many concurrent microVMs needs validation. Mitigation: stress test in phase 1, fall back to bare metal on Proxmox host if nested is flaky.
- **Deno Worker lifecycle** — Web Workers in Deno may have edge cases around memory leaks or stale state. Mitigation: periodically recycle the host process, monitor memory.
- **vsock reliability** — Less battle-tested than HTTP for host-guest communication. Mitigation: fall back to serial console + HTTP if vsock has issues.

### Design Decisions Made
- **`auto` mode heuristic:** JS/TS → isolate, OCI image reference → gVisor, code archive → microVM. Simple, deterministic, no magic.
- **Local state per node (SQLite), no distributed DB.** Fly.io's model works. Each node is authoritative for its workloads. Multi-node queries fan out to each node agent.
- **Synchronous scheduling.** No pending queue — either a node can run it now or the request fails. Simple, predictable. Async endpoint is for long-running workloads, not queuing.
- **CNI for networking, not manual TAP management.** More setup upfront but dramatically simpler per-VM networking.

### Remaining Open Questions
- **Rootfs image updates** — How to rebuild and distribute updated rootfs images across nodes? CI-built and pulled via HTTP?
- **Multi-node workload migration** — If a node goes down, do we restart workloads on another node? Probably not for MVP — stateless workloads just fail and the client retries.
- **Auth** — API is unauthenticated in MVP (homelab, private network). Token auth for multi-node?

---

## References & Inspiration
- [Fly.io architecture](https://fly.io/blog/) — flyd, flaps, Corrosion, synchronous scheduling, NATS placement
- [Cloudflare Workers architecture](https://blog.cloudflare.com/cloud-computing-without-containers/) — V8 isolate model, cold start elimination
- [Cloudflare Workers security model](https://blog.cloudflare.com/mitigating-spectre-and-other-security-threats-the-cloudflare-workers-security-model/) — MPK, V8 sandbox, Spectre mitigations
- [workerd](https://github.com/cloudflare/workerd) — Open-source Workers runtime
- [firecracker-go-sdk](https://github.com/firecracker-microvm/firecracker-go-sdk) — Go bindings
- [Firecracker docs](https://github.com/firecracker-microvm/firecracker/tree/main/docs) — networking, rootfs, jailer, vsock
- [gVisor](https://gvisor.dev/) — User-space kernel, runsc OCI runtime
- [Kata Containers](https://katacontainers.io/) — OCI-compatible microVMs, Firecracker support
- [Kata vs Firecracker vs gVisor](https://northflank.com/blog/kata-containers-vs-firecracker-vs-gvisor) — Comparison
- [Weave Ignite](https://github.com/weaveworks/ignite) — Docker UX for Firecracker
- [tc-redirect-tap](https://github.com/awslabs/tc-redirect-tap) — CNI plugin for Firecracker TAP devices
