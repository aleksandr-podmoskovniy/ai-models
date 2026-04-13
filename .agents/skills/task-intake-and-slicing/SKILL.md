---
name: task-intake-and-slicing
description: Use first for any non-trivial request. Converts a human request into a task bundle under plans/active/<slug>/ with scope, non-goals, acceptance criteria, slices, validations, and rollback point.
---

# Task intake and slicing

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. `docs/development/TASK_TEMPLATE.ru.md`
5. `plans/README.md`

## Use this skill when

- the request is larger than one focused code change;
- the request changes architecture, module shell, internal backend integration, or public API;
- the request is still fuzzy and needs to be turned into an executable task.

## Workflow

1. Define the current project phase.
2. Restate the user request in platform terms.
3. Write explicit scope and non-goals.
4. Write acceptance criteria that can actually be checked.
5. If the task touches controller/runtime architecture, include explicit architecture acceptance criteria:
   - use case / port / adapter split
   - max LOC if relevant
   - complexity/quality-gate expectations
   - test evidence shape
6. Split the work into slices with concrete file areas.
7. Add validation commands per slice.
8. Add one rollback point.
9. Decide orchestration mode: `solo`, `light`, or `full`.
10. If the task is not `solo`, name the read-only subagents that should review it before implementation.
11. If the task touches repo-local workflow governance (`AGENTS.md`, `.codex/*`, `.agents/skills/*`, `.codex/agents/*`, `docs/development/CODEX_WORKFLOW.ru.md`, `docs/development/TASK_TEMPLATE.ru.md`, `docs/development/REVIEW_CHECKLIST.ru.md`, `plans/README.md`), make it a dedicated governance bundle and add explicit consistency acceptance criteria across all touched instruction layers.
12. If the task defines durable project discipline, encode it in repo-local skills or skill references instead of leaving it only in the current bundle.
13. Avoid duplicate active slugs for the same workstream, reuse the current canonical active bundle when the request is a continuation, and archive stale/finished or oversized active bundles when active context drifts.
14. Save the result in `plans/active/<slug>/TASK.ru.md` and `plans/active/<slug>/PLAN.ru.md`.

## Output

A task bundle that another agent can execute without guessing what the user meant.
