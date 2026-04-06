# Implementation Order

## Slice 1. Add Quality Gates First

Before deep refactor, add the following to `verify`:

- `gocyclo <= 15`
- controller LOC check
- thin reconciler heuristic
- controller coverage helper

Reason:

- without gates the repo can regress during the refactor itself.

## Slice 2. Extract `modelpublish` Use Cases

Cut first:

- `images/controller/internal/modelpublish/reconciler.go`

Target split:

- `domain/publication` status projection
- `adapters/k8s/modelpublish/reconciler.go`

Outcome:

- reconciler becomes thin shell;
- status transition logic leaves adapter layer.

## Slice 3. Extract `publicationoperation` Use Cases

Cut next:

- `images/controller/internal/publicationoperation/reconciler.go`
- `images/controller/internal/publicationoperation/contract.go`

Target split:

- `application/publication/start_publication.go`
- `domain/publication` runtime decisions
- `ports/operation_store.go`
- `ports/worker_runtime.go`
- `adapters/k8s/publicationoperation/*`

Outcome:

- worker/session interpretation becomes use-case code;
- ConfigMap/Pod IO stays adapter-only.

## Slice 4. Split `uploadsession`

Completed bounded cut:

- `application/publication/issue_upload_session.go`
- `images/controller/internal/uploadsession/request.go`
- `images/controller/internal/uploadsession/service.go`
- `images/controller/internal/uploadsession/resources.go`
- `images/controller/internal/uploadsession/pod.go`
- `images/controller/internal/uploadsession/status.go`
- `images/controller/internal/uploadsession/names.go`
- `images/controller/internal/uploadsession/options.go`

Deferred explicitly:

- separate `observe_upload_session.go` cut
- `ports/token_issuer.go`
- package move to `adapters/k8s/uploadsession/*`

Outcome:

- `uploadsession/session.go` removed completely
- upload-session validation now goes through a pure application use case
- K8s CRUD shell, object builders, naming helpers and session rehydration are no
  longer mixed in one 600+ LOC file
- `uploadsession` removed from LOC / complexity allowlists

## Slice 5. Refactor `modelcleanup`

Completed bounded cut:

- `images/controller/internal/application/deletion/ensure_cleanup_finalizer.go`
- `images/controller/internal/application/deletion/finalize_delete.go`
- `images/controller/internal/modelcleanup/options.go`
- `images/controller/internal/modelcleanup/observation.go`
- `images/controller/internal/modelcleanup/persistence.go`
- `images/controller/internal/modelcleanup/reconciler.go`

Outcome:

- finalizer guard logic and delete decision policy moved out of adapter code
- `modelcleanup/reconciler.go` is now a thin shell over application decisions
- delete status persistence and cleanup-job observation no longer live in one
  file with reconcile branching
- `application/deletion` now has its own branch matrix and coverage gate

Deferred explicitly:

- separate ports for cleanup runtime/store
- additional bounded cut for `publicationoperation/contract.go`
- additional bounded cut for `sourcepublishpod/pod.go`

## Slice 6. Split `publicationoperation/contract`

Completed bounded cut:

- `images/controller/internal/publicationoperation/types.go`
- `images/controller/internal/publicationoperation/constants.go`
- `images/controller/internal/publicationoperation/configmap_codec.go`
- `images/controller/internal/publicationoperation/configmap_mutation.go`

Outcome:

- former `publicationoperation/contract.go` removed completely
- operation types/validation no longer live in the same file as ConfigMap codec
  and mutation helpers
- ConfigMap protocol remains unchanged, but store helpers are explicit and
  separately testable
- `publicationoperation` removed from LOC allowlist after passing verify

Deferred explicitly:

- dedicated store port abstraction for operation persistence
- bounded cut for `sourcepublishpod/pod.go`
- extra wiring tests for `publicationoperation/upload.go` expiry/terminal replay

## Slice 7. Split `sourcepublishpod`

Completed bounded cut:

- `images/controller/internal/application/publication/plan_source_worker.go`
- `images/controller/internal/sourcepublishpod/types.go`
- `images/controller/internal/sourcepublishpod/validation.go`
- `images/controller/internal/sourcepublishpod/names.go`
- `images/controller/internal/sourcepublishpod/build.go`

Outcome:

- former `images/controller/internal/sourcepublishpod/pod.go` removed completely
- source-specific worker acceptance moved into
  `application/publication/plan_source_worker.go`
- source publish pod contract, structural validation, naming and Pod assembly no
  longer live in one adapter file
- `(Request).Validate` complexity is reduced below the repo threshold and
  removed from the temporary complexity allowlist
