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
   - `k8s-api-design` for public resource semantics
   - `module-config-contract` for values/OpenAPI boundaries
   - module-specific overlay skills for artifact, backend, or domain rules
9. Prefer the repository's primary implementation language for durable
   controller/runtime code. Shell or Python may remain for tooling, build, or
   thin adapter glue only when the plan justifies it.
10. If the task continues an existing canonical workstream, update that bundle
    instead of creating a sibling active bundle.
11. If the controller package map or controller test tree changes, sync:
    - `images/controller/STRUCTURE.ru.md`
    - `images/controller/TEST_EVIDENCE.ru.md`
12. Before handoff, make sure controller-specific verification remains
    explicit and visible in `make verify`.

## Hard rules

- Do not smuggle module-specific artifact names, backend brands, or public API
  semantics into this core skill; pair with overlays instead.
- Do not let durable controller/runtime code drift into scripts by default when
  the repository already has a primary implementation language for that code.

## Output

A controller implementation that is debuggable and operationally predictable.
