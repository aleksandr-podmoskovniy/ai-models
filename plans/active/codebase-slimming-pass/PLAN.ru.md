# Plan: codebase slimming and boundary pass

## 1. Current phase

Задача относится к Phase 2/3 boundary:

- Phase 2 требует понятного runtime distribution topology и node-local delivery;
- Phase 3 требует, чтобы кодовая база не росла через legacy compatibility
  layers и patchwork helpers.

## 2. Orchestration

Режим: `full`.

Причина: задача меняет архитектурные границы и потенциально затрагивает
runtime/storage/HA/API surfaces.

Read-only subagents:

- `repo_architect` — package boundaries, anti-monolith, deletion candidates;
- `integration_architect` — runtime/storage/HA and virtualization-style module
  patterns;
- `api_designer` — public API/status/RBAC noise and safe API-facing cuts.
- continuation `repo_architect` — safe next internal slices after the first
  cut;
- continuation `integration_architect` — runtime/storage/HA slice ordering;
- continuation `backend_integrator` — DMCR/modelpack boundaries and
  direct-upload anti-coupling.

Subagent conclusions:

- `repo_architect`: safest internal cuts are owner-local `workloaddelivery`
  boilerplate collapse and package-local adapter helper collapse. Larger
  `sourcefetch`/`publishworker` and `modelpack/oci` handoff refactors are
  valuable but risky.
- `api_designer`: `ClusterModel.spec.source.authSecretRef` is public schema
  noise because current behavior already rejects it; final slice keeps it only
  as an explicit CEL-denied field so the intentional deny path is not replaced
  by apiserver pruning behavior.
- `integration_architect`: biggest virtualization-style drift is component
  coupling: upload gateway inside controller Deployment, DMCR GC inside serving
  Deployment, controller-owned node-cache/storage substrate and broad internal
  RBAC. These need separate runtime/template/RBAC slices, not incidental edits.
- continuation `repo_architect`: broad `DMCR` / `modelpack/oci` cuts were not
  safe while the repo was dirty across metadata and slimming workstreams;
  bounded `modeldelivery` render collapse and `sourceworker` package-local
  cleanup were selected and later completed in this bundle.
- continuation `backend_integrator`: do not deduplicate controller and DMCR
  direct-upload helpers across image boundaries. Safe backend candidates must
  stay package-local: object-source tar entry shaping, GC policy normalization,
  request classification or direct-upload completion cleanup extraction.
- continuation `integration_architect`: `modeldelivery` render collapse was the
  best bounded runtime slice at that point and is now completed;
  `nodecacheruntime` ownership/RBAC/substrate work still requires a separate
  integration/RBAC/template task.
- continuation `integration_architect` for DMCR lease: do not merge GC executor
  lease and maintenance gate state machines. Safe consolidation is only a
  DMCR-local policy-free helper for holder/reference/pointer/duration
  primitives, with explicit tests for takeover and missing lease timestamps.
- continuation `backend_integrator` / `integration_architect` for DMCR request
  normalization: centralize request classification only; keep
  `phaseQueued`/`phaseArmed` as UX markers, keep gate/quorum/release outside
  generic helpers, and keep direct-upload token-derived cleanup policy
  package-local.
- continuation `backend_integrator` for DMCR direct-upload completion: keep
  cleanup/finalization package-local; do not deduplicate controller and DMCR
  upload helpers across image boundaries; preserve current key validation and
  verification precedence.
- continuation `integration_architect` for DMCR direct-upload completion:
  transient or ambiguous storage errors after successful verification must not
  delete the only sealed byte path. Link write must be retry-safe; deleting a
  duplicate upload object is allowed only after repository link write succeeds.

## 3. Active bundle disposition

- `model-metadata-contract` — keep. Содержит следующий executable slice:
  internal `profilecalc`; текущий code slimming pass не должен менять этот
  public metadata contract случайно.
- `publication-runtime-chaos-resilience` — archived to
  `plans/archive/2026/publication-runtime-chaos-resilience`. The bundle is a
  post-implementation/live-rollout record and currently has only external
  rollout blockers, not an executable in-repo slice.
