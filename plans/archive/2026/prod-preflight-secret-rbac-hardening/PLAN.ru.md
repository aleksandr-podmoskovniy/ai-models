# План: controller runtime RBAC hardening

## Current Phase

Phase 1/2 publication/runtime baseline перед production rollout.

## Orchestration

`solo`: пользователь не авторизовал subagents в текущем turn; изменение узкое,
по одному RBAC-risk и render guardrail.

## Active Bundle Disposition

- `live-e2e-ha-validation` — keep: executable e2e после rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельная workload migration.

## Slices

1. Audit Secret RBAC usage.
   - Files: `templates/controller/rbac.yaml`, controller/adapters.
   - Goal: confirm which Secret verbs need cluster scope vs module namespace.
   - Status: completed.
   - Finding: workload delivery currently projects registry/imagePull Secrets
     into arbitrary workload namespaces and cleans them up by workload UID.
     Kubernetes RBAC cannot restrict this by object name/label; removing
     cluster-wide Secret writes now would break delivery. This needs a separate
     delivery-auth redesign slice, not a fake RBAC hardening patch.

2. Split controller runtime object RBAC.
   - Files: `templates/controller/rbac.yaml`.
   - Goal: keep required cluster-wide read/watch for Pods/PVCs, but move
     module-owned Pod/PVC/Lease writes into module namespace Role/RoleBinding.
   - Status: completed.

3. Add render guardrail.
   - Files: `tools/helm-tests/validate-renders.py`,
     `tools/helm-tests/validate_renders_test.py`.
   - Goal: fail render validation if controller ClusterRole regains
     cluster-wide write verbs for Pods/PVCs/Leases or loses the module
     namespace runtime-writes Role.
   - Status: completed.
   - Check: `python3 -m unittest tools/helm-tests/validate_renders_test.py`.

4. Validate and archive.
   - Checks: `git diff --check`, `make helm-template`, `make kubeconform`,
     `make verify`, review-gate.
   - Status: completed.
   - Checks passed:
     - `python3 -m unittest tools/helm-tests/validate_renders_test.py`
     - `git diff --check -- templates/controller/rbac.yaml tools/helm-tests/validate-renders.py tools/helm-tests/validate_renders_test.py plans/active/prod-preflight-secret-rbac-hardening`
     - `make helm-template`
     - `make kubeconform`
     - `make verify`
   - Review-gate finding addressed: original Secret-write acceptance was
     over-promising and is now recorded as residual delivery-auth architecture
     risk instead of hidden under this slice.

## Rollback Point

Revert this bundle's RBAC and render-validator changes.

## Final Validation

- `python3 -m unittest tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
