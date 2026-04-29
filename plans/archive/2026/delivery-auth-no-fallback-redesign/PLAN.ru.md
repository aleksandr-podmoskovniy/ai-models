# PLAN: Remove workload materialize fallback and cluster-wide Secret writes

## Active bundle disposition

- keep `plans/active/live-e2e-ha-validation`: live validation remains
  executable after this slice.
- keep `plans/active/observability-signal-hardening`: separate observability
  workstream.
- keep `plans/active/ray-a30-ai-models-registry-cutover`: separate workload
  cutover workstream.
- keep `plans/active/modelpack-efficient-compression-design`: separate design
  workstream.
- archive `plans/archive/2026/delivery-auth-no-fallback-redesign`: auth/runtime
  hardening workstream is complete after final validation.

## Slice 1. Read-only architecture review

Status: done.

Inputs:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `templates/controller/rbac.yaml`
- current node-cache/CSI delivery contract

Expected output:

- exact Secret write sites;
- target ownership split;
- condition/backoff semantics;
- RBAC shape.

Evidence:

- `integration_architect`: controller ClusterRole must not write Secrets
  cluster-wide; workload delivery must stop projecting registry auth/image-pull
  Secrets into workload namespaces.
- `api_designer`: legacy bridge modes must leave the public delivery contract;
  blocked/not-ready state must stay explicit and stable.

## Slice 2. Remove materialize fallback from modeldelivery

Status: done.

Files:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/workloaddelivery/*`
- related tests.

Work:

- remove initContainer materializer injection from workload mutation;
- remove OCI credential env projection and runtime pull secret projection;
- keep stale cleanup for previously projected resources;
- fail closed when SharedDirect/node-cache cannot be selected.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery`

## Slice 3. Controller reconciliation UX

Status: done.

Files:

- `images/controller/internal/controllers/workloaddelivery/*`

Work:

- keep stable pending condition/reason;
- avoid noisy event spam for repeated not-ready cache state;
- ensure cleanup finalizer still removes stale legacy mutations.

Checks:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`

## Slice 4. RBAC/template hardening

Status: done.

Files:

- `templates/controller/rbac.yaml`
- helm tests under `tools/helm-tests/*`

Work:

- remove cluster-wide workload namespace Secret create/update/patch verbs from
  controller ClusterRole;
- keep cluster-wide Secret `delete` only for bounded cleanup of legacy projected
  auth/CA/image-pull Secrets that older versions already created in workload
  namespaces;
- keep module namespace Role for module-owned Secret state only;
- update static render checks.

Checks:

- `make helm-template`
- `make kubeconform`
- `git diff --check`

## Slice 5. Docs/e2e evidence

Status: done.

Files:

- e2e runbook / active validation plan if needed.

Work:

- record that fallback is removed;
- add check: annotated workload without ready node-cache fails closed and never
  receives Secret/materializer injection;
- add check: ready SharedDirect workload mounts CSI only.

Checks:

- `git diff --check`

Evidence:

- docs now describe SharedDirect CSI as the only supported workload delivery
  path;
- explicit workload cache volumes are documented as rejected, not as fallback;
- runtimehealth no longer exposes materializer init-state metrics.
- final review found stale legacy Secret cleanup gaps, stale README wording and
  active-bundle hygiene; cleanup tests now seed stale auth/CA/image-pull
  Secrets, README no longer describes materialize fallback, and this bundle is
  ready to archive after final validation.

## Final validation target

- done: `cd images/controller && go test ./...`
- done: `make deadcode`
- done: `make helm-template`
- done: `make kubeconform`
- done: `make verify`
- done: `git diff --check`