- direct negative tests now pin invalid owner, invalid identity, unsupported
  source kinds and not-yet-implemented auth/session branches, while
  application-level branch tests pin HF/HTTP/upload source-worker planning

Deferred explicitly:

- dedicated branch-matrix artifact for `sourcepublishpod`
- further cut of service-layer persistence/runtime ports
- replay/wiring tests beyond current `Build` and `Service` surface

## Slice 8. Harden `publicationoperation` Store/Codec Replay

Completed bounded cut:

- `images/controller/internal/publicationoperation/types.go`
- `images/controller/internal/publicationoperation/configmap_codec.go`
- `images/controller/internal/publicationoperation/status.go`
- `images/controller/internal/publicationoperation/reconciler.go`

Outcome:

- persisted operation protocol now fails closed on unsupported source type,
  semantically invalid upload payload and `Succeeded` phase without a valid
  persisted `result`
- terminal/corrupt persisted state is validated before reconcile no-op, instead
  of silently bypassing downstream status projection
- replay evidence now covers `AwaitingResult -> Succeeded`, running upload with
  malformed persisted upload payload, identical upload-status no-op replay, and
  expired upload terminal replay
- repo-level `make verify` is green after this slice

Deferred explicitly:

- dedicated store port abstraction for operation persistence
- broader modelpublish-side projection strategy for already-corrupted terminal
  operation state before publicationoperation reconcile runs
- broader replay matrix for worker/service recreation races

## Slice 9. Extract `publicationoperation` Store and Runtime Ports

Completed bounded cut:

- `images/controller/internal/publicationoperation/ports.go`
- `images/controller/internal/publicationoperation/store.go`
- `images/controller/internal/publicationoperation/runtime.go`
- `images/controller/internal/publicationoperation/source.go`
- `images/controller/internal/publicationoperation/upload.go`
- `images/controller/internal/publicationoperation/reconciler.go`

Outcome:

- `publicationoperation` no longer constructs `ConfigMap` persistence or
  source/upload runtime services inline inside reconcile helpers
- explicit `operationStore`, `sourceWorkerRuntime` and `uploadSessionRuntime`
  seams now separate use-case orchestration from concrete K8s CRUD adapters
- source/upload reconcile paths now depend on narrow worker/session handles
  instead of leaking concrete `Pod` / `Session` types through the adapter logic
- new adapter-level tests pin concrete store cleanup semantics and
  source/upload runtime create-delete behavior, while application and reconciler
  tests continue to own lifecycle policy

Deferred explicitly:

- package move from local adapter ports to shared `ports/*`
- separate codec/store package outside `publicationoperation`
- broader runtime replay coverage for worker/session recreation races

## Slice 10. Introduce `domain/publication`

Completed bounded cut:

- `images/controller/internal/domain/publication/operation.go`
- `images/controller/internal/domain/publication/status.go`
- `images/controller/internal/domain/publication/runtime_decisions.go`

## Slice 19. Normalize Controller Test Architecture

Completed bounded cut:

- `images/controller/internal/support/testkit/testkit.go`
- `images/controller/internal/controllers/catalogstatus/test_helpers_test.go`
- `images/controller/internal/controllers/catalogcleanup/test_helpers_test.go`
- `images/controller/internal/controllers/catalogstatus/reconciler_test.go`
- `images/controller/internal/controllers/catalogcleanup/reconciler_test.go`
- `images/controller/internal/controllers/publicationops/reconciler_test.go`

Outcome:

- controller tests no longer bootstrap scheme/model/fake-client fixtures in
  three different ways
- shared scheme/object/fake-client fixtures now live under
  `internal/support/testkit`
- package-local `test_helpers_test.go` is now the explicit place for
  adapter-local options, resource builders and assertions
- a real duplicate cleanup-controller test was removed instead of being kept
  under another name
- review bundle and repo-memory now pin this test taxonomy so future slices do
  not recreate fixture sprawl

## Slice 20. Add Explicit Controller Structure Inventory

Completed bounded cut:

- `images/controller/STRUCTURE.ru.md`
- `images/controller/README.md`
- `.agents/skills/controller-architecture-discipline/references/controller-discipline.md`
- `.agents/skills/controller-runtime-implementation/SKILL.md`

Outcome:

- controller tree now has a repo-local file/folder inventory with explicit
  rationale for each remaining package and file
- package-map discussions no longer depend only on chat memory or active
  bundle history
- future files in `images/controller` now have to fit the same
  `purpose / why here / why not elsewhere` discipline

## Slice 21. Remove Remaining Micro-Splits In Controller Adapters

Completed bounded cut:

- `images/controller/internal/controllers/catalogstatus/io.go`
- `images/controller/internal/controllers/catalogcleanup/io.go`
- `images/controller/internal/controllers/publicationops/configmap_protocol.go`
- removed:
  - `catalogstatus/observation.go`
  - `catalogstatus/persistence.go`
  - `catalogcleanup/observation.go`
  - `catalogcleanup/persistence.go`
  - `publicationops/constants.go`
  - `publicationops/configmap_codec.go`
  - `publicationops/configmap_mutation.go`
  - `publicationops/status.go`

Outcome:

- `catalogstatus` now keeps one honest adapter IO file instead of a fake split
  between observation and persistence
- `catalogcleanup` now keeps one honest adapter IO file instead of the same
  duplicated micro-split
- `publicationops` now treats persisted `ConfigMap` protocol as one concrete
  protocol boundary instead of scattering it across four tiny production files
- `STRUCTURE.ru.md` now reflects the compacted tree rather than the earlier
  over-fragmented map

## Slice 22. Split `publicationops` Reconcile Tests By Decision Family

Completed bounded cut:

- `images/controller/internal/controllers/publicationops/worker_result.go`
- `images/controller/internal/controllers/publicationops/test_helpers_test.go`
- `images/controller/internal/controllers/publicationops/reconcile_core_test.go`
- `images/controller/internal/controllers/publicationops/reconcile_source_worker_test.go`
- `images/controller/internal/controllers/publicationops/reconcile_upload_session_test.go`
- removed:
  - `publicationops/reconciler_test.go`

Outcome:

- shared worker-result decode and snapshot mapping no longer duplicates between
  `source.go` and `upload.go`
- `publicationops` adapter tests are now grouped by lifecycle decision families
  instead of one 1k+ `reconciler_test.go`
- shared bootstrap moved into `test_helpers_test.go`, while scenario assertions
  stay in explicit decision-family files
- `STRUCTURE.ru.md`, test strategy and repo-memory now describe this split so
  future slices do not recreate a monolithic controller test file

## Slice 23. Collapse `publicationops` Persisted Protocol Tests Into One Boundary

Completed bounded cut:

- `images/controller/internal/controllers/publicationops/configmap_protocol.go`
- `images/controller/internal/controllers/publicationops/configmap_protocol_test.go`
- removed:
  - `publicationops/configmap_mutation_test.go`
  - `publicationops/status_test.go`

Outcome:

- persisted protocol helpers now share one small internal API for required vs
  optional payload lookup and JSON persistence, without adding a new adapter
  package or resurrecting protocol micro-splits
- `publicationops` now tests one concrete `ConfigMap` boundary in one honest
  `configmap_protocol_test.go` file instead of scattering mutation/status
  coverage across separate files
- `STRUCTURE.ru.md` now explains this as one concrete seam rather than three
  adjacent test files around the same boundary

## Slice 24. Move Shared Runtime Port Implementations Into K8s Adapters

Completed bounded cut:

- `images/controller/internal/adapters/k8s/sourceworker/runtime.go`
- `images/controller/internal/adapters/k8s/sourceworker/runtime_test.go`
- `images/controller/internal/adapters/k8s/uploadsession/runtime.go`
- `images/controller/internal/adapters/k8s/uploadsession/runtime_test.go`
- `images/controller/internal/controllers/publicationops/reconciler.go`
- `images/controller/internal/controllers/publicationops/options.go`
- removed:
  - `publicationops/runtime.go`
  - `publicationops/runtime_test.go`

Outcome:

- shared `publication` runtime ports are now implemented by the concrete
  `sourceworker` and `uploadsession` adapters themselves
- `publicationops` no longer owns concrete `OperationContext -> adapter request`
  wiring and service-to-handle adaptation
- controller package map is more hexagonal: ports stay shared, adapters
  implement them, controllers only orchestrate
- `STRUCTURE.ru.md` and runtime README now describe this ownership explicitly
- `images/controller/internal/domain/publication/conditions.go`
- `images/controller/internal/domain/publication/operation_test.go`
- `images/controller/internal/domain/publication/status_test.go`
- `images/controller/internal/domain/publication/runtime_decisions_test.go`
- `images/controller/internal/domain/publication/BRANCH_MATRIX.ru.md`
- `images/controller/internal/application/publication/start_publication.go`

Outcome:

- first real `domain/publication` package now owns publication terminal phase
  semantics, runtime decision tables and model status/condition projection rules
- status projection moved behind the new domain seam instead of living in a
  dedicated application facade file
- runtime decision tables moved behind the new domain seam instead of living in
  a dedicated application facade file
- `AcceptedStatus` and `IsTerminalOperationPhase` now depend on the new domain
  seam instead of package-private projection helpers
