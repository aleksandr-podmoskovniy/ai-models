# Production readiness hardening plan

## 1. Current phase

Phase 1/2 rollout hardening. Цель — закрыть продовые дефекты текущего baseline
без расширения public API и без переноса отложенных оптимизаций в rollout
gate.

## 2. Orchestration

Mode: `full` по правилам репозитория, потому что scope затрагивает security,
storage, observability, HA and API conventions.

Practical execution in this pass:

- use existing active bundles and prior architecture reviews as input;
- do not spawn new subagents unless the user explicitly requests delegation in
  the current turn;
- run `review-gate` before final answer for any code/template changes.

## 3. Active bundle disposition

- `csi-workload-ux-contract` — keep. Следующий executable slice for generic
  PodTemplate CSI delivery UX and docs/e2e alignment.
- `live-e2e-ha-validation` — keep. Это отдельный live-runbook, запускать только
  после явной команды пользователя.
- `modelpack-efficient-compression-design` — keep. Chunked immutable layout is
  implemented enough for code validation, but publish-side compression remains
  future work.
- `observability-signal-hardening` — keep. Live proof and any remaining backend
  SDK noise cleanup belong there.
- `production-readiness-hardening` — current compact audit/fix bundle.

## 4. Slices

### Slice 1. Current diff safety audit

Goal:

- проверить текущий uncommitted diff на file-size, security, API convention and
  fallback regressions before deeper changes.

Files:

- current changed files from `git status`;
- this plan.

Checks:

- `./tools/ci/run-suite.sh verify`;
- static scans for privileged pods, secret writes, broad RBAC, TODO/fallback,
  unsafe logs.

Artifact:

- concrete findings, not generic rewrite requests.

Status: completed.

Findings:

- Controller and upload-gateway used legacy service account token automount
  while DMCR already used explicit projected short-lived token. This was a
  real security convention drift.
- Controller-owned publication worker Pods and node-cache runtime Pods used
  legacy service account token automount even though both only need kube API
  access through the standard in-cluster token path.
- The new projected kube API token volume shaping was duplicated across two
  concrete K8s adapters; leaving it that way would create a third local token
  contract on the next runtime Pod.
- Controller ClusterRole still granted cluster-wide `get/list/watch` on
  `secrets` while the remaining cluster-scoped secret operation is only
  point-delete of legacy projected workload secrets during cleanup.
- Public `status.resolved.sourceCapabilities` projected provider evidence from
  external metadata without a count/length guard before status update.
- CRD schema did not mirror the new source-capability projection bounds, so
  generated OpenAPI did not document the controller-owned status envelope.
- Chunked materialize validation covered path/digest/ranges but did not bound
  single-chunk payload size and did not guard all offset/length additions
  against `int64` overflow.

### Slice 2. Targeted defect fixes

Goal:

- исправлять только подтверждённые defects from Slice 1.

Files:

- touched packages only.

Checks:

- package-level tests;
- `git diff --check`.

Artifact:

- smaller, safer diff with no new abstractions unless needed by a real
  boundary.

Status: completed for current pass.

Implemented:

- controller and upload-gateway now set `automountServiceAccountToken: false`
  and mount explicit projected service account token volumes with
  `expirationSeconds: 3600`;
- publication worker and node-cache runtime Pods now also disable legacy token
  automount and mount explicit projected service account token volumes with
  `expirationSeconds: 3600`;
- projected kube API service account token shaping is now a single reusable
  `k8s/podprojection` primitive used by both generated runtime Pod adapters;
- controller ClusterRole now grants only cluster-wide `delete` on `secrets`;
  module-namespace Secret writes stay in the namespaced runtime Role;
- Helm render guardrail now rejects legacy automount for these deployments and
  requires projected token evidence;
- Helm RBAC guardrail now rejects any controller ClusterRole secret verb other
  than the required legacy-cleanup `delete`;
- public source-capability evidence projection now bounds provider-originated
  task/feature strings before writing status;
- `Model` / `ClusterModel` CRD status schema now bounds endpoint/feature lists
  and source-capability evidence arrays/string length to match controller
  projection limits;
- chunked immutable layout validation now enforces bounded per-chunk payload
  size and rejects overflowing pack/file ranges;
- chunked materialize avoids double-closing the output file descriptor.

### Slice 3. Plan and docs alignment

Goal:

- убрать stale statements if code changed;
- keep active plans executable.

Files:

- relevant `plans/active/**`;
- docs only if behavior changed.

Checks:

- `git diff --check`.

Status: completed for current pass.

Updated:

- current plan records concrete findings and validation evidence.

### Slice 4. Final gate

Goal:

- prove repository is ready for push and live e2e after user rollout.

Checks:

- focused package tests;
- `./tools/ci/run-suite.sh verify` if feasible;
- `review-gate`.

Status: completed for current pass.

Validation passed:

- `python3 -m unittest tools/helm-tests/validate_renders_test.py`;
- `make helm-template`;
- `make kubeconform`;
- `cd images/controller && go test -count=1 ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/nodecacheruntime`;
- `cd images/controller && go test -count=1 ./internal/adapters/k8s/podprojection`;
- `cd images/controller && go test -count=1 ./internal/domain/publishstate`;
- `cd api && go test ./...`;
- `bash api/scripts/update-codegen.sh`;
- `bash api/scripts/verify-crdgen.sh`;
- `cd images/controller && go test -count=1 ./internal/adapters/modelpack/oci`;
- `./tools/ci/run-suite.sh verify`;
- `git diff --check`.

Review-gate result:

- no blocking architecture/security/API-convention findings in the final diff;
- remaining active bundles are executable continuation surfaces, not duplicate
  sources of truth.

## 5. Rollback point

The safe rollback point is before Slice 2. Slice 1 is plan/audit only. Any
targeted code fix must remain independently revertible by package.

## 6. Final validation

- `git diff --check`
- package tests for changed packages
- `./tools/ci/run-suite.sh verify`