- `codebase-slimming-pass` — current. Compact workstream для планомерного
  сокращения live code. Current completed slice: `DMCR GC inventory/helper
  normalization`. Next executable slice: `ModelPack OCI direct-upload session
  runner slimming`; no controller/DMCR direct-upload cross-image helper
  deduplication.

Working-tree note:

- repo already contains staged/unstaged `model-metadata-contract` changes;
  this bundle claims only the bounded slimming/API-hardening edits listed in
  the slices below and must not be used as rollback evidence for that older
  metadata workstream.

## 4. Baseline metrics

Live Go LOC snapshot without archives/cache/render artifacts:

```text
  8842 images/controller/internal/adapters/modelpack/oci
  6013 images/dmcr/internal/garbagecollection
  4434 images/controller/internal/adapters/sourcefetch
  4095 images/controller/internal/adapters/k8s/modeldelivery
  3802 images/controller/internal/dataplane/publishworker
  3332 images/controller/internal/controllers/workloaddelivery
  2979 images/controller/internal/adapters/k8s/sourceworker
  2712 images/dmcr/internal/directupload
  2500 images/controller/internal/nodecache
  2281 images/controller/internal/dataplane/uploadsession
```

## 5. Completed slice history

### Completed slice 1. Cut obvious source-admission residue

Цель:

- убрать мелкие leftover wrappers после переноса source parsing в domain;
- не менять API/templates/runtime entrypoints;
- получить быстрый проверяемый reduction и подготовить следующий крупный
  slice.

Файлы:

- `images/controller/internal/application/sourceadmission/`
- `images/controller/internal/adapters/k8s/sourceworker/preflight.go`
- `images/controller/internal/adapters/k8s/sourceworker/service_get_or_create.go`
- sourceworker validation tests and live controller docs.

Проверки:

- `cd images/controller && go test ./internal/domain/modelsource ./internal/adapters/k8s/sourceworker`
- `cd images/controller && go test -count=3 ./internal/domain/modelsource ./internal/adapters/k8s/sourceworker`
- `git diff --check`

### Completed slice 2. Workload delivery owner-local boilerplate collapse

Цель:

- сохранить public annotation contract, включая legacy single-model
  annotations, потому что они всё ещё documented live behavior;
- схлопнуть четыре повторяющиеся controller/list/map/reconcile ветки
  `Deployment` / `StatefulSet` / `DaemonSet` / `CronJob` в package-local
  `workloadKind` table;
- не выносить это в новый package и не менять controller names.

Файлы:

- `images/controller/internal/controllers/workloaddelivery/`

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`
- `go test -count=3` / `go test -race` for touched controller package.

### Completed slice 3. Sourcefetch archive inspection duplicate collapse

Цель:

- схлопнуть повторяющиеся tar/zip archive inspection preparation paths:
  root-prefix, normalized file list, format resolution, selected files and
  selected-file set;
- оставить sourcefetch concrete adapter, но вынести только reusable primitives
  в уже существующие support/domain boundaries без новых оболочек.

Файлы:

- `images/controller/internal/adapters/sourcefetch/`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch`
- `go test -count=3` / `go test -race` for touched sourcefetch package.

### Completed slice 4. ClusterModel source schema hardening

Цель:

- отделить `ClusterModelSpec` от namespaced `ModelSpec`;
- сохранить явный deny-path для `ClusterModel.spec.source.authSecretRef`,
  потому что cluster-scoped object не должен ссылаться на namespaced Secret;
- сохранить namespaced `Model.spec.source.authSecretRef`;
- сохранить internal runtime contract by converting `ClusterModelSpec` to the
  existing `ModelSpec` handoff at controller boundary.

Файлы:

- `api/core/v1alpha1/`
- `crds/`
- controller call sites that pass `ClusterModel.Spec` into shared runtime
  ports.

Проверки:

- `bash api/scripts/update-codegen.sh`
- `cd api && go test ./...`
- `bash api/scripts/verify-crdgen.sh`
- targeted controller tests.

### Completed slice 5. Modeldelivery render/prune duplicate collapse

Цель:

- схлопнуть duplicated managed-cache remove/prune helpers;
- вынести общие materializer container/env/volume-mount helpers из
  single-model и alias render paths;