- branch-matrix and coverage evidence for status projection now lives at the
  domain layer where the lifecycle rules actually belong

Deferred explicitly:

- introduce shared `ports/*` outside adapter-local packages
- split `publicationoperation` ConfigMap protocol into a dedicated adapter/store
  package

## Slice 11. Clean Up Legacy Patchwork

Completed bounded cut:

- remove repo junk directory `.VSCodeCounter`
- rename stale test filenames after refactor:
  - `publicationoperation/contract_test.go` -> `publicationoperation/configmap_protocol_test.go`
  - `sourcepublishpod/pod_test.go` -> `sourcepublishpod/build_test.go`
  - `uploadsession/session_test.go` -> `uploadsession/service_roundtrip_test.go`

## Slice 12. Extract Shared `ports/*`

Completed bounded cut:

- `images/controller/internal/ports/publication/ports.go`
- `images/controller/internal/publicationoperation/runtime.go`
- `images/controller/internal/publicationoperation/options.go`
- `images/controller/internal/publicationoperation/source.go`
- `images/controller/internal/publicationoperation/upload.go`
- `images/controller/internal/publicationoperation/runtime_test.go`

Outcome:

- `OperationStore`, `SourceWorkerRuntime`, `UploadSessionRuntime` and
  worker/session handles now live under `internal/ports/publication`
- concrete `ConfigMap` store and Pod/Session runtimes remain adapter-local, but
  they now implement shared ports instead of package-private interfaces

Deferred explicitly:

- complete operation-contract extraction into shared ports
- split `publicationoperation` ConfigMap protocol into a dedicated adapter/store
  package outside `publicationoperation`

## Slice 13. Complete Shared `ports/publication` Contract Extraction

Completed bounded cut:

- `images/controller/internal/ports/publication/operation_contract.go`
- `images/controller/internal/ports/publication/operation_contract_test.go`
- `images/controller/internal/ports/publication/ports.go`
- `images/controller/internal/publicationoperation/types.go`
- `images/controller/internal/publicationoperation/test_helpers_test.go`

Outcome:

- shared `internal/ports/publication` now owns publication operation contract
  primitives too:
  - `Phase`
  - `Owner`
  - `Request`
  - `Result`
  - `Status`
- `ConfigMapNameFor` was initially moved there with the contract but later
  relocated to `internal/support/resourcenames` once the refactor proved that
  resource naming is support policy, not a port primitive
- stale duplicate `internal/domain/publicationoperation/*` removed completely;
  it no longer pollutes controller coverage gates or acts as a shadow source of
  truth
- operation-contract validation evidence moved next to the shared seam in
  `internal/ports/publication/operation_contract_test.go`

Deferred explicitly:

- remove the temporary `publicationoperation/types.go` compatibility shim once
  downstream consumers switch to direct shared-port imports
- split `publicationoperation` ConfigMap protocol into a dedicated adapter/store
  package outside `publicationoperation`

## Slice 14. Remove Compatibility Shims And Duplicate Tests

Completed bounded cut:

- removed no-signal alias layers:
  - `images/controller/internal/application/materialization/types.go`
  - `images/controller/internal/modelpackinit/types.go`
  - `images/controller/internal/ports/materialization/*`
- removed duplicate controller tests that repeated already-covered paths:
  - `images/controller/internal/publicationoperation/configmap_protocol_test.go`
  - `images/controller/internal/uploadsession/service_roundtrip_test.go`
- kept release-path packages intact:
  - `publicationoperation/*`
  - `sourcepublishpod/*`
  - `uploadsession/*`
  - `modelpublish/*`
  - `modelcleanup/*`

Outcome:

- `materialization` and `modelpackinit` now depend directly on the real domain
  contract instead of carrying extra alias files
- dead speculative `internal/ports/materialization` is gone completely
- duplicated controller tests no longer inflate size without adding new branch
  or replay evidence

## Slice 15. Import GPU-Control-Plane Verification Patterns And Continue Reduction

Completed bounded cut:

- added bounded controller coverage artifact collection under
  `artifacts/coverage`
- added `deadcode` install/check shell and wired it into `make verify`
- removed dead helpers and one-off wrappers confirmed by deadcode/import graph
- deleted duplicate phase-mapping helpers and low-signal acceptance proxy code

Outcome:

- the repo now has objective controller reduction tooling, not just manual
  review
- deadcode and coverage artifacts became part of the normal verify shell
- controller tree shrank again without changing release-path behavior

## Slice 16. Remove Detached Runtime-Materialization Graph

Completed bounded cut:

- removed the entire speculative runtime graph:
  - `internal/application/materialization/*`
  - `internal/domain/materialization/*`
  - `internal/modelpackinit/*`
