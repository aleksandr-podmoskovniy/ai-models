---
name: controller-architecture-discipline
description: Use for controller implementation or refactor work when the task touches controller runtime internals and needs durable architecture guardrails: thin reconcilers, explicit domain/application/ports/adapters split, quality-gate compliance, and centralized controller test evidence.
---

# Controller architecture discipline

## Read first

1. `AGENTS.md`
2. `references/controller-discipline.md`
3. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task touches `images/controller/internal/*`;
- the task implements or refactors controller lifecycle code;
- the task changes controller-owned lifecycle orchestration, external workload
  management, or deletion/finalizer flow;
- the task risks adding new code on top of already-fat controller boundaries.

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
9. Require state-matrix testing for lifecycle logic plus centralized controller
   test evidence; do not treat happy-path adapter tests as sufficient evidence.
10. Treat `_test.go` files as part of the architecture:
   - keep them under the same LOC discipline as production files
   - split by decision surface
   - do not hide business logic inside helper-only test files

## Hard rules

- No fat reconciler growth.
- No controller test monolith growth.
- No inline `Pod` / `Service` / `Secret` / `ConfigMap` assembly inside reconciler files.
- No mixing ConfigMap serialization format with domain contract.
- No “service” extraction that still mixes K8s object shapes and lifecycle decisions.
- No new controller feature slice before checking whether the corrective refactor bundle should land first.

## Output

A controller change that follows the current corrective architecture path instead
of compounding the old monolith.