- сохранить runtime env, CA volume, image-pull-secret и byte-path contract.

Файлы:

- `images/controller/internal/adapters/k8s/modeldelivery/`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`
- `go test -count=3` / `go test -race` for touched modeldelivery/workloaddelivery packages.

### Completed slice 6. Sourceworker package-local cleanup

Цель:

- убрать мёртвый parameter pass-through из pod-spec/volume shaping;
- не менять owner-generation recreation, projected auth secret, concurrency or
  direct-upload state handoff.

Файлы:

- `images/controller/internal/adapters/k8s/sourceworker/`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/sourceworker`
- `go test -count=3` / `go test -race` for touched sourceworker package.

### Completed slice 7. DMCR lease utility consolidation

Цель:

- вынести только policy-free lease primitives между GC executor lease и
  maintenance gate;
- сохранить текущую zero-rollout maintenance semantics, ack quorum and request
  lifecycle;
- не менять storage byte path, registry writes or cleanup request API.

Файлы:

- `images/dmcr/internal/leaseutil/lease.go`
- `images/dmcr/internal/garbagecollection/executor_lease.go`
- `images/dmcr/internal/maintenance/lease.go`
- `images/dmcr/internal/maintenance/ack.go`
- `images/dmcr/internal/maintenance/file.go`
- related GC/maintenance tests for takeover and missing timestamp behavior.

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection ./internal/maintenance`
- `go test -count=3` / `go test -race` for the same packages.

Evidence:

- `cd images/dmcr && go test ./internal/garbagecollection ./internal/maintenance`
- `cd images/dmcr && go test -count=3 -run 'TestExecutorLeaseCreatesLeaseWhenAbsent|TestExecutorLeaseSkipsWorkWhenAnotherHolderLeaseIsLive|TestExecutorLeaseTakesOverExpiredLease|TestExecutorLeaseRenewsOwnLease|TestExecutorLeaseUsesCreationTimestampWhenLeaseTimesAreMissing|TestRunRequestCycleActivatesAndReleasesMaintenanceGate|TestRunRequestCycleWaitsForMaintenanceGateAckQuorum|TestRunRequestCycleSkipsCleanupWhenMaintenanceGateAckQuorumMissing' ./internal/garbagecollection`
- `cd images/dmcr && go test -count=3 -run 'TestLeaseGateActivateAndRelease|TestFileMirrorTreatsExpiredLeaseAsInactive|TestLeaseGateRefusesActiveGateHeldByOtherIdentity|TestLeaseGateTakesOverExpiredLeaseWithMissingTransitions|TestLeaseGateDoesNotTreatCreationTimestampOnlyForeignLeaseAsActive|TestFileMirrorTreatsLeaseWithOnlyCreationTimestampAsInactive|TestFileMirrorsExposeOneClusterGateToMultiplePods|TestAckMirrorPublishesQuorumOnlyAfterAllRuntimeAcks|TestAckMirrorRejectsStaleSequence|TestAckQuorumIgnoresLeaseWithOnlyCreationTimestamp|TestRegistryWriteGateBlocksMutatingV2Requests|TestRegistryWriteGateAllowsReads' ./internal/maintenance`
- `cd images/dmcr && go test -race ./internal/garbagecollection ./internal/maintenance`
- `git diff --check && git diff --cached --check`

### Completed slice 8. DMCR GC request/policy normalization

Цель:

- схлопнуть package-local request classification/policy duplication inside
  `images/dmcr/internal/garbagecollection`;
- сохранить cleanup request Secret API, queued/armed/done UX and retry
  semantics;
- не менять storage inventory, registry deletion, maintenance gate or ack
  quorum.

Файлы для read-only review перед edit:

- `images/dmcr/internal/garbagecollection/request_policy.go`
- `images/dmcr/internal/garbagecollection/request_selection_test.go`
- `images/dmcr/internal/garbagecollection/request_result.go`
- `images/dmcr/internal/garbagecollection/runner.go`

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- targeted `go test -count=3` for request policy/selection/result tests.

Read-only review conclusions:

- Request lifecycle truth is currently split across runner/result/schedule.
  Safe cut is a package-local classifier that preserves exact precedence:
  non-request ignored, `phase=done` completed, non-done `switch` active,
  non-done no `switch` plus `requested-at` queued.
- `phaseQueued` and `phaseArmed` are UX markers only. Do not make them
  authoritative.
- Keep `shouldActivateGarbageCollection` separate because it is queued-request
  policy, including malformed timestamp fail-open behavior.
- Keep maintenance gate activation, ack quorum, release and cleanup execution
  out of generic request helpers.
- Direct-upload cleanup policy may be split into local stages only:
  collect immediate-mode tokens, decode claims, derive prefix/multipart
  targets, assemble `cleanupPolicy`. Do not move it to shared direct-upload or
  maintenance packages.

Evidence:

- `cd images/dmcr && go test ./internal/garbagecollection ./internal/maintenance`
- `cd images/dmcr && go test -count=3 -run 'TestShouldRunGarbageCollection|TestRequestClassificationPrecedence|TestHasPendingRequestUsesQueuedAndActiveOnly|TestShouldActivateGarbageCollection|TestRunRequestCycleArmsQueuedRequestsAndLogs|TestRunRequestCycleMarksActiveRequestsDoneAndLogs|TestPruneExpiredCompletedRequestsDeletesOnlyExpiredResults|TestCompletedRequestWithMalformedTimestampExpiresFailOpen|TestBoundedResultRegistryOutputTruncatesLargeOutput|TestCleanupPolicyForActiveRequests|TestRunRequestCycleActivatesAndReleasesMaintenanceGate|TestRunRequestCycleWaitsForMaintenanceGateAckQuorum|TestRunRequestCycleSkipsCleanupWhenMaintenanceGateAckQuorumMissing|TestRunRequestCycleBoundsFullActiveCleanupWindow' ./internal/garbagecollection`
- `cd images/dmcr && go test -count=3 -run 'TestDiscoverDirectUploadInventoryTargetsFreshPrefixWhenCleanupPolicyRequestsIt|TestDeletePostGarbageCollectDirectUploadPrefixesDeletesFormerlyProtectedFreshPrefix|TestBuildReportKeepsFreshDirectUploadPrefixAgeBoundedWhenNoLiveOwnersRemain|TestBuildReportKeepsFreshDirectUploadPrefixAgeBoundedWhileOwnerIsDeleting|TestBuildReportKeepsFreshDirectUploadPrefixAgeBoundedWhenDeleteTriggeredPolicyHasNoTarget' ./internal/garbagecollection`
- `cd images/dmcr && go test -race ./internal/garbagecollection ./internal/maintenance`
- `git diff --check && git diff --cached --check`

### Completed slice 9. DMCR mark-completed replay/idempotency proof

Цель:

- добавить точный replay/idempotency proof для случая, когда cleanup уже
  выполнился, но часть `markRequestsCompleted` updates упала;
- не менять request Secret schema и не трогать gate/quorum/cleanup execution.

Файлы:

- `images/dmcr/internal/garbagecollection/request_result_test.go`
- при необходимости только package-local test helper.

Проверки:

- `cd images/dmcr && go test -count=3 -run 'TestMarkRequestsCompleted|TestRunRequestCycleMarksActiveRequestsDoneAndLogs' ./internal/garbagecollection`
- `cd images/dmcr && go test ./internal/garbagecollection`

Evidence:

- `TestMarkRequestsCompletedLeavesFailedUpdatesReplayable` proves that a
  partial post-cleanup completion failure leaves the failed Secret active and
  replayable, while already completed Secrets stay done.
- `cd images/dmcr && go test -count=3 -run 'TestMarkRequestsCompletedLeavesFailedUpdatesReplayable|TestRunRequestCycleMarksActiveRequestsDoneAndLogs' ./internal/garbagecollection`
- `cd images/dmcr && go test ./internal/garbagecollection ./internal/maintenance`
- `cd images/dmcr && go test -race ./internal/garbagecollection ./internal/maintenance`
- `cd images/dmcr && go test ./...`
- `git diff --check && git diff --cached --check`

### Completed slice 10. DMCR direct-upload completion replay-safe finalization

Цель:

- убрать монолитный post-verification storage block из `handleComplete`;
- закрыть HA-дефект, при котором ambiguous `metadata`/`repo-link` storage
  error мог удалить единственный byte path уже sealed upload;
- сохранить token format, public upload API, registry layout, byte path,
  verification policy precedence and controller direct-upload client contract.

Файлы:

- `images/dmcr/internal/directupload/completion.go`
- `images/dmcr/internal/directupload/sealed_storage.go`
- `images/dmcr/internal/directupload/verification.go`
- package-local directupload tests/support helpers.

Поведение:

- transient/ambiguous failure after successful verification keeps uploaded
  object and sealed metadata for retry;
- duplicate upload object is removed only after repository link write
  succeeds;
- same-session retry after metadata written but link failed does not delete the
  object referenced by sealed metadata;
- deterministic bad request/key errors and verification mismatch cleanup stay
  fail-closed as before.

Проверки:

- `cd images/dmcr && go test ./internal/directupload -run 'TestServiceComplete'`
- `cd images/dmcr && go test ./internal/directupload -run 'TestTrustedBackendVerification|TestVerificationReadProgressWriter'`
- `cd images/controller && go test ./internal/adapters/modelpack/oci -run 'Test.*DirectUpload'`
- `cd images/dmcr && go test ./internal/directupload`
- `cd images/dmcr && go test -count=3 ./internal/directupload`
- `cd images/dmcr && go test -race ./internal/directupload`
- `cd images/dmcr && go test ./...`
- `git diff --check && git diff --cached --check`

### Completed slice 11. DMCR direct-upload support/log slimming

Цель:

- убрать повторяющиеся direct-upload test helpers and verification log
  formatting without changing upload protocol, token format, registry layout
  or verification precedence;
- keep all edits inside `images/dmcr/internal/directupload`.

Файлы:

- `images/dmcr/internal/directupload/verification.go`
- `images/dmcr/internal/directupload/service_test_support_test.go`
- `images/dmcr/internal/directupload/service_fake_backend_test.go`
- `images/dmcr/internal/directupload/service_assertions_test.go`

Поведение:

- client-asserted verification logging now has one local formatting helper;
- test support is split into fake backend, HTTP harness and assertions;
- repeated response decoders collapsed into one generic helper.

Проверки:

- `cd images/dmcr && go test ./internal/directupload`
- `cd images/dmcr && go test -count=3 ./internal/directupload`
- `cd images/dmcr && go test -race ./internal/directupload`
- `cd images/dmcr && go test ./...`
- `git diff --check && git diff --cached --check`

### Completed slice 12. ModelPack OCI object-source tar-entry shaping

Цель:

- схлопнуть duplicated tar path/header shaping между full-stream and ranged
  object-source publish paths;
- preserve exact archive path construction, direct-upload protocol, layer media
  types, digest calculation and compressed fallback behavior;
- keep helpers package-local and do not share controller helpers with DMCR
  server-side direct-upload code.

Файлы:

- `images/controller/internal/adapters/modelpack/oci/publish_object_source.go`
- `images/controller/internal/adapters/modelpack/oci/publish_object_source_range.go`
- `images/controller/internal/adapters/modelpack/oci/layer_matrix_object_source_test.go`

Поведение:

- generated archive and ranged archive now use the same package-local tar
  header/path helper;
- uncompressed tar object-source direct-upload can resume via ranged object
  reads after an interrupted upload;
- compressed tar object-source keeps the generated-archive fallback.

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `cd images/controller && go test -count=3 -run 'Test.*ObjectSource|Test.*Range|Test.*DirectUpload' ./internal/adapters/modelpack/oci`
- `cd images/controller && go test -race ./internal/adapters/modelpack/oci`
- `git diff --check && git diff --cached --check`

### Completed slice 13. DMCR GC inventory/helper normalization

Цель:

- collapse duplicated storage-key normalization and stored-prefix scan skeleton
  inside DMCR GC;
- preserve cleanup request lifecycle, maintenance gate/quorum/release,
  direct-upload stale-age/protected-prefix policy, multipart gone-upload
  handling, report fields and delete behavior;
- keep `directUploadDeletePrefix` trailing-slash deletion guard explicit.

Файлы:

- `images/dmcr/internal/garbagecollection/storage_inventory.go`
- `images/dmcr/internal/garbagecollection/directupload_inventory.go`
- `images/dmcr/internal/garbagecollection/multipart_inventory.go`
- `images/dmcr/internal/garbagecollection/storage_s3.go`
- `images/dmcr/internal/garbagecollection/storage_s3_multipart.go`
- adjacent report/live/config/test-support key-normalization call sites.

Поведение:

- repository/raw and direct-upload object inventory now share the same
  package-local prefix scan helper for normalized key handling, prefix
  inference, object counting, sample capture and deterministic sorting;
- direct-upload timestamp fail-closed check stays in direct-upload inventory;
- direct-upload stale/protected/target policy and multipart part-count/gone
  semantics stay separate.

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/dmcr && go test -count=3 -run 'TestDiscoverDirectUploadInventory|TestDeletePostGarbageCollectDirectUploadPrefixes|TestBuildReportKeepsFreshDirectUploadPrefixAgeBounded|TestDiscoverDirectUploadMultipartInventory|TestBuildReportIncludesDirectUploadMultipartResidue|TestBuildReportKeepsFreshMultipartUploadAgeBounded|TestDeleteStalePrefixesAbortsMultipartUploads|TestCleanupPolicyForActiveRequests|TestInferRepositoryMetadataPrefix|TestInferSourceMirrorPrefix' ./internal/garbagecollection`
- `cd images/dmcr && go test -race ./internal/garbagecollection`
- `git diff --check && git diff --cached --check`