- archived the superseded active runtime-materializer bundle

Outcome:

- the live tree no longer claims a runtime implementation that is not connected
  to any consumer path
- controller README and active cleanup bundle now match the code that actually
  exists

## Slice 17. Remove Publication-Operation Compatibility Shim

Completed bounded cut:

- `images/controller/internal/modelpublish/observation.go`
- `images/controller/internal/modelpublish/persistence.go`
- `images/controller/internal/modelpublish/reconciler.go`
- `images/controller/internal/modelpublish/reconciler_test.go`
- `images/controller/internal/publicationoperation/configmap_codec.go`
- `images/controller/internal/publicationoperation/configmap_mutation.go`
- `images/controller/internal/publicationoperation/source.go`
- `images/controller/internal/publicationoperation/store.go`
- `images/controller/internal/publicationoperation/test_helpers_test.go`
- `images/controller/internal/publicationoperation/upload.go`
- deleted `images/controller/internal/publicationoperation/types.go`

Outcome:

- downstream consumer `modelpublish` now depends directly on
  `internal/ports/publication`, not on `publicationoperation` aliases
- production compatibility shim `publicationoperation/types.go` is gone
  completely; only test-local aliases remain in
  `publicationoperation/test_helpers_test.go`
- publication ConfigMap codec and mutation helpers now speak the shared
  operation contract directly

Deferred explicitly:

- split `publicationoperation` ConfigMap protocol into a dedicated adapter/store
  package outside `publicationoperation`
- broader replay coverage for worker/session recreation races

## Slice 18. Rewrite Concrete Package Map And Shared Support Seams

Completed bounded cut:

- moved concrete reconciler packages under `internal/controllers/*`:
  - `catalogstatus`
  - `publicationops`
  - `catalogcleanup`
- moved concrete K8s worker/session/job adapters under
  `internal/adapters/k8s/*`:
  - `sourceworker`
  - `uploadsession`
  - `cleanupjob`
- moved cleanup-handle persistence under `internal/support/cleanuphandle`
- introduced shared support packages:
  - `internal/support/modelobject`
  - `internal/support/resourcenames`
  - `internal/adapters/k8s/ociregistry`

Outcome:

- controller tree is no longer a flat top-level patchwork of reconcilers,
  resource builders and helpers at the same directory depth
- duplicated `Model` / `ClusterModel` request/status/kind helpers were removed
  from controller packages and moved into a single shared support seam
- duplicated owner-based naming, label normalization and OCI registry
  env/volume rendering were removed from `sourceworker`, `uploadsession` and
  `cleanupjob`
- controller README, target layout and durable repo-memory now describe the
  real current package map instead of the old flat one

Deferred explicitly:

- split `controllers/publicationops` ConfigMap protocol into a dedicated
  adapter/store package outside the controller package itself
- further shrink duplicated service/session observation scaffolding between
  `adapters/k8s/sourceworker` and `adapters/k8s/uploadsession`

## Feature Work Resume Point

Only after slices 1-18:

- `HF/HTTP authSecretRef`
- runtime init/materializer consumer wiring
- PVC/cache plane

## Slice 25. Centralize Canonical Resource Naming

Completed bounded cut:

- `internal/support/resourcenames` now owns canonical names for:
  - publication operation `ConfigMap`
  - source-worker Pod/auth Secret
  - upload-session Pod/Service/Secret
  - cleanup Job
- removed package-local naming shims:
  - `internal/adapters/k8s/sourceworker/names.go`
  - `internal/adapters/k8s/uploadsession/names.go`
- switched controller/adapter tests to the shared naming support seam

Outcome:

- owner-based naming policy no longer leaks into `ports/publication` or
  adapter-local `names.go` wrappers
- `operation_contract.go` is narrower again and only owns publication contract
  primitives, not resource-name helpers
- `sourceworker`, `uploadsession`, `cleanupjob`, `catalogstatus` and
  `publicationops` now speak one canonical naming policy through
  `support/resourcenames`
- `STRUCTURE.ru.md`, README and repo-local controller skills stay aligned with
  this boundary

## Slice 26. Remove Adapter-Local Request Mirrors

Completed bounded cut:

- `sourceworker` and `uploadsession` now consume shared
  `publication.OperationContext` directly in runtime/service/build paths
- removed local request mirror types:
  - `internal/adapters/k8s/sourceworker/types.go`
  - `internal/adapters/k8s/uploadsession/types.go`
- kept only concrete adapter planning where it is still justified:
  - `sourceworker/validation.go`
  - `uploadsession/request.go`
