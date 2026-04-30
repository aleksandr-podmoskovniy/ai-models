# CSI workload UX and SharedPVC delivery contract plan

## Active bundle disposition

- `csi-workload-ux-contract`: keep/current только для следующего executable
  slice `SharedPVC RWX delivery contract`. Текущий rename-removal slice закрыт,
  но bundle не архивируется, потому что в нём остался целевой SharedPVC
  ownership/auth/GC design, который нельзя делать hidden fallback'ом.
- `live-e2e-ha-validation`: keep. Исполняемый validation stream после выката.
- `modelpack-efficient-compression-design`: keep. Отдельная storage-efficiency
  оптимизация.
- `observability-signal-hardening`: keep. Отдельный observability stream.
- `production-readiness-hardening`: keep. Широкий hardening stream; этот slice
  не должен дублировать его.

## Orchestration

Mode: `full` для contract/storage boundary review.

Read-only review before implementation:

- `api_designer`: annotations, public contract, failure UX.
- `integration_architect`: SharedDirect/SharedPVC storage/auth/HA boundaries.

Reviewer findings recorded:

- Name-only mount paths require a collision rule. Decision: `Model/foo` and
  `ClusterModel/foo` cannot be used in the same workload because both would
  target `/data/modelcache/models/foo`.
- `SharedPVC` must be controller-owned end to end. User-facing contract remains
  only workload model annotations; users must not bring their own PVC or DMCR
  path.
- Safe implementation requires exactly one public admin knob:
  `sharedPVC.storageClassName`.
- Workload-scoped claim/materializer readiness must not be pushed into
  `Model` / `ClusterModel` status.
- Controller-only RBAC may write internal PVCs in workload namespaces; human
  roles stay limited to `Model` / `ClusterModel`.
- Hidden materialize fallback or namespace DMCR Secret copy remains forbidden.

## Slice 1. Public annotation simplification

Files:

- `images/controller/internal/controllers/workloaddelivery/annotations.go`
- `images/controller/internal/controllers/workloaddelivery/resolve.go`
- tests in `internal/controllers/workloaddelivery`

Implementation:

- Replace legacy rename parser with comma-separated model names.
- Use model name as stable path name.
- Allow both namespaced and cluster models, reject duplicate path names.
- Remove legacy reference annotation from parser, webhook trigger and tests.

Validation:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`

Status: implemented.

## Slice 2. Stable model paths without primary special-case

Files:

- `images/controller/internal/nodecache/materialization_layout.go`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/workloaddelivery/*`
- related tests.

Implementation:

- Always expose `/data/modelcache/models/<model-name>`.
- Remove `/data/modelcache/model` from runtime env/mount contract.
- Keep simple env surface: `AI_MODELS_MODELS_DIR` and `AI_MODELS_MODELS`.
- Remove per-model legacy env variables from public docs; code may keep internal
  helpers only if tests prove they are not user-facing.

Validation:

- `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/modeldelivery ./internal/workloaddelivery`

Status: implemented for `modeldelivery` and `workloaddelivery` controller.

## Slice 3. SharedDirect / SharedPVC target boundary

Files:

- `docs/CONFIGURATION*.md`
- `docs/FAQ*.md`
- `docs/EXAMPLES*.md`
- `docs/USER_GUIDE*.md`
- `plans/active/csi-workload-ux-contract/PLAN.ru.md`

Implementation:

- Document only three delivery outcomes: `SharedDirect`, `SharedPVC`,
  `Disabled/Blocked`.
- State RWX StorageClass requirement for `SharedPVC`.
- State no user-facing RWO mode.
- State SharedPVC implementation requires workload-owned RWX PVC and
  digest-scoped materializer auth; unsafe secret-copy fallback is forbidden.

Validation:

- `git diff --check`

Status: docs updated for current `SharedDirect` behavior and target
`SharedPVC` boundary.

## Slice 4. Final review

- Run focused Go tests.
- Run `git diff --check`.
- Run `make verify` if feasible.
- Run `review-gate`.
- If subagent findings add blockers, record them in this plan before final.

Status:

- implemented.
- Focused checks passed:
  `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/workloaddelivery`.
- Full controller check passed:
  `cd images/controller && go test ./...`.
- Repo checks passed:
  `make deadcode`, `make helm-template`, `make kubeconform`,
  `git diff --check`, `make verify`.
- Final reviewer required because the slice used architecture subagents and
  changed controller/runtime/docs surfaces.

Status:

- `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/workloaddelivery` passed.
- `cd images/controller && go test ./...` passed.
- `make helm-template` passed.
- `make kubeconform` passed.
- `make deadcode` passed after removing the unreachable legacy single-model
  managed cache path.
- `make verify` passed.

## Slice 5. SharedPVC RWX delivery foundation

Files:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/deployment.yaml`
- `templates/controller/rbac.yaml`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `docs/CONFIGURATION*.md`
- `docs/FAQ*.md`

Implementation:

- Add public `sharedPVC.storageClassName` as the single RWX StorageClass knob.
- Pass SharedPVC config into controller runtime.
- Add controller-owned RWX PVC naming/ownership based on workload owner and
  requested model name/digest set.
- Compute PVC request from published artifact `sizeBytes` plus internal
  filesystem headroom.
- Mutate workload template to mount the controller-owned PVC read-only at
  `/data/modelcache/models`.
- Keep workload fail-closed on scheduling gate while the PVC is not `Bound` or
  digest-scoped materialization is not ready.
- Delete stale SharedPVC claims for the same workload owner when the model set
  changes and on delivery cleanup.
- Delete SharedPVC claims when delivery becomes pending/blocked again, so stale
  bytes are not kept behind a stopped workload.
- Report missing delivery backend as stable `SharedPVCStorageClassMissing`
  instead of a generic cache-mount contract error.
- Update helm render guardrails: controller cluster-wide writes remain
  forbidden for Pods/Leases/Secrets; PVC lifecycle verbs are intentional for
  controller-owned SharedPVC.
- Do not create workload namespace DMCR Secret/CA or materializer init
  container.

Validation:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `cd images/controller && go test ./...`
- `make helm-template`
- `make kubeconform`
- `make deadcode`
- `git diff --check`
- `make verify`

Status: implemented.

## Next executable slice. SharedPVC digest-scoped materializer grant

Open work that remains intentionally out of this code slice:

- define digest-scoped materializer auth/read-grant without copying shared
  registry Secrets into workload namespaces;
- define controller-owned short-lived materializer Job lifecycle, retry and
  cleanup;
- release SharedPVC gate only after all requested model directories have ready
  markers;
- add metrics for materialized RWX bytes and stale claim pressure;
- add RBAC evidence for namespace-local materializer Jobs/ServiceAccounts
  before implementation.
