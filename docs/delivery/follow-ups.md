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
| 9 | tech-debt | Extract shared E2E test helpers | PBI-4 | 2026-02-19 | pbi2_test.go, pbi4_test.go, pbi9_test.go all duplicate stubBackend, server setup, postAsync, pollStatus helpers. Extract into shared helpers_test.go. |
| 10 | fix | Sync endpoint should default isolation to "auto" | PBI-4 | 2026-02-19 | POST /v1/workloads (sync) does not default isolation to "auto" when omitted, but the async endpoint does. Pre-existing inconsistency — both should behave the same. |
| 11 | enhancement | Add data-testid attributes to frontend for Playwright robustness | PBI-4 | 2026-02-19 | Playwright tests use positional selectors (select:nth(1), .font-mono:last). Adding data-testid attributes would make tests more resilient to layout changes. |
| 12 | ~~fix~~ | ~~SSE log streaming buffered by Next.js rewrite proxy~~ | PBI-4 | 2026-02-19 | **Closed by PBI-10** — Fixed with streaming API route handler + fallback rewrite in next.config.ts. Root cause: rewrites() array applied before route handlers; switching to `{ fallback: [...] }` lets the route handler intercept SSE requests. |
| 13 | ~~fix~~ | ~~SSE log connections not cleaned up after workload completion~~ | PBI-4 | 2026-02-19 | **Closed by PBI-10** — Go backend now sends `event: done` before closing; frontend useLogStream hook handles it and closes EventSource cleanly without reconnect. |
| 14 | tech-debt | Centralize backend URL constant across web project | PBI-10 | 2026-02-20 | `http://localhost:8080` is duplicated in `next.config.ts`, `route.ts` (BACKEND_BASE_URL), and Playwright spec files. Should be a shared env var or config constant. |
| 15 | ~~enhancement~~ | ~~Propagate client abort signal to upstream SSE fetch in streaming route~~ | PBI-10 | 2026-02-20 | **Closed** — already implemented in route.ts (signal: request.signal on line 22). |
| 16 | tech-debt | Simplify SSE proxy route — remove manual ReadableStream piping | PBI-10 | 2026-02-20 | With the `fallback` rewrite fix in next.config.ts, the route handler now receives requests correctly. The manual getReader()/ReadableStream piping may no longer be necessary — passing upstream.body directly might work now. Worth testing to simplify the code. |