- introduced package-local test fixtures:
  - `sourceworker/test_helpers_test.go`
  - `uploadsession/test_helpers_test.go`

Outcome:

- adapters no longer duplicate shared `Request` / `Owner` contract under local
  `Request` / `OwnerRef` wrappers
- source/upload adapter tests now build from one canonical `OperationContext`
  fixture per package instead of retyping the same request literal everywhere

## Slice 27. Collapse Adapter Runtime Proxy Layer

Completed bounded cut:

- deleted:
  - `internal/adapters/k8s/sourceworker/runtime.go`
  - `internal/adapters/k8s/uploadsession/runtime.go`
- moved shared runtime-port implementation directly into:
  - `internal/adapters/k8s/sourceworker/service.go`
  - `internal/adapters/k8s/uploadsession/service.go`
- kept package-local internal CRUD as unexported helper methods:
  - `getPod` / `getOrCreatePod` / `deletePod`
  - `getSession` / `getOrCreateSession` / `deleteSession`

Outcome:

- concrete K8s adapters now own one runtime object instead of `service + runtime`
  proxy chaining
- handle wrapping stays in the adapter package, but no longer needs a second
  file-level layer
- the package keeps one real boundary:
  shared publication port <-> concrete K8s resource CRUD

## Slice 28. Centralize Controlled Resource Create-Or-Get

Completed bounded cut:

- added:
  - `internal/adapters/k8s/ownedresource/create_or_get.go`
  - `internal/adapters/k8s/ownedresource/create_or_get_test.go`
- moved repeated K8s owned create/reuse shell out of:
  - `internal/adapters/k8s/sourceworker/service.go`
  - `internal/adapters/k8s/uploadsession/pod.go`
  - `internal/adapters/k8s/uploadsession/resources.go`

Outcome:

- controller-owned `Pod` / `Service` / `Secret` supplements no longer open-code
  the same `SetControllerReference -> Create -> AlreadyExists -> Get` flow
- concrete adapters keep resource shape/build logic locally, while shared K8s
  object IO shell is now one canonical adapter helper

## Slice 29. Centralize Workload Pod Shell

Completed bounded cut:

- added:
  - `internal/adapters/k8s/workloadpod/render.go`
  - `internal/adapters/k8s/workloadpod/render_test.go`
- moved repeated workspace/registry Pod shell out of:
  - `internal/adapters/k8s/sourceworker/build.go`
  - `internal/adapters/k8s/uploadsession/pod.go`

Outcome:

- worker and upload adapters no longer open-code the same workspace
  `EmptyDir`, `/tmp` mount, and registry CA volume/mount wiring
- `sourceworker` and `uploadsession` keep only command/env/extra-volume
  differences, while shared Pod shell now has one canonical adapter helper

## Slice 30. Remove Fake Shared Store Seam From `publicationops`

Completed bounded cut:

- removed the fake shared `OperationStore` seam from:
  - `internal/ports/publication/ports.go`
- simplified `controllers/publicationops` by:
  - deleting the `store` object layer and wiring persistence directly through
    controller-local helpers on the reconciler
  - collapsing duplicate source/upload decision-apply shell into one shared
    controller helper
  - shrinking `test_helpers_test.go` so it keeps only canonical scenario
    builders and bootstrap helpers instead of a shadow API of aliases and
    overlapping fixtures

Outcome:

- shared `ports/publication` is narrower again and now contains only real
  reusable runtime contracts and handles
- persisted `ConfigMap` protocol stays where it actually belongs today:
  inside `controllers/publicationops`
- `publicationops` lost one whole fake abstraction layer instead of just moving
  it between packages

## Slice 31. Remove Misleading Controller Options/Test Names

Completed bounded cut:

- removed `RequeueAfter` from:
  - `internal/controllers/catalogstatus/options.go`
  - controller app wiring in `cmd/ai-models-controller/run.go`
- moved status polling cadence into:
  - `internal/controllers/catalogstatus/reconciler.go`
- renamed misleading adapter tests:
  - `internal/adapters/k8s/sourceworker/runtime_test.go` ->
    `internal/adapters/k8s/sourceworker/service_roundtrip_test.go`
  - `internal/adapters/k8s/uploadsession/runtime_test.go` ->
    `internal/adapters/k8s/uploadsession/service_roundtrip_test.go`

Outcome:

- `catalogstatus.Options` is narrower and now contains only real controller
  setup input instead of a hidden lifecycle policy knob
- source/upload adapter tests no longer pretend there is a separate runtime
  layer after the earlier `runtime.go` removal; they now describe what they
  actually check: concrete service roundtrip on the shared adapter seam

## Slice 32. Collapse PublicationOps Store/Polling Drift

Completed bounded cut:

- removed `RequeueAfter` from:
  - `internal/controllers/publicationops/options.go`
  - controller app wiring in `cmd/ai-models-controller/run.go`
- moved source-worker result polling cadence into:
  - `internal/controllers/publicationops/source.go`
- deleted the extra persistence sublayer:
  - `internal/controllers/publicationops/store.go`
  - `internal/controllers/publicationops/store_test.go`
- folded terminal/running persistence directly into:
  - `internal/controllers/publicationops/reconciler.go`
  - `internal/controllers/publicationops/reconcile_core_test.go`

Outcome:

- `publicationops.Options` now contains only real setup input, not hidden
  lifecycle policy
- `publicationops` no longer pretends to have a separate store layer beside
  the one actual persisted protocol boundary in `configmap_protocol.go`

## Slice 33. Remove Fake Cleanup Adapter Boundary

Completed bounded cut:

- moved cleanup Job build/inspect shell into:
  - `internal/controllers/catalogcleanup/job.go`
  - `internal/controllers/catalogcleanup/job_test.go`
- deleted:
  - `internal/adapters/k8s/cleanupjob/job.go`
  - `internal/adapters/k8s/cleanupjob/job_test.go`
- rewired:
  - `internal/controllers/catalogcleanup/options.go`
  - `internal/controllers/catalogcleanup/io.go`
  - `internal/controllers/catalogcleanup/test_helpers_test.go`
  - `internal/bootstrap/bootstrap_test.go`
  - `cmd/ai-models-controller/run.go`

Outcome:

- cleanup Job is now owned where it is actually used: inside `catalogcleanup`
- `internal/adapters/k8s` lost one fake package that had no second adapter and
  only added another hop around one controller-local delete detail

## Slice 34. Reduce PublicationOps Protocol Seam

Completed bounded cut:

- compacted `internal/controllers/publicationops/configmap_protocol.go`
  around one local protocol-helper shell instead of repeated hand-written
  decode/marshal boilerplate
- kept that helper shell file-local; no new shared utility package or extra
  protocol sublayer was introduced
- rewrote `internal/controllers/publicationops/configmap_protocol_test.go`
  from scattered file-oriented checks into protocol families:
  required decoders, accessors/defaults, upload payload handling, persisted
  status validation, and mutation helpers

Outcome:

- `configmap_protocol.go` remains one honest seam and now stays under the
  controller LOC gate
- `configmap_protocol_test.go` shrank materially while preserving the same
  fail-closed protocol coverage shape

## Slice 35. Remove Ambiguous Bootstrap Naming And Make Controller Deadcode Explicit

Completed bounded cut:

- renamed the composition root package from `internal/app` to
  `internal/bootstrap`
- renamed files to:
  - `internal/bootstrap/bootstrap.go`
  - `internal/bootstrap/bootstrap_test.go`
- rewired:
  - `cmd/ai-models-controller/run.go`
  - controller README / STRUCTURE inventory / bundle notes
- rewired deadcode verification so controller checks are first-class:
  - `make deadcode-controller`
  - `make deadcode-hooks`
  - `make deadcode`
  - `tools/check-controller-deadcode.sh`

Outcome:

- the tree no longer keeps the ambiguous `app` / `application` naming pair
- controller deadcode is now explicit and controller-first in the verify shell
- review guidance now treats ambiguous naming and misleading verification
  output as real findings, not cosmetic issues

## Slice 36. Remove Package-Local Branch Matrix Patchwork

Completed bounded cut:

- deleted package-local evidence files:
  - `internal/domain/publication/BRANCH_MATRIX.ru.md`
  - `internal/application/publication/BRANCH_MATRIX.ru.md`
  - `internal/application/deletion/BRANCH_MATRIX.ru.md`
- introduced one canonical controller-level evidence inventory:
  - `images/controller/TEST_EVIDENCE.ru.md`
- replaced the verify gate:
  - `tools/check-controller-branch-matrix.sh`
  - `check-controller-branch-matrix`
  with:
  - `tools/check-controller-test-evidence.sh`
  - `check-controller-test-evidence`
- rewired controller docs and repo-memory so local `BRANCH_MATRIX` files are no
  longer treated as the expected pattern

Outcome:

- controller test evidence is now one explicit source of truth instead of a
  half-adopted file-per-package convention
- verify still enforces evidence coverage for every non-test
  `internal/domain/*` and `internal/application/*` package
- the tree is simpler because package folders no longer carry optional local
  markdown policy files

## Slice 37. Replace Generic Publication Package Names With Role-Based Names

Completed bounded cut:

- renamed:
  - `internal/application/publication` -> `internal/application/publishplan`
  - `internal/domain/publication` -> `internal/domain/publishstate`
  - `internal/ports/publication` -> `internal/ports/publishop`
  - `internal/publication` -> `internal/publishedsnapshot`
- updated package names and imports across controller code to match the new
  role-based structure
- synced:
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - repo-local skills and discipline docs

Outcome:

- the tree no longer repeats one vague `publication` name across four
  unrelated responsibilities
- naming is closer to virtualization-style ownership: package names now encode
  what the layer actually owns, not just the broad feature area

## Slice 38. Rename The Concrete Publication Controller To `publishrunner`

Completed bounded cut:

- renamed:
  - `internal/controllers/publicationops` -> `internal/controllers/publishrunner`
- rewired:
  - `internal/bootstrap/bootstrap.go`
  - `internal/bootstrap/bootstrap_test.go`
  - `cmd/ai-models-controller/run.go`
  - `internal/controllers/catalogstatus/io.go`
  - `internal/controllers/catalogstatus/test_helpers_test.go`
  - `internal/controllers/catalogstatus/reconciler_test.go`
- synced:
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - current bundle docs

Outcome:

- the last live generic `publicationops` package name is gone from the
  controller tree
- the concrete controller boundary now uses a role-based name that matches its
  responsibility: durable publication run orchestration
- old `publicationops` references remain only as historical notes in earlier
  slice logs

## Slice 39. Refresh Full File-Level Structure Rationale And Reviewer Baseline

Completed bounded cut:

- expanded `images/controller/STRUCTURE.ru.md` so remaining test groups are
  documented file-by-file instead of grouped as vague package blobs
- tightened file-level rationale for `publishrunner`, `sourceworker`,
  `uploadsession`, controller tests, and domain/application evidence files
- prepared the controller tree for another strict read-only reviewer pass by
  making the current live structure defensible file-by-file, not only
  package-by-package

Outcome:

- `STRUCTURE.ru.md` now describes every live controller file in the tree,
  including test files and why they still exist
- remaining reviewers can now flag unjustified files as concrete findings
  instead of fighting stale or underspecified structure docs

## Slice 40. Reduce Protocol And Adapter Constructor/Deletion Duplication

Completed bounded cut:

- reduced `internal/controllers/publishrunner/configmap_protocol.go` by moving
  reconcile-only helpers (`validatePersistedStatus`, `operationStatusView`)
  into the reconcile boundary where they are actually used
- removed adapter-local `NewRuntime` proxy constructors from:
  - `internal/adapters/k8s/sourceworker/service.go`
  - `internal/adapters/k8s/uploadsession/service.go`
- extended `internal/adapters/k8s/ownedresource` from create/reuse-only helper
  into one honest controlled-resource lifecycle helper with shared
  ignore-not-found delete
- renamed:
  - `internal/adapters/k8s/ownedresource/create_or_get.go`
    -> `internal/adapters/k8s/ownedresource/lifecycle.go`
  - `internal/adapters/k8s/ownedresource/create_or_get_test.go`
    -> `internal/adapters/k8s/ownedresource/lifecycle_test.go`
- rewired roundtrip tests and publishrunner bootstrap to instantiate concrete
  services directly

Outcome:

- `publishrunner/configmap_protocol.go` is now materially smaller while keeping
  one bounded persisted-protocol seam
- source/upload adapters no longer keep a second constructor path beside the
  real `Service`
- create/reuse and delete shells now live together under one correctly named
  K8s lifecycle helper instead of open-coded cleanup branches

## Slice 41. Collapse Runtime Adapter Surface To One `GetOrCreate` Contract

Completed bounded cut:

- removed the extra source-worker runtime method from
  `internal/ports/publishop/ports.go`; both runtime adapters now expose one
  `GetOrCreate` contract
- simplified `internal/controllers/publishrunner/source.go` to one source
  worker entrypoint around `GetOrCreate`, removing the extra create-vs-observe
  wrapper split
- removed private test-only seams from:
  - `internal/adapters/k8s/sourceworker/service.go`
  - `internal/adapters/k8s/uploadsession/service.go`
  by switching tests to the public adapter contract instead of internal
  `getOrCreate*` helpers
- dropped the unused public `Get` method from upload-session service and the
  asymmetry-driving `Get` runtime method from source-worker service

Outcome:

- source/upload adapters now expose a simpler and more symmetric runtime
  contract
- `publishrunner/source.go` no longer needs a separate create wrapper just to
  compensate for adapter surface asymmetry
- tests now exercise the same public adapter contract that production code uses

## Rule for every implementation slice

- one bounded cut at a time
- narrow checks after each cut
- no new feature work in touched package until the cut is complete
