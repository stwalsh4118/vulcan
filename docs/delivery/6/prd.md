# PBI-6: gVisor Backend

[View in Backlog](../backlog.md)

## Overview

Implement the gVisor backend using containerd and runsc. This is the third and final isolation tier — syscall-level isolation via gVisor's user-space kernel, supporting any language via standard OCI container images. After this PBI, all three backends are operational and users can compare them side by side from the frontend.

## Problem Statement

Firecracker gives hardware isolation but requires custom rootfs images. V8 isolates are fast but JS/TS only. gVisor fills the gap — it runs standard OCI container images (Docker images) with stronger isolation than regular containers. This completes Vulcan's isolation spectrum and is the backend most users will reach for first since it works with any existing container image.

## User Stories

- As a user, I want to run OCI container images in gVisor so that I can execute any containerized application with syscall-level isolation.
- As a user, I want to specify a Docker image reference (e.g., `alpine:latest`) and have Vulcan pull and run it.
- As a user, I want to test gVisor workloads from the frontend and compare them against Firecracker and isolate execution.

## Technical Approach

- **containerd Go client**: pull OCI images, create containers, manage lifecycle. containerd must be running on the host as a prerequisite.
- **runsc as OCI runtime**: configure containerd to use `runsc` for Vulcan's containers. gVisor's Sentry kernel intercepts syscalls in user space.
- **Container lifecycle**: pull image (cached after first pull) → create container with resource limits via OCI spec → start → capture stdout/stderr → destroy.
- **Resource limits**: CPU and memory limits set via OCI runtime spec (cgroup enforcement). Timeout via context deadline.
- **Image caching**: containerd handles image layer caching natively. First pull is slow; subsequent runs reuse cached layers.
- **Frontend updates**: workload form should support OCI image reference input (text field for image name); show `gvisor` as available in the isolation selector; `auto` mode should route OCI image references to gVisor.

## UX/UI Considerations

- Frontend workload form needs an OCI image reference input field, visible when gVisor isolation is selected or when the workload type is "container."
- Should also support an optional command override (e.g., `echo hello` inside `alpine:latest`).
- Workload detail view should show container-specific metadata (image pulled, container ID, pull time vs execution time).

## Acceptance Criteria

1. gVisor backend implements the `Backend` interface from PBI-2.
2. OCI images are pulled via containerd and cached locally.
3. Containers execute with `runsc` (gVisor) as the OCI runtime.
4. Stdout/stderr are captured and returned as workload output.
5. Resource limits (CPU, memory, timeout) are enforced via OCI spec.
6. Containers are cleaned up after execution (no orphaned containers).
7. A user can submit an OCI image reference from the frontend and see the container execute.
8. `auto` mode correctly routes OCI image references to the gVisor backend.
9. Prometheus metrics include gVisor-specific stats (image pull time, container start time, active containers).

## Dependencies

- **Depends on**: PBI-2 (Backend interface), PBI-3 (Frontend for manual testing)
- **External**: containerd (running on host), `runsc` (gVisor runtime installed), OCI-compatible container images

## Open Questions

- Should containerd be managed (started/stopped) by Vulcan, or assumed to be running as a system service?
- How should image pull authentication work for private registries (deferred to future, or basic support now)?
- Should there be a size limit on images that can be pulled?

## Related Tasks

_Tasks will be created when this PBI moves to Agreed via `/plan-pbi 6`._
