---
name: controller-runtime-implementation
description: Use for controller implementation work: reconciliation boundaries, watches, finalizers, status updates, and narrow verification loops.
---

# Controller runtime implementation

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task implements or refactors a controller;
- the task changes reconciliation flow, status writing, finalizers, or watches;
- the task changes the mapping between public resources and internal systems.

## Workflow

1. Keep reconcile ownership and side effects explicit.
2. Separate desired-state resolution from status reporting.
3. Make finalizer and deletion flow easy to reason about.
4. Keep validations narrow and repeatable.
5. Do not expand controller scope without an explicit plan.

## Output

A controller implementation that is debuggable and operationally predictable.
