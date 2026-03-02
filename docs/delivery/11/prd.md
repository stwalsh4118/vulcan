# PBI-11: .env File Configuration Support

[View in Backlog](../backlog.md)

## Overview

Add support for loading configuration from a `.env` file in the project root so developers no longer need to pass all `VULCAN_*` environment variables inline on every command invocation. Uses `godotenv` to load the file at startup, with explicit environment variables taking precedence over `.env` values.

## Problem Statement

Currently, starting the Vulcan API server requires passing all configuration as inline environment variables:

```bash
sudo env \
  PATH="$PATH" \
  VULCAN_FC_BIN=../tools/firecracker/bin/firecracker \
  VULCAN_FC_KERNEL_PATH=../tools/firecracker/bin/vmlinux \
  VULCAN_FC_ROOTFS_DIR=../tools/firecracker/images \
  VULCAN_FC_CNI_BIN_DIR=/opt/cni/bin \
  go run ./cmd/vulcan
```

This is error-prone, hard to remember, and tedious during development. A `.env` file lets developers configure once and run simply with `sudo go run ./cmd/vulcan`.

## User Stories

- As a developer, I want to store configuration in a `.env` file so that I can start the server without passing environment variables on every command.

## Technical Approach

1. Add `github.com/joho/godotenv` as a dependency.
2. Call `godotenv.Load()` early in `cmd/vulcan/main.go` before any config loading. If no `.env` file exists, silently continue (this is the standard godotenv behavior with `godotenv.Load()` vs `godotenv.Overload()`).
3. `godotenv.Load()` only sets variables that are **not already set** in the environment, so explicit env vars naturally override `.env` values with no extra logic.
4. Create `.env.example` documenting all 11 `VULCAN_*` variables with comments explaining each.
5. Add `.env` to the root `.gitignore`.

No changes to the config packages themselves (`internal/config/` or `internal/backend/firecracker/`) since they already read from `os.Getenv()`.

## UX/UI Considerations

N/A — backend/infrastructure PBI.

## Acceptance Criteria

1. A `.env` file in the project root is loaded at startup.
2. Explicit environment variables override `.env` values.
3. `.env.example` documents all 11 `VULCAN_*` variables with comments.
4. `.env` is in `.gitignore` (root level).
5. Existing behavior is unchanged when no `.env` file exists (no error, no warning).
6. `go test ./...` passes with no regressions.

## Dependencies

- **Depends on**: None
- **Blocks**: None
- **External**: `github.com/joho/godotenv`

## Open Questions

None.

## Related Tasks

_Tasks will be created when this PBI is planned via `/plan-pbi 11`._
