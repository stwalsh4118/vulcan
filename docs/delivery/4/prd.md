# PBI-4: Firecracker microVM Backend

[View in Backlog](../backlog.md)

## Overview

Implement the Firecracker microVM backend — the most isolation-heavy option in Vulcan. This includes the host-side integration (firecracker-go-sdk, CNI networking, vsock), the guest agent that runs inside microVMs, and pre-built rootfs images for Go, Node, and Python. After this PBI, users can submit workloads via the frontend and have them execute inside real KVM-backed microVMs.

## Problem Statement

The backend interface exists (PBI-2) and the frontend is ready (PBI-3), but no actual compute backend is available. Firecracker is the most complex backend but also the most compelling — hardware-level isolation via KVM with ~125ms cold starts. This is the backend that makes Vulcan more than a container orchestrator.

## User Stories

- As a user, I want to run workloads in Firecracker microVMs so that I get hardware-level KVM isolation.
- As a user, I want to submit Go, Node, or Python code and have it execute inside a microVM with the appropriate runtime pre-installed.
- As a user, I want to test microVM workloads from the frontend dashboard so that I can verify the backend works end-to-end.
- As a learner, I want to see how Firecracker networking (TAP devices, CNI) and host-guest communication (vsock) work in practice.

## Technical Approach

- **firecracker-go-sdk**: VM configuration (kernel, rootfs, vcpus, memory), machine lifecycle (create, start, wait, stop).
- **CNI networking**: bridge plugin + `tc-redirect-tap` for TAP device creation. Each VM gets a TAP device and an IP. Outbound NAT via nftables masquerade.
- **vsock communication**: host opens vsock connection to guest agent. Protocol: send workload spec (JSON) + code payload → receive result (JSON) + stdout/stderr.
- **Guest agent (`vulcan-guest`)**: small static Go binary baked into rootfs. Starts as PID 1 or early init. Listens on vsock, receives code, extracts to /work, executes entrypoint, returns result.
- **Rootfs images**: pre-built ext4 images — Alpine base + runtime (Go/Node/Python). Built via script or Makefile.
- **Kernel**: standard Firecracker-compatible Linux kernel (5.10+), bundled or fetched on first run.
- **Jailer**: Firecracker jailer for production-grade isolation (chroot, namespaces, seccomp, dedicated UID per VM).
- **Frontend updates**: ensure the workload form supports file/archive upload for microVM workloads; show `microvm` as an available isolation mode.

## UX/UI Considerations

- Frontend workload form should support code archive upload (tar.gz) for microVM workloads in addition to inline code.
- Workload detail view should show isolation-specific metadata (VM ID, boot time, vsock connection status).
- Available backends list on the frontend should show Firecracker as online with its capabilities.

## Acceptance Criteria

1. Firecracker backend implements the `Backend` interface from PBI-2.
2. MicroVMs boot successfully with the pre-built kernel and rootfs images.
3. CNI networking creates TAP devices and provides connectivity (guest can reach the host).
4. Guest agent receives workload code over vsock and returns execution results.
5. Go, Node, and Python workloads execute correctly inside microVMs with the respective rootfs.
6. Resource limits (vCPUs, memory, timeout) are enforced per workload.
7. VMs are cleaned up (stopped, TAP removed) after workload completion or failure.
8. A user can submit a microVM workload from the frontend, watch it execute, and see the result.
9. Prometheus metrics include Firecracker-specific stats (boot time, active VMs, vsock latency).

## Dependencies

- **Depends on**: PBI-2 (Backend interface), PBI-3 (Frontend for manual testing)
- **External**: `github.com/firecracker-microvm/firecracker-go-sdk`, Firecracker binary, `tc-redirect-tap` CNI plugin, Linux kernel image, nested KVM support on host

## Open Questions

- Should rootfs images be built in CI and downloaded, or built locally via a Makefile target?
- Should the jailer be enabled from the start or added as a hardening step later?
- How should kernel and Firecracker binary versioning be managed (pinned versions, auto-download)?

## Related Tasks

_Tasks will be created when this PBI moves to Agreed via `/plan-pbi 4`._
