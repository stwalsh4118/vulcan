# PBI 4: Firecracker microVM Backend — Architecture Guide

[Back to PBI](./prd.md) | [View Tasks](./tasks.md)

This document explains how the Firecracker microVM backend works end-to-end, the decisions behind the task breakdown, and the underlying technology (KVM, Firecracker, vsock, CNI) for anyone who needs context on what this PBI is building and why.

---

## Part 1: Background — KVM and Firecracker

### What is KVM?

KVM (Kernel-based Virtual Machine) is a Linux kernel module that turns the Linux kernel into a hypervisor. It exposes a device at `/dev/kvm` that userspace programs can talk to via `ioctl` system calls.

What it actually does: it lets a userspace program say "hey kernel, I want to create a virtual CPU, load these instructions into it, and run them in a hardware-isolated sandbox." The CPU itself (Intel VT-x / AMD-V) has hardware support for this — there's a special CPU mode called "guest mode" where code runs thinking it has full control of the hardware, but the real kernel can intercept and virtualize any privileged operation.

KVM is **not** a VM itself. It's the kernel plumbing that makes VMs possible. It's the same thing QEMU, VirtualBox, and VMware use under the hood on Linux.

### What is Firecracker?

Firecracker is a **VMM** (Virtual Machine Monitor) — the userspace program that talks to `/dev/kvm`. Think of it as an extremely stripped-down alternative to QEMU.

QEMU is a general-purpose VMM. It can emulate USB devices, graphics cards, sound cards, BIOS, UEFI, dozens of CPU architectures. It's millions of lines of code. Firecracker throws all of that away and keeps only:

- A virtio block device (for the root filesystem)
- A virtio network device (for networking)
- A virtio vsock device (for host-guest communication)
- A serial console

This is why Firecracker boots in ~125ms instead of seconds. There's almost nothing to initialize. Amazon built it specifically for Lambda and Fargate — boot a VM, run one function, throw it away.

### Firecracker vs Docker

Docker:
```
docker run python:3.12 python script.py

Docker daemon → pulls OCI image layers → sets up overlayfs →
creates cgroups + namespaces → runs process in container
```

The process runs **on your kernel**. Docker just isolates it with Linux namespaces (separate PID/network/mount views) and cgroups (resource limits). There's no hardware boundary — a kernel exploit in the container can escape to the host.

Firecracker:
```
firecracker --config-file vm.json

Firecracker process → opens /dev/kvm → creates a virtual CPU + RAM →
loads a Linux kernel into VM memory → attaches a root filesystem (ext4 image) →
boots the kernel → kernel runs inside hardware-isolated VM
```

The code runs on **its own kernel** inside a hardware-isolated VM. The guest has no access to the host kernel. Even a kernel exploit inside the VM is contained — you'd need to escape KVM itself (which is a much smaller attack surface than the full Linux syscall API).

There's no daemon like Docker's `dockerd`. Firecracker is just a binary. Each VM is a separate Firecracker process. Our Go code spawns one per workload and manages its lifecycle directly via the SDK. When the workload is done, the process exits and the VM is gone.

### What is vsock?

vsock (virtio socket) is a communication channel between the host and a VM that doesn't require network configuration. It works like a Unix socket but across the VM boundary. Each VM gets a unique Context ID (CID), and you connect to `CID:port` from the host to reach a service inside the guest. It's faster and simpler than setting up networking just for host-guest communication.

### What is CNI?

CNI (Container Network Interface) is a standard for configuring network interfaces. Instead of manually creating TAP devices and managing IP addresses, you write a JSON config file and CNI plugins handle the plumbing. Firecracker needs TAP devices (not veth pairs like Docker), so we use the `tc-redirect-tap` plugin that converts standard CNI output into TAP devices. This is the same approach Kata Containers uses in production.

---

## Part 2: End-to-End Request Flow

When a user clicks "Submit" on the frontend with isolation set to `microvm`:

```
Browser → POST /v1/workloads/async → Engine.Submit() → Registry.Resolve("microvm", "go")
  → FirecrackerBackend.Execute()
    → allocate CID, set up CNI networking (TAP device + IP)
    → configure VM (kernel, rootfs, vcpus, memory, network, vsock)
    → firecracker-go-sdk Machine.Start()  ← actual KVM VM boots here
    → wait for guest agent on vsock
    → send code over vsock → guest agent writes to /work, runs it
    → guest streams stdout lines back over vsock → LogWriter → SSE to browser
    → guest sends final result → stop VM → teardown networking
  → Engine updates workload status to completed
→ Browser polls GET /v1/workloads/:id, sees "completed" + output
```

