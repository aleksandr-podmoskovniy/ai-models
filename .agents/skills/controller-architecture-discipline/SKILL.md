---
name: controller-architecture-discipline
description: Use for ai-models controller implementation or refactor work when the task touches images/controller/internal. Encodes the current corrective discipline: hexagonal split, no new feature work on fat controller packages, thin reconcilers, quality-gate compliance, and branch-matrix testing.
---

# Controller architecture discipline

## Read first

1. `AGENTS.md`
2. `references/controller-discipline.md`
3. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task touches `images/controller/internal/*`;
- the task implements or refactors controller lifecycle code;
- the task changes publication, upload, materialization, or deletion flow;
- the task risks adding new code on top of current fat controller boundaries.

## Workflow

1. Read `references/controller-discipline.md` before changing controller lifecycle code.
2. Do not add new feature work to current fat packages when the same slice can start the planned corrective cut instead.
3. Keep the split explicit:
   - `domain`
   - `application`
   - `ports`
   - `adapters`
4. Treat reconcilers as thin K8s adapters only.
5. Move lifecycle decisions, transitions, and policy branching into domain/application code.
6. Keep K8s object rendering and persistence semantics in adapter code.
7. Follow the corrective cut order from `references/controller-discipline.md` unless the current task bundle explicitly justifies a different slice.
8. Before closing a slice, make sure it still passes controller quality gates from `make verify`.
9. Require branch/state-matrix testing for lifecycle logic; do not treat happy-path adapter tests as sufficient evidence.

## Hard rules

- No fat reconciler growth.
- No inline `Pod` / `Service` / `Secret` / `ConfigMap` assembly inside reconciler files.
- No mixing ConfigMap serialization format with domain contract.
- No “service” extraction that still mixes K8s object shapes and lifecycle decisions.
- No new controller feature slice before checking whether the corrective refactor bundle should land first.

## Output

A controller change that follows the current corrective architecture path instead
of compounding the old monolith.
