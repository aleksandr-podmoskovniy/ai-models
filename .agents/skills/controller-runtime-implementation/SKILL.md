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
7. If the task touches current fat controller packages, follow the corrective refactor order before adding new feature logic.
8. For phase-2 runtime/materialization work, keep `ModelPack` as the contract,
   `OCI from registry` as the only runtime input, and concrete tools such as
   `KitOps`, `Modctl`, or init images behind adapters.
9. If the task continues an existing canonical phase-2 workstream, update that
   bundle instead of creating a sibling active bundle.
10. Keep the concrete package map stable:
   - reconcilers under `internal/controllers/*`
   - K8s object/service adapters under `internal/adapters/k8s/*`
   - shared helper code only under `internal/support/*`
   - shared ports must be implemented by concrete adapters, not by a temporary
     controller-side wrapper package
   - canonical owner-based resource naming and owner-label policy live only in
     `internal/support/resourcenames`, not in package-local `names.go` shims
   - concrete adapters should consume shared `publication.OperationContext`
     directly; do not clone it into local `Request` / `OwnerRef` wrappers
     unless the adapter truly needs a different boundary
   - do not keep a second `runtime.go` proxy layer if the same concrete adapter
     can implement the shared port directly and keep CRUD as unexported helper
     methods
   - do not duplicate the same controlled-resource create/reuse shell across
     multiple adapters; keep one shared helper under `internal/adapters/k8s/*`
     when only the concrete object type differs
   - do not duplicate the same workload `Pod` shell (`EmptyDir` workspace,
     `/tmp` mount, registry CA volumes/mounts) across worker/session adapters;
     centralize it under `internal/adapters/k8s/*`
   - do not elevate a controller-local persisted protocol helper into
     `internal/ports/*` until there is a real second adapter behind that seam;
     fake shared store interfaces are architecture debt, not reuse
11. Keep controller tests systematic:
   - shared scheme/object/fake-client fixtures under `internal/support/testkit`
   - package-local `test_helpers_test.go` only for adapter-local builders and
     assertions
   - split adapter-heavy reconcile coverage by decision family; do not let a
     single `reconciler_test.go` become the package dumping ground
   - business decisions stay in domain/application tests, not in helper files
12. If the controller package map changes or grows, sync
    `images/controller/STRUCTURE.ru.md` so every folder/file keeps an explicit
    rationale.

## Output

A controller implementation that is debuggable and operationally predictable.
