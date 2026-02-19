# Follow-Ups

Ideas, improvements, and deferred work captured during planning and implementation.
Review periodically — good candidates become new PBIs via `/new-pbi`.

## Open

| # | Type | Summary | Source | Date | Notes |
|---|------|---------|--------|------|-------|
| 1 | enhancement | Firecracker jailer integration for production-grade isolation | PBI-4 | 2026-02-19 | Deferred from initial implementation. Config includes `JailerEnabled` flag (default false). Jailer adds chroot, namespaces, seccomp, and per-VM UID isolation. Should be a dedicated task or PBI once basic VM execution is proven. |
| 2 | enhancement | CI-built rootfs images with artifact download | PBI-4 | 2026-02-19 | Current plan builds rootfs locally via Makefile (simpler for dev). For team workflows, rootfs images should be built in CI and downloadable as versioned artifacts to avoid requiring root/mount privileges on every dev machine. |
| 3 | perf | VM warm pool for reduced cold starts | PBI-4 | 2026-02-19 | Firecracker cold starts are ~125ms. A warm pool of pre-booted VMs per runtime could eliminate boot latency entirely for bursty workloads. Trade-off: memory overhead for idle VMs. |
| 4 | perf | Copy-on-write rootfs overlays instead of full copies | PBI-4 | 2026-02-19 | Each VM needs a writable rootfs. Full copy is simple but slow for large images. Device-mapper thin provisioning or overlayfs snapshots would be faster. Revisit after measuring boot time with full copies. |
| 5 | enhancement | Auto-download Firecracker binary and kernel on first run | PBI-4 | 2026-02-19 | Current plan uses pinned versions with manual `make download`. Auto-downloading on first server start (if binaries missing) would improve DX for new contributors. Risk: network dependency at startup. |
| 6 | feature | Multi-file workload editor in frontend | PBI-4 | 2026-02-19 | Archive upload (tar.gz) covers multi-file projects, but a multi-tab code editor in the browser would be a better UX for small projects. Lower priority than archive upload. |
| 7 | enhancement | Firecracker snapshot/restore for instant cold starts | PBI-4 | 2026-02-19 | Firecracker supports VM snapshots. Boot a VM, snapshot it after guest agent is ready, then restore from snapshot instead of booting. Could reduce cold start from ~125ms to <5ms. Significant complexity. |
| 8 | docs | Firecracker host prerequisites guide | PBI-4 | 2026-02-19 | Document KVM setup, nested virtualization config for cloud VMs, required kernel modules, and troubleshooting. Important for contributors who don't have bare-metal Linux. |