The whole thing slots into the existing architecture without changing it. PBI 2 defined the `Backend` interface so that adding new backends is a plug-in operation. The engine doesn't know or care that it's talking to Firecracker — it just calls `Execute()` and gets back a `WorkloadResult`.

### Concrete Provisioning Chain

Here's what happens step by step when Vulcan runs a Python workload in a microVM:

```
1. Prerequisites (already on disk):
   - Firecracker binary (downloaded by tools/firecracker/download.sh)
   - Linux kernel image (same download)
   - python.ext4 rootfs image (built by Makefile)

2. Our Go code (via firecracker-go-sdk):
   a. Creates a temp directory for this VM's socket files
   b. Copies or snapshots python.ext4 → /tmp/vulcan-vm-<id>/rootfs.ext4
      (each VM needs its own writable copy)
   c. Calls into the SDK which:
      - Spawns the `firecracker` binary as a child process
      - Talks to it over a Unix socket (Firecracker exposes a REST API)
      - Sends: "here's the kernel, here's the rootfs, 1 vCPU,
        128MB RAM, this TAP network device, this vsock CID"
      - Sends: "boot it"

3. Firecracker (the child process):
   a. Opens /dev/kvm
   b. Creates a VM with the specified resources
   c. Loads the kernel into VM memory
   d. Attaches the rootfs as a virtio block device
   e. Starts the virtual CPU

4. Inside the VM (completely separate kernel):
   a. Linux kernel boots (~100ms)
   b. Kernel runs /usr/local/bin/vulcan-guest as PID 1
   c. Guest agent mounts /proc, /sys, /dev
   d. Guest agent opens vsock listener on port 1024

5. Back on the host:
   a. Our Go code connects to the VM via vsock (CID + port)
   b. Sends: {"runtime": "python", "code": "print('hello')"}
   c. Guest agent writes code to /work/main.py
   d. Guest agent runs: python3 /work/main.py
   e. stdout lines stream back over vsock → LogWriter → SSE → browser
   f. Final result: {"exit_code": 0, "output": "hello\n"}

6. Cleanup:
   a. Tell Firecracker to stop the VM (kills the virtual CPU)
   b. Firecracker process exits
   c. Remove TAP device, network namespace
   d. Delete temp directory with rootfs copy
```

---

## Part 3: Task Breakdown — Layer by Layer

### Layer 1: Config & Protocol (Task 4-1)

The shared vocabulary. Before writing any real code, we need agreement on:

- **How does the host talk to the guest?** Length-prefixed JSON over vsock. Length-prefixed framing (4-byte big-endian length, then JSON payload) because vsock is a stream protocol, not message-oriented. Without framing, you'd have to do delimiter-based parsing which is fragile with arbitrary binary data. Length-prefixed is simple, unambiguous, and the same pattern used by most RPC protocols.

- **What configuration knobs exist?** Kernel path, rootfs directory, Firecracker binary path, CNI paths, vsock port, etc. All loaded from environment variables following the same `VULCAN_*` pattern the existing config uses.

- **What are the magic numbers?** CID starts at 3 (0-2 are reserved by vsock spec), default vsock port 1024, default 1 vCPU and 128MB memory. These become named constants so they're not scattered as literals throughout the codebase.

**Why this is Task 1:** Everything else imports these types. You can't write the guest agent without knowing the protocol format, can't write the backend without knowing the config structure. It's the foundation.

### Layer 2: The Guest Agent (Task 4-2)

A separate Go binary that runs *inside* the VM, not on the host. It's a tiny server:

1. VM boots, Linux kernel starts, init runs `vulcan-guest`
2. Guest agent opens a vsock listener (the guest side)
3. Host connects and sends a `GuestRequest`: "here's some Python code, run it"
4. Guest agent writes the code to `/work/main.py`, runs `python3 /work/main.py`
5. As stdout lines appear, it sends them back over vsock immediately (this is how real-time log streaming works through the VM boundary)
6. When the process exits, it sends a `GuestResponse` with the exit code and any remaining output

**Why vsock instead of other approaches?** Firecracker gives you three options for getting data into/out of a VM:

- **MMDS** (MicroVM Metadata Service): Like AWS instance metadata. Good for small config, terrible for streaming results back.
- **Drive mounting**: Mount a shared filesystem. Complex, requires filesystem coordination, no streaming.
- **vsock**: Bidirectional socket between host and guest. No networking required. Supports streaming. This is what Kata Containers uses and what Firecracker's docs recommend.

vsock wins because we need real-time log streaming (the existing `LogWriter` callback pattern) and it doesn't require the VM to have network connectivity just to communicate with the host.

**Why static compilation?** The binary runs inside a minimal Alpine rootfs. Static linking means zero dependency on shared libraries — just copy the binary in and it works. `CGO_ENABLED=0` gives us a pure Go binary with no glibc dependency.

