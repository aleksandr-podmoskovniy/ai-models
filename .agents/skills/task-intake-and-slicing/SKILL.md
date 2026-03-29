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
5. Split the work into slices with concrete file areas.
6. Add validation commands per slice.
7. Add one rollback point.
8. Decide orchestration mode: `solo`, `light`, or `full`.
9. If the task is not `solo`, name the read-only subagents that should review it before implementation.
10. Save the result in `plans/active/<slug>/TASK.ru.md` and `plans/active/<slug>/PLAN.ru.md`.

## Output

A task bundle that another agent can execute without guessing what the user meant.
