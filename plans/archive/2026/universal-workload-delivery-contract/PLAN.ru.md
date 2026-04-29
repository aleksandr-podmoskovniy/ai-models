# PLAN: Universal workload delivery contract without controller-specific mutation

## Active bundle disposition

- keep `plans/active/live-e2e-ha-validation`: live validation runbook remains executable after this architecture slice.
- keep `plans/active/observability-signal-hardening`: separate observability hardening workstream.
- keep `plans/active/modelpack-efficient-compression-design`: separate ModelPack design workstream.
- archived `plans/archive/2026/ray-a30-ai-models-registry-cutover`: its
  KubeRay-specific controller support premise is no longer target
  architecture; future Ray validation must use the generic PodTemplate CSI
  contract.
- current: `plans/active/universal-workload-delivery-contract`.

## Slice 1. Read-only architecture review

Status: completed.

Files:

- `images/controller/internal/controllers/workloaddelivery/*ray*`
- `images/controller/internal/controllers/workloaddelivery/setup.go`
- `images/controller/internal/controllers/workloaddelivery/admission.go`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `templates/controller/*`

Expected evidence:

- exact KubeRay-specific code to remove;
- new universal annotation/CSI/PVC contract;
- RBAC/storage/auth risks.

Subagent conclusions:

- `repo_architect`: KubeRay support is a first-class fork across setup,
  watches, admission, template extraction and reconciliation. It must be
  removed instead of generalized by adding more third-party CRD shims.
- `integration_architect`: workload namespace materialize/PVC fallback is not
  safe without a new auth/data-plane design because it reintroduces registry
  Secret projection into arbitrary namespaces.
- `api_designer`: canonical contract should be PodTemplate/Pod oriented; CSI
  delivery must be explicit and user-scheduled. Controller may resolve model
  refs and stamp artifact attributes, but must not invent scheduling policy.

Decision for this implementation:

- Remove `RayService` / `RayCluster` support from the controller and webhook.
- Keep only built-in Kubernetes workload kinds with stable PodTemplate fields:
  `Deployment`, `StatefulSet`, `DaemonSet`, `CronJob`.
- `SharedDirect` requires an explicit node-cache CSI volume in the PodTemplate.
  The controller may fill canonical artifact URI/digest/family attributes and
  inject runtime env/mounts, but it must not inject node selectors/labels,
  affinity or node-cache readiness scheduling policy.
- Do not implement PVC/materialize fallback in this slice. A compatibility path
  requires a separate design with module-owned auth, cleanup and limits.

## Slice 2. Remove third-party controller ownership

Status: completed.

Work:

- remove RayService/RayCluster GVK registration, watches, mappers and reconcile branch;
- keep generic core workload handling only where ai-models owns a stable PodTemplate contract;
- update tests that encoded Ray-specific mutation.

Checks:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`

## Slice 3. Rework delivery topology contract

Status: completed.

Work:

- stop injecting node selectors/labels for CSI path;
- require explicit user-provided node-cache CSI volume declaration;
- do not define PVC/materialize compatibility path in this slice because
  current auth/storage constraints are not safe;
- keep no cluster-wide workload Secret create/update/patch.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`

## Slice 4. Templates/RBAC/docs/e2e plan

Status: completed.

Work:

- update webhook scope if Pod-level/generic admission is used;
- update RBAC validation;
- update docs and `live-e2e-ha-validation` runbook;
- update/close `ray-a30-ai-models-registry-cutover` assumptions.

Checks:

- `python3 -m unittest tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `make kubeconform`
- `git diff --check`

## Final validation target

- `cd images/controller && go test ./...`
- `make deadcode`
- `make verify`
- final `review-gate` and `reviewer` pass.

## Validation evidence

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller` — passed.
- `python3 -m unittest tools/helm-tests/validate_renders_test.py` — passed.
- `make helm-template` — passed.
- `make kubeconform` — passed.
- `git diff --check` — passed.
- `cd images/controller && go test ./...` — passed.
- `make deadcode` — passed.
- `make verify` — passed; rerun after review-gate cleanup also passed.
- final reviewer findings were addressed:
  - delivery cleanup finalizer is now added on production apply path;
  - examples, FAQ and e2e plan use the explicit user-declared CSI volume
    contract;
  - stale ready-node gating and auto-injected CSI wording was removed.

## Final disposition

Completed and archived to `plans/archive/2026/universal-workload-delivery-contract`.
