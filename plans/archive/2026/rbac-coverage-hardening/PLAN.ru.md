# Plan: RBAC coverage hardening

## 1. Phase

Phase 1/2 hardening. Work is limited to human-facing RBAC coverage and e2e
evidence gates.

## 2. Orchestration

Mode: `solo`.

Reason: user did not explicitly request subagents in this turn; the Deckhouse
reference is local and the implementation is limited to templates/evidence
checks.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep active; receives the executable live RBAC
  matrix.
- `rbac-coverage-hardening` — archived after completion; no follow-up slice
  remains inside this focused static/evidence hardening bundle.

## 4. Slices

### Slice 1. Deckhouse reference check

Status: done.

Evidence:

- `modules/140-user-authz/hooks/handle_custom_cluster_roles.go` aggregates
  custom module ClusterRoles for six levels only:
  `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`,
  `ClusterAdmin`.
- `SuperAdmin` remains a global persona in user-authz; module-local
  `SuperAdmin` fragments are not part of Deckhouse custom role aggregation.
- rbacv2 module roles use `rbac.deckhouse.io/kind: use/manage` and
  aggregate labels such as `aggregate-to-kubernetes-as`.

### Slice 2. Static/render guardrails

Status: done.

Files:

- `tools/helm-tests/validate-renders.py`

Result:

- Static checks now require all six Deckhouse custom-role access levels.
- Static checks reject a module-local `SuperAdmin` fragment because Deckhouse
  user-authz does not aggregate custom `SuperAdmin` ClusterRoles.
- Static checks now verify rbacv2 `use` / `manage` role names, aggregate
  labels, resources and verbs.

### Slice 3. E2E matrix

Status: done.

Files:

- `plans/active/live-e2e-ha-validation/PLAN.ru.md`
- `plans/active/live-e2e-ha-validation/RUNBOOK.ru.md`
- `plans/active/live-e2e-ha-validation/NOTES.ru.md`

Result:

- E2E plan/runbook now requires an explicit allow/deny table for
  `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`,
  `ClusterAdmin`, global `SuperAdmin`, and rbacv2 `use` / `manage`.
- Sensitive human-facing paths are stop conditions.

### Slice 4. Verification

Status: done.

Checks:

- `make helm-template` — passed.
- `make kubeconform` — passed.
- `git diff --check` — passed.
- `git diff --cached --check` — passed.

## 5. Rollback

Remove added validation/e2e matrix text. RBAC templates stay at the previous
permissions.