## 6. Next executable slice

### ModelPack OCI direct-upload session runner slimming

Цель:

- reduce duplicated direct-upload session progress/finalization plumbing inside
  `modelpack/oci`;
- preserve direct-upload protocol, late digest, retry/resume behavior,
  object-source ranged reads and registry verification semantics;
- keep this controller-side client path separate from DMCR server-side helpers.

Файлы:

- `images/controller/internal/adapters/modelpack/oci/direct_upload.go`
- `images/controller/internal/adapters/modelpack/oci/direct_upload_session.go`
- `images/controller/internal/adapters/modelpack/oci/publish_layers.go`
- package-local `modelpack/oci` tests only.

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- targeted `go test -count=3 -run 'Test.*DirectUpload|Test.*ObjectSource|Test.*Range' ./internal/adapters/modelpack/oci`
- `git diff --check && git diff --cached --check`

## 7. Future candidates

### DMCR GC state-machine consolidation

Резать только отдельным narrowed pass:

- lease utility consolidation inside `dmcr`;
- storage/TLS helper consolidation inside `dmcr`;
- GC request classification and policy normalization;

Не смешивать controller `modelpack/oci` direct-upload helpers with DMCR
server-side helpers across image boundary.

### ModelPack OCI byte-path consolidation

Резать только package-local and byte-path-safe:

- direct-upload session runner inside `modelpack/oci`;
- no cross-image shared protocol helpers.

## 8. Current validation evidence

Completed slices are kept here only as compact handoff state; detailed command
history lives in the chat/run logs, not in this active bundle.

- Slices 1-4: source-admission residue, workloaddelivery boilerplate,
  sourcefetch archive inspection, and `ClusterModel` schema hardening were
  validated with package tests plus `api` codegen/CRD checks.
- Slices 5-6: `modeldelivery` render/prune collapse and `sourceworker`
  package-local cleanup were validated with targeted package tests, `-count=3`,
  `-race`, `git diff --check`, and `make verify`.
- Follow-up sourceadmission slimming: removed the stale
  `internal/application/sourceadmission` pseudo-boundary. Source type detection
  remains in `domain/modelsource` / `sourcePlan`; owner-binding preflight now
  stays local to sourceworker request-state preparation. Validated with
  `cd images/controller && go test ./internal/domain/modelsource ./internal/adapters/k8s/sourceworker`,
  `cd images/controller && go test -count=3 ./internal/domain/modelsource ./internal/adapters/k8s/sourceworker`,
  and diff checks.
- Follow-up modeldelivery slimming: collapsed `Rendered` init-container output
  to a single slice contract and removed the separate single-container flag/
  field/apply branch. Direct delivery now means an empty init-container slice;
  materialize bridge and multi-model bridge use the same apply path. Validated
  with `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`,
  `cd images/controller && go test -count=3 -race ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`,
  and diff checks.
