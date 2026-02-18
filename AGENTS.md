# Project Policy — Core Rules

> Detailed workflows live in skills. Load the relevant skill BEFORE starting any workflow step.
> Shared reference material is in `~/.codex/_references/`.

## Actors

- **User**: Defines requirements, prioritises work, approves changes. Accountable for all code modifications.
- **AI_Agent**: Executes the User's instructions precisely as defined by PBIs and tasks.

## Tooling

- **Package Manager**: `pnpm` only (never npm or yarn).
- **Browser Automation**: Playwright MCP server available for browser-based verification.

## Core Principles

1. **Task-Driven Development**: No code changes without an agreed-upon task authorising them.
2. **PBI Association**: No task exists without a parent PBI.
3. **PRD Alignment**: PBI features must align with the PRD scope when one exists.
4. **User Authority**: The User is the sole decider for scope and design of ALL work.
5. **Prohibition of Unapproved Changes**: Changes outside task scope are EXPRESSLY PROHIBITED.
6. **Task Status Sync**: Status in the task index and individual task file must always match. Update both immediately.
7. **Controlled File Creation**: No files outside defined PBI/task/source structures without explicit User confirmation.
8. **External Package Research**: Before using external packages, research docs via web. Create `<task-id>-<package>-guide.md` with date-stamped API usage examples. No hallucinated APIs.
9. **Task Granularity**: As small as practicable while still a cohesive, testable unit.
10. **DRY**: Define information once, reference elsewhere. Task details live in task files only.
11. **Named Constants**: Any value used more than once must be a named constant.
12. **Technical Documentation**: PBIs that create/modify APIs must include/update technical docs.

## Change Management

- Every code change conversation must start by identifying the linked PBI or Task.
- All changes must be associated with a specific task.
- No changes outside current task scope. Scope creep must be rolled back and addressed in a new task.
- If the User requests a change without a task reference, STOP — discuss whether it maps to an existing task or needs a new PBI + task.

## Scope Limitations

- No gold plating or scope creep. All work scoped to the specific task.
- Improvements or optimizations must be proposed as separate tasks.
- Consult API specs before implementing to avoid duplication.

## Available Skills

| Skill | Trigger |
|-------|---------|
| **plan-pbi** | Breaking a PBI into tasks |
| **implement-pbi** | Implementing all tasks for a PBI sequentially |
| **implement-task** | Implementing a single task |
| **complete-task** | Marking a single task as Done |
| **complete-pbi** | Marking all PBI tasks as Done |
| **pr-pbi** | Creating a pull request for a PBI |

## Shared References

Detailed policy material lives in `~/.codex/_references/`:

| Reference | Content |
|-----------|---------|
| `pbi-management.md` | PBI statuses, transitions, backlog format, detail doc structure |
| `task-management.md` | Task statuses, transitions, documentation format, status sync rules |
| `testing-strategy.md` | Test scoping, test plans, browser verification, test pyramid |
| `api-specs.md` | API spec location, format, loading/update requirements |
| `code-quality.md` | Linters, formatting, pre-review checks, commit conventions |
