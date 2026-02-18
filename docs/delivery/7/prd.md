# PBI-7: CLI Tool

[View in Backlog](../backlog.md)

## Overview

Build the `vulcan` CLI using Cobra, providing terminal-based access to all Vulcan API functionality. The CLI is the power-user and scripting interface — submit workloads, stream logs, check status, list workloads, view nodes, and run benchmarks, all from the command line.

## Problem Statement

The frontend (PBI-3) provides visual interaction, but many workflows are faster from the terminal — especially scripting, piping output, and running automated benchmarks. The CLI also serves as a reference client for the API, demonstrating every endpoint's usage.

## User Stories

- As a user, I want to run workloads from the terminal so that I can script and automate compute tasks.
- As a user, I want to stream logs from the CLI so that I can tail workload output like any other process.
- As a user, I want to run benchmarks from the CLI and see a comparison table so that I can quickly evaluate backend performance.

## Technical Approach

- **Cobra** for command structure, flag parsing, help generation.
- **Commands**:
  - `vulcan run` — execute a workload (flags: `--runtime`, `--isolation`, `--code`, `--file`, `--image`, `--cmd`, `--input`, `--cpus`, `--mem`, `--timeout`). Supports inline code, file path, or OCI image.
  - `vulcan status <id>` — show workload status and result.
  - `vulcan logs -f <id>` — stream workload logs (SSE client in the terminal).
  - `vulcan ls` — list recent workloads (table format).
  - `vulcan nodes` — list cluster nodes and capacity.
  - `vulcan benchmark` — run built-in benchmark suite, output comparison table.
- **Output formatting**: table output for lists, JSON output with `--json` flag, colored status indicators.
- **API client**: shared HTTP client targeting the Vulcan API base URL (configurable via `--api` flag or `VULCAN_API` env var).
- **SSE client**: for `vulcan logs -f`, consume the SSE stream and print lines to stdout in real time.

## UX/UI Considerations

N/A — CLI tool. Focus on clear output formatting, helpful error messages, and sensible defaults.

## Acceptance Criteria

1. `vulcan run --runtime node --code 'export default () => "hello"'` submits and returns the result.
2. `vulcan run --runtime python --file ./script.py --input '{"n": 42}'` submits a file-based workload.
3. `vulcan run --image alpine:latest --cmd "echo hello"` submits an OCI workload.
4. `vulcan status <id>` displays workload status, output, duration, and isolation used.
5. `vulcan logs -f <id>` streams log lines to stdout in real time.
6. `vulcan ls` displays a formatted table of recent workloads.
7. `vulcan nodes` displays node information and capacity.
8. `vulcan benchmark` runs workloads across all available backends and outputs a comparison table.
9. All commands support `--json` for machine-readable output.
10. API base URL is configurable via `--api` flag or `VULCAN_API` environment variable.

## Dependencies

- **Depends on**: PBI-1 (API endpoints)
- **External**: `github.com/spf13/cobra`

## Open Questions

- Should `vulcan run` default to sync execution, or async with auto-polling until complete?
- Should there be a `vulcan config` command for persisting API URL and other settings?

## Related Tasks

_Tasks will be created when this PBI moves to Agreed via `/plan-pbi 7`._
