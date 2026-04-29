# PLAN: Controller structure and architecture alignment

## Current Phase

Phase 2: distribution/runtime topology уже активно развивается, но baseline
controller tree должен оставаться explainable и не должен копить stale
third-party integration narrative.

## Active Bundle Disposition

- keep `csi-workload-ux-contract`: связан с pending e2e rollout/validation.
- keep `live-e2e-ha-validation`: executable validation stream.
- keep `modelpack-efficient-compression-design`: separate storage-layout design.
- keep `observability-signal-hardening`: separate observability workstream.
- current bundle: `controller-structure-architecture-alignment`.

## Orchestration

Mode: `full`.

Read-only review:

- `repo_architect`: package boundaries, anti-patchwork, virtualization-style
  package discipline.

Findings captured:

- `STRUCTURE.ru.md` was stale: live `modelsource`, `backendprefix` and shared
  `workloaddelivery` contract were missing.
- Empty `internal/application/sourceadmission/` contradicted the documented
  no-placeholder rule.
- `nodecache` was importing workload-delivery annotation contract and parsing
  workload annotations directly. This leaked workload trust/projection
  semantics into cache-core and created an avoidable boundary triangle.
- The old `workloaddelivery` / `modeldelivery` seam is a package-level hotspot:
  no single giant file is the problem, but delivery resolution/mutation/gating
  must not grow third-party controller shims again.

## Slice 1. Truthful Structure Doc

Files:

- `images/controller/STRUCTURE.ru.md`

Work:

- sync package map with actual tree;
- add `modelsource`, `workloaddelivery`, `backendprefix`;
- remove stale KubeRay/RayCluster live baseline wording;
- add rules preventing third-party controller-specific branches from returning
  into workload delivery.
- remove empty `internal/application/sourceadmission/` placeholder.

Checks:

- `git diff --check -- images/controller/STRUCTURE.ru.md`

## Slice 2. Narrow Coupling Cleanup

Files:

- `images/controller/internal/adapters/k8s/modeldelivery/delivery_gate.go`
- `images/controller/internal/adapters/k8s/modeldelivery/service.go`

Work:

- remove unused context/template parameters from delivery gate helper;
- keep gate decision pure over topology, input and managed-cache options.
- move resolved workload annotation projection from `internal/nodecache` into
  `internal/workloaddelivery`; keep `nodecache` focused on cache artifact DTO
  normalization/layout.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery`

## Slice 3. Architect Findings

Files:

- `plans/active/controller-structure-architecture-alignment/PLAN.ru.md`
- possible follow-up code files only if the finding is safe and bounded.

Work:

- capture `repo_architect` findings;
- either implement the next small refactor or record it as next executable
  slice.

Checks:

- package-specific tests for touched code;
- `git diff --check`.

## Rollback Point

Revert this bundle plus the specific `STRUCTURE.ru.md` and narrow code cleanup
changes. No runtime state or public API migration is involved.

## Final Validation

- `git diff --check`
- targeted Go tests for touched controller/adapter package
- `review-gate`

Run:

- `git diff --check -- images/controller/STRUCTURE.ru.md ... plans/active/controller-structure-architecture-alignment`
- `cd images/controller && go test ./internal/workloaddelivery ./internal/nodecache ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery`
