---
name: controller-runtime-implementation
description: Use for controller implementation work: reconciliation boundaries, watches, finalizers, status updates, and narrow verification loops.
---

# Controller runtime implementation

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. `.agents/skills/controller-architecture-discipline/SKILL.md`
5. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task implements or refactors a controller;
- the task changes reconciliation flow, status writing, finalizers, or watches;
- the task changes the mapping between public resources and internal systems.

## Workflow

1. Keep reconcile ownership and side effects explicit.
2. Separate lifecycle decisions from K8s adapter code.
3. Separate desired-state resolution from status reporting.
4. Make finalizer and deletion flow easy to reason about.
5. Keep validations narrow and repeatable.
6. Do not expand controller scope without an explicit plan.
7. If the task touches current fat controller packages, follow the corrective
   refactor order before adding new feature logic.
8. Use neighboring skills instead of duplicating their concerns here:
   - `controller-architecture-discipline` for package map, LOC discipline,
     thin reconcilers, and controller test shape
   - `model-catalog-api` for public `Model` / `ClusterModel` contract
   - `module-config-contract` for values/OpenAPI boundaries
9. For phase-2 runtime/materialization work, keep:
   - `ModelPack` as the publication contract
   - immutable `OCI from registry` as the only runtime input
   - concrete tools (`KitOps`, `Modctl`, init images) behind adapters
10. Prefer Go-first phase-2 data-plane code by default. Python or shell may
    remain only where they are strictly phase-1 backend-adjacent or
    build/install tooling.
11. If the task continues an existing canonical workstream, update that bundle
    instead of creating a sibling active bundle.
12. If the controller package map or controller test tree changes, sync:
    - `images/controller/STRUCTURE.ru.md`
    - `images/controller/TEST_EVIDENCE.ru.md`
13. Before handoff, make sure controller-specific verification remains
    explicit and visible in `make verify`.

## Output

A controller implementation that is debuggable and operationally predictable.
