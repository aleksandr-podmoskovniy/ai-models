# Plan: capacity and cache admission hardening

## 1. Current phase

Phase 1/2 hardening. Publication/runtime baseline works, but capacity planning
must become part of the runtime contract before larger production tests.

## 2. Orchestration

Mode: `full`.

Reason: the task touches storage, node-cache delivery, DMCR observability and
runtime failure semantics. Read-only reviews before code changes:

- `integration_architect`: storage/cache topology, Deckhouse/virtualization
  style guardrails, live safety.
- `backend_integrator`: DMCR/direct-upload/GC and reservation failure modes.
- `api_designer`: whether the failure/status shape leaks internal backend
  details or needs public API changes.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep. It is the canonical live runbook/evidence
  workstream; this task will only add next-run requirements if needed.
- `observability-signal-hardening` — keep. It owns collector/log signal
  hardening; this task may fix a narrow DMCR GC output issue but should not
  consume the whole observability workstream.
- `capacity-cache-admission-hardening` — current implementation workstream.

## 4. Slices

### Slice 1. Storage accounting wiring audit

Goal:

- verify current `storagecapacity` ledger usage across Upload, HF Direct,
  Mirror, Ready commit and delete release.

Files:

- `images/controller/internal/domain/storagecapacity/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/internal/adapters/k8s/storageaccounting/*`

Checks:

- `cd images/controller && go test ./internal/domain/storagecapacity ./internal/adapters/k8s/storageaccounting ./internal/dataplane/uploadsession ./internal/controllers/catalogstatus`

Artifact:

- notes in this plan about exact gap before implementation.

Audit notes:

- Upload already reserves on probe and releases on terminal upload paths.
- Ready/delete commit and release published usage.
- HF Direct/Mirror do not reserve before source-worker heavy byte transfer.
- Backend review: Upload reservation can leak when a `Model`/`ClusterModel` is
  deleted before terminal upload phase; delete cleanup must release both owner
  and upload-session reservations before deleting runtime state.
- Backend review: ledger needs a single promote operation from reservation to
  published artifact; separate release+commit can double-count during retries.
- Backend review: Mirror steady-state owns both canonical DMCR artifact bytes
  and durable raw mirror bytes until backend cleanup; Ready accounting must not
  count only public artifact bytes.
- API split after `api_designer` review: `InsufficientStorage` is a stable
  public catalog publication failure reason on existing conditions;
  `InsufficientNodeCache` is workload-delivery event/retry/fallback state and
  must not be written to `Model` / `ClusterModel` status.
- Integration review: SharedDirect readiness currently proves a ready runtime
  node exists, but not that the digest set fits that node cache. The correct
  boundary is module-private node-cache summary plus workload-delivery
  admission; no public `Model` status or access-level RBAC widening.
- Integration review: DMCR GC should keep structured counters as the normal
  signal and keep raw registry output only for failure/debug paths.

Status: implemented.

Evidence:

- `storagecapacity` gained atomic commit-and-replace semantics for owner/upload
  reservations.
- catalog Ready commit replaces owner/upload reservation IDs in one ledger
  mutation; terminal Failed releases pending publication reservations; delete
  cleanup releases owner reservation, upload reservation and published usage.
- Targeted tests passed:
  `cd images/controller && go test ./internal/domain/storagecapacity ./internal/adapters/k8s/storageaccounting ./internal/controllers/catalogstatus ./internal/controllers/catalogcleanup`.

### Slice 2. Publication fail-fast reservation

Goal:

- reserve estimated publish bytes for HF Direct/Mirror after source listing and
  before source-worker heavy upload;
- surface insufficient storage as stable public condition/reason.

Files:

- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/publishstate/*`

Checks:

- targeted tests for successful reservation, replay/idempotency and
  insufficient-storage failure.

Status: implemented for HF Direct/Mirror and existing Upload path.

Evidence:

- HF Direct/Mirror now perform cheap remote file HEAD planning before byte
  transfer.
- Direct reserves a conservative canonical OCI artifact estimate, including
  selected file bytes, tar padding/header overhead and OCI metadata reserve.
- Mirror reserves both durable raw source bytes and the same canonical OCI
  artifact estimate before mirror transfer.
- Source-worker receives storage accounting context from controller-owned
  runtime args.
- `InsufficientStorage` is mapped to public catalog condition reason on failed
  source-worker termination messages.
- Targeted tests passed:
  `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/k8s/sourceworker ./internal/dataplane/publishworker ./internal/application/publishobserve ./cmd/ai-models-artifact-runtime`.

### Slice 3. Node-cache capacity preflight

Goal:

- decide SharedDirect only when node-cache runtime is ready and requested
  digest set fits cache budget or has an eviction plan;
- otherwise produce explicit reason and use allowed fallback.

Files:

- `images/controller/internal/nodecache/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`

Checks:

- nodecache capacity math tests;
- workload delivery topology tests for insufficient node-cache capacity.

Status: implemented first guardrail.

Evidence:

- `resolved-models` annotation now carries `sizeBytes` for multi-model
  SharedDirect contracts.
- Workload delivery keeps a scheduling gate when requested model bytes are
  unknown or exceed the configured per-node shared cache capacity.
- Workload delivery now records explicit node-cache gate events:
  `ModelDeliveryBlocked` for insufficient capacity and `ModelDeliveryPending`
  for no matching ready node-cache runtime node.
- Node-cache shared contract has size normalization/capacity math tests.
- Targeted tests passed:
  `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`.

Follow-up status:

- Superseded by Slice 6 in this bundle.

### Slice 4. DMCR GC output hardening

Goal:

- keep structured deletion counters as operator signal;
- reduce raw `registryOutput` log/Secret bloat without hiding failure output.

Files:

- `images/dmcr/internal/garbagecollection/*`

Checks:

- DMCR GC tests for result shape and bounded output.

Status: implemented.

Evidence:

- Successful GC stores/logs structured counters and no longer emits raw
  `registryOutput` as normal operator signal.
- Completed request results keep only `registryOutput` line count and SHA-256
  fingerprint for drift/debug evidence, not raw registry output.
- Targeted tests passed:
  `cd images/dmcr && go test ./internal/garbagecollection`.

### Slice 5. Mirror e2e runbook closure

Goal:

- make forced Mirror-mode validation an explicit next e2e step with rollback
  and expected capacity accounting evidence.

Files:

- `plans/active/live-e2e-ha-validation/RUNBOOK.ru.md`
- `plans/active/live-e2e-ha-validation/PLAN.ru.md`

Checks:

- `git diff --check`

Status: implemented.

Evidence:

- `plans/active/live-e2e-ha-validation/RUNBOOK.ru.md` now requires Direct and
  Mirror capacity evidence, Mirror double-owned-byte accounting and
  SharedDirect size/capacity gate evidence.

### Slice 6. Per-node node-cache free-space summary

Goal:

- publish module-private per-node cache usage/free summary from node-cache
  runtime to controller;
- make SharedDirect admission use live free bytes on matching ready nodes, not
  only static PVC capacity;
- keep the signal private to the module runtime namespace and avoid public
  `Model` / `ClusterModel` API changes.

Files:

- `images/controller/internal/nodecache/*`
- `images/controller/internal/dataplane/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `templates/node-cache-runtime/rbac.yaml`

Checks:

- `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/dataplane/nodecacheruntime`
- `make helm-template`
- `git diff --check`

Status: implemented.

Evidence:

- node-cache runtime now reports module-private per-node usage/free summary by
  patching its own managed runtime Pod annotation in the module namespace.
- summary includes post-maintenance used bytes, configured budget free bytes,
  actual filesystem free bytes from `statfs` and ready digest set, so workload
  delivery charges only missing artifacts against effective live free space.
- SharedDirect admission now waits for a matching ready node usage summary and
  blocks only when all matching summarized nodes lack enough free cache bytes.
- node-cache runtime RBAC was kept narrow: cluster-wide read remains only for
  scheduled workload Pod discovery; write is namespace-local `pods/patch` for
  the runtime Pod annotation.
- Targeted checks passed:
  `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/dataplane/nodecacheruntime`.
- Template checks passed: `make helm-template`, `make kubeconform`,
  `git diff --check`.

## 5. Rollback point

Rollback is safe at slice boundary by reverting changed files. If a build with
new storage reservations is deployed, delete only test reservations/artifacts
created by the e2e run; do not delete `ai-models-storage-accounting` unless the
whole slice is rolled back before production use.

## 6. Final validation

- Targeted Go tests for every changed package.
- `cd images/controller && go test ./...` if controller code changes.
- `cd images/dmcr && go test ./...` if DMCR code changes.
- `git diff --check`.
- `make helm-template` and `make kubeconform` if templates change.
- `review-gate`; because this task uses delegation, run a final reviewer pass
  if the code changes cross controller and DMCR boundaries.

Executed:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/nodecache`
- `cd images/dmcr && go test ./internal/garbagecollection`
- `git diff --check`
- `make deadcode`
- `make verify`
- `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/dataplane/nodecacheruntime`
- `make helm-template`
- `make kubeconform`

Final reviewer findings were addressed:

- HF reservation no longer underestimates the final OCI artifact by using raw
  file bytes only.
- SharedDirect capacity gate now has explicit workload events instead of a
  generic scheduling gate only.
- DMCR GC success keeps structured fingerprint evidence without raw normal-path
  `registryOutput`.