- Slice 7: DMCR lease utility consolidation was validated with:
  `cd images/dmcr && go test ./internal/garbagecollection ./internal/maintenance`;
  targeted `-count=3` GC/maintenance lease, gate and ack tests; `go test -race`
  for the same packages; `git diff --check && git diff --cached --check`.
- Slice 7 reviewer findings fixed: maintenance gate/file/ack paths remain
  fail-closed when `RenewTime` and `AcquireTime` are missing, and maintenance
  keeps its pre-existing `nil LeaseTransitions -> 0` takeover policy. Only GC
  executor election uses `CreationTimestamp` fallback. The negative tests
  `TestLeaseGateDoesNotTreatCreationTimestampOnlyForeignLeaseAsActive`,
  `TestFileMirrorTreatsLeaseWithOnlyCreationTimestampAsInactive` and
  `TestAckQuorumIgnoresLeaseWithOnlyCreationTimestamp` lock this behavior.
- Slice 8: DMCR request-state classification centralized in one
  package-local classifier; direct-upload cleanup policy split into local
  collect/decode/target stages. Gate/quorum/cleanup execution and request
  Secret schema were not changed. Targeted request selection/policy/result,
  direct-upload inventory adjacency, package, race and diff checks passed.
- Slice 9: partial `markRequestsCompleted` update failure is now covered by a
  helper-level replay/idempotency proof and a full `runRequestCycle` replay
  proof. The failed request stays active/replayable after cleanup already ran,
  the completed request stays done, a second cycle re-enters cleanup and
  completes the remaining request. Targeted `-count=3`, package, race, full
  `dmcr` and diff checks passed.