**Why it might run as PID 1:** In a microVM, there's no systemd or init system. The kernel needs something to run. The simplest approach is making the guest agent the init process itself. It needs to mount `/proc`, `/sys`, `/dev` because nothing else will.

### Layer 3: Kernel & Rootfs Build Tooling (Task 4-3)

Firecracker doesn't use container images. It boots a real Linux kernel with a real root filesystem. So we need:

- **A kernel**: Firecracker maintains official compatible kernels (5.10+). We download a specific pinned version. No reason to build our own.
- **Rootfs images**: ext4 filesystem images, one per runtime. Each contains Alpine Linux (tiny, ~5MB base), the runtime (`go`, `node`, or `python3` package), and the guest agent binary.

**Why Alpine?** Smallest practical Linux distro. A Go rootfs image will be maybe 200-300MB instead of 1GB+ with Ubuntu. Smaller images = faster VM boot = better cold start times.

**Why ext4 and not squashfs?** Firecracker expects a block device. ext4 is the standard, well-supported, and the guest agent needs to write code to `/work/` during execution. squashfs is read-only.

**Why separate images per runtime instead of one big image?** Each image stays small and purpose-built. A Go developer's workload doesn't need Node.js installed. This also maps cleanly to the runtime selection — `spec.Runtime = "go"` → mount `go.ext4`.

**Why this depends on 4-2:** The guest agent binary gets baked into the rootfs image. You have to build it first.

### Layer 4: vsock Host-Side Communication (Task 4-4)

The mirror of what the guest agent does, but from the host side:

1. After the VM boots, the host dials into it via vsock (CID identifies the specific VM, port identifies the service)
2. Sends the `GuestRequest` using the length-prefixed framing
3. Enters a read loop: guest sends log lines (type-tagged as "log") and finally a result (type-tagged as "result")
4. Each log line triggers the `spec.LogWriter` callback, which feeds into the existing LogBroker → SSE → frontend pipeline

**Why retry logic on connect?** The VM takes ~125ms to boot. The guest agent needs a moment to start listening. So the host retries with exponential backoff (100ms, 200ms, 400ms...) up to 5 attempts. This is the "wait for guest agent readiness" step.

**Why this is separate from the backend (4-6)?** Separation of concerns. The vsock communication layer is testable in isolation with mock connections. The backend orchestrates the full lifecycle. Keeping them separate means you can unit test the protocol handling without needing a real VM.

### Layer 5: CNI Networking (Task 4-5)

Each VM needs a network interface if the code inside wants internet access (e.g., `pip install`, `go get`, API calls). Firecracker uses TAP devices, and the standard way to manage those is CNI:

1. Create a network namespace for the VM
2. Run CNI ADD with the `bridge` plugin → creates a veth pair, one end in the namespace
3. `tc-redirect-tap` plugin converts the veth into a TAP device that Firecracker understands
4. The VM gets an IP on a private subnet (e.g., `10.168.0.x/24`)
5. nftables masquerade rule provides outbound NAT (VM can reach the internet through the host)

**Why CNI and not manual TAP management?** The firecracker-go-sdk has native CNI support. CNI is declarative (JSON config) and handles all the plumbing. Manual TAP management means writing ioctl calls, managing IP allocation yourself, and handling edge cases that CNI plugins already solve. This is the same approach Kata Containers uses in production.

**Why `tc-redirect-tap` specifically?** Standard CNI plugins create veth pairs. Firecracker needs TAP devices. `tc-redirect-tap` is a CNI plugin maintained by AWS that bridges this gap — it takes the veth output and redirects traffic to a TAP device. It was built specifically for Firecracker.

**Why this can run parallel to 4-2/4-3:** CNI networking only depends on config types from 4-1. It doesn't need the guest agent or rootfs images. So tasks 4-2/4-3 (guest agent path) and 4-4/4-5 (host communication path) can be developed in parallel, converging at 4-6.

### Layer 6: The Core Backend (Task 4-6)

The integration hub. Implements the three-method `Backend` interface:

**`Execute(ctx, spec)`** — the main event:
1. Pick rootfs by runtime (`spec.Runtime = "python"` → `python.ext4`)
2. Allocate a unique CID (atomic counter, thread-safe)
3. Call NetworkManager.Setup() → get TAP device + IP
4. Build Firecracker VM config via the SDK (kernel, rootfs, vcpus, memory, network interface, vsock device)
5. Start the VM
6. Connect via vsock, send workload, stream logs, get result
7. Stop VM, teardown network, release CID
8. Return `WorkloadResult`

**`Capabilities()`** — tells the registry what this backend can do: runtimes `[go, node, python]`, isolation `[microvm]`.

