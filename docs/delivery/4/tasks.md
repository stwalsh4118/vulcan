# Tasks for PBI 4: Firecracker microVM Backend

This document lists all tasks associated with PBI 4.

**Parent PBI**: [PBI 4: Firecracker microVM Backend](./prd.md)

## Task Summary

| Task ID | Name | Status | Description |
| :------ | :--- | :----- | :---------- |
| 4-1 | [Firecracker Configuration, Types & Protocol](./4-1.md) | Done | Define config structs, environment variables, vsock protocol types, and named constants for the Firecracker backend |
| 4-2 | [Guest Agent Binary](./4-2.md) | Done | Build the vulcan-guest static Go binary that runs inside microVMs — listens on vsock, receives code, executes, returns results |
| 4-3 | [Kernel & Rootfs Build Tooling](./4-3.md) | Done | Download scripts for Firecracker binary + kernel, Makefile targets for building Alpine-based ext4 rootfs images with Go/Node/Python runtimes |
| 4-4 | [vsock Host-Side Communication](./4-4.md) | Done | Host-side vsock client that connects to guest agent, sends workload specs, receives results, and streams log lines |
| 4-5 | [CNI Networking Setup](./4-5.md) | Done | CNI configuration with bridge + tc-redirect-tap for TAP device creation, IP allocation, and outbound NAT per microVM |
| 4-6 | [Firecracker Backend Core Implementation](./4-6.md) | Done | Implement the Backend interface using firecracker-go-sdk — VM lifecycle, registry integration, resource limits, concurrent VM management |
| 4-7 | [Firecracker Prometheus Metrics](./4-7.md) | Done | Register and instrument Firecracker-specific Prometheus metrics (boot time, active VMs, vsock latency, cleanup duration) |
| 4-8 | [Frontend microVM Enhancements](./4-8.md) | Done | Add archive upload support for microVM workloads, show isolation-specific metadata in workload detail view |
| 4-9 | [E2E CoS Test](./4-9.md) | Done | End-to-end tests verifying all PBI 4 acceptance criteria across Go, Node, and Python workloads in Firecracker microVMs |

## Dependency Graph

```
4-1 (Config, Types & Protocol)
 ├──► 4-2 (Guest Agent)
 │     └──► 4-3 (Kernel & Rootfs Tooling)
 │           └──► 4-6 (Backend Core) ◄── 4-4 + 4-5
 ├──► 4-4 (vsock Host-Side)
 │     └──► 4-6
 └──► 4-5 (CNI Networking)
       └──► 4-6
             ├──► 4-7 (Prometheus Metrics)
             └──► 4-8 (Frontend Enhancements)
                   └──► 4-9 (E2E CoS Test) ◄── 4-7
```

## Implementation Order

1. **4-1** — Foundational types, config, and protocol definitions (no dependencies)
2. **4-2** — Guest agent binary (depends on 4-1 for protocol types)
3. **4-3** — Kernel & rootfs tooling (depends on 4-2 for guest binary to bake into images)
4. **4-4** — vsock host-side communication (depends on 4-1 for protocol types; parallel with 4-2/4-3)
5. **4-5** — CNI networking setup (depends on 4-1 for config; parallel with 4-2/4-3/4-4)
6. **4-6** — Firecracker backend core (depends on 4-3, 4-4, 4-5 — the main integration point)
7. **4-7** — Prometheus metrics (depends on 4-6 for instrumentation points)
8. **4-8** — Frontend microVM enhancements (depends on 4-6 for a working backend)
9. **4-9** — E2E CoS test (depends on 4-7, 4-8 — validates everything together)

## Complexity Ratings

| Task ID | Complexity | External Packages |
|---------|------------|-------------------|
| 4-1 | Simple | None |
| 4-2 | Complex | `mdlayher/vsock` |
| 4-3 | Complex | None (shell tooling) |
| 4-4 | Medium | `mdlayher/vsock` |
| 4-5 | Complex | `containernetworking/cni` |
| 4-6 | Complex | `firecracker-go-sdk` |
| 4-7 | Simple | None |
| 4-8 | Medium | None |
| 4-9 | Complex | None |

## External Package Research Required

| Package | Used By | Guide Document |
|---------|---------|----------------|
| `github.com/mdlayher/vsock` | 4-2, 4-4 | `4-2-vsock-guide.md` |
| `github.com/containernetworking/cni` | 4-5 | `4-5-cni-guide.md` |
| `github.com/firecracker-microvm/firecracker-go-sdk` | 4-6 | `4-6-firecracker-sdk-guide.md` |