- Slice 10: direct-upload completion finalization is replay-safe after
  successful verification. Ambiguous storage errors no longer delete uploaded
  bytes or sealed metadata; duplicate upload cleanup happens only after repo
  link write succeeds; same-session retry preserves the object referenced by
  sealed metadata. Targeted `TestServiceComplete`, verification tests,
  controller direct-upload client tests, package, `-count=3`, race, full
  `dmcr` and diff checks passed.
- Slice 11: direct-upload test support and verification log slimming completed
  inside the DMCR package only. `service_test_support_test.go` was split into
  fake backend, HTTP harness and assertions; repeated decode helpers collapsed
  to one generic helper; client-asserted verification logging now uses one
  formatting helper while keeping precedence in `verifyUploadedObject` /
  `resolveFallbackVerification`. `verification.go` is 330 LOC,
  `service_test_support_test.go` is 154 LOC, all directupload files remain
  below the 350 LOC budget. Targeted package, `-count=3`, race and diff checks
  passed.
- Slice 12: `modelpack/oci` object-source tar shaping now has one
  package-local header/path helper shared by generated and ranged archive
  paths. New tests prove byte-for-byte equality for ranged tar output,
  compressed fallback to generated archive, and interrupted direct-upload
  resume through ranged reads for uncompressed tar object-source layers.
  Targeted package, repeated object-source/range/direct-upload and race checks
  passed.
- Slice 13: DMCR GC inventory now has one package-local storage path
  normalizer and one stored-prefix scan helper shared by repository/raw and
  direct-upload inventories. Direct-upload timestamp fail-closed behavior,
  stale-age/protected-prefix policy, multipart gone-upload handling and
  trailing-slash delete scope remain in their original owner paths. Package,
  targeted inventory/report/delete `-count=3`, race and diff checks passed.

Final check before handoff:

- `make verify` — pass after slice 13 GC inventory/helper normalization and
  `modelpack/oci` object-source tar-entry shaping.

## 9. Rollback point

Each slice remains rollbackable by its bounded write-set. For the current
DMCR slices, rollback means reverting `images/dmcr/internal/leaseutil/`, the
touched `garbagecollection` / `maintenance` lease/request/test files, and the
package-local `directupload` completion/finalization/test edits. No public
API, template, RBAC, registry byte path, token format or cleanup request API
was changed.