**`Cleanup(workloadID)`** — emergency cleanup. The engine calls this if something goes wrong outside normal execution flow (e.g., workload killed by user via DELETE endpoint).

**Why CID management matters:** vsock uses a Context ID (CID) to identify each VM. CID 0-2 are reserved. If you start two VMs with the same CID, vsock routing breaks. An atomic counter starting at 3 gives each VM a unique CID. Released on cleanup so they can be reused over time.

**Why copy-on-write for rootfs:** The rootfs images are shared read-only templates. Each VM needs its own writable copy (the guest agent writes code to `/work/`). Options are: full copy (simple, slow for large images), or a device-mapper/overlay snapshot (fast, more complex). The task leaves this as an implementation decision.

**Why conditional registration:** The backend only registers if the Firecracker binary and kernel actually exist on the host. If you're developing on a Mac without KVM, the backend silently doesn't register, and everything else still works. The frontend already handles this — it greys out unavailable isolation modes.

### Layer 7: Prometheus Metrics (Task 4-7)

Five new metrics, following the exact same pattern as the existing HTTP metrics in `internal/api/metrics.go`:

- **Boot time histogram** (`vulcan_firecracker_vm_boot_seconds`): How long from `Machine.Start()` to vsock connection established. This is the cold start metric — the number that makes Firecracker compelling (~125ms).
- **Active VMs gauge** (`vulcan_firecracker_active_vms`): How many VMs are running right now. Critical for capacity planning and detecting leaks.
- **vsock latency histogram** (`vulcan_firecracker_vsock_latency_seconds`): Round-trip time for the workload protocol. Measures communication overhead.
- **Cleanup duration histogram** (`vulcan_firecracker_vm_cleanup_seconds`): How long teardown takes. Helps detect slow cleanup.
- **Workloads total counter** (`vulcan_firecracker_workloads_total`): How many workloads by runtime and outcome. Usage patterns.

### Layer 8: Frontend Enhancements (Task 4-8)

Two additions:

1. **Archive upload**: The existing form only supports inline code. For microVM workloads, users might want to upload a tarball with multiple files (e.g., a Go project with `go.mod`). This adds a toggle between "Inline Code" and "Upload Archive" modes, plus a file input with drag-and-drop. The archive gets base64-encoded and sent through the API.

2. **Metadata display**: When viewing a completed microVM workload, show a "microVM Details" section with the isolation mode badge and runtime. The structure is future-proofed so when boot time and VM ID are added to the API response later, they'll just appear.

**Why extend the API with `code_archive`?** The existing `code` field is a string. Binary archive data needs a separate field (`[]byte`). Adding `CodeArchive` to `WorkloadSpec` is cleaner than overloading `code` with a prefix marker — it keeps the type system honest.

### Layer 9: E2E CoS Test (Task 4-9)

Tests each of the 9 acceptance criteria in two categories:

- **API-level tests** (Go test binary): Submit workloads via HTTP, verify results. Tests AC1-AC7 and AC9. These exercise the full backend path without needing a browser.
- **Playwright tests**: AC8 specifically — submit from the browser, watch status progress, see output. This is the "can a real user do it" test.

**Why skippable?** These tests require KVM support (`/dev/kvm`). CI runners or dev machines without KVM should skip gracefully rather than fail. A build tag or environment check gates execution.

---

## Part 4: Dependency Structure and Parallelism

```
4-1 is the root — everything needs types/config
  │
  ├── 4-2 → 4-3: guest agent must exist before rootfs can bake it in
  │         │
  ├── 4-4 ──┤
  │         ├──► 4-6: backend needs all three pieces
  └── 4-5 ──┘       │
                     ├──► 4-7: metrics instrument the backend
                     └──► 4-8: frontend needs a working backend to test against
                               │
                               └──► 4-9: E2E tests need everything
```

The critical path is: **4-1 → 4-2 → 4-3 → 4-6 → 4-9**. Tasks 4-4 and 4-5 can run in parallel with the guest agent work to shorten the overall timeline.

---

## Part 5: Open Questions and Assumptions

These assumptions were made during planning — they can be revisited:

1. **Rootfs built locally via Makefile** (not CI download). Simpler for development, and the build tooling is educational (part of the "as a learner" user story). CI builds can be added later.

2. **Jailer deferred**. The backend config includes `JailerEnabled` (default false), but Task 4-6 focuses on getting basic VM execution working first. Jailer adds chroot, seccomp, and UID isolation — important for production but not needed to prove the concept. It can be a follow-up task or PBI.

3. **Pinned versions for Firecracker and kernel**. Explicit version variables in the download script. No auto-update magic. Reproducible builds matter more than being on latest.

See `docs/delivery/follow-ups.md` for the full list of deferred items captured during planning.
