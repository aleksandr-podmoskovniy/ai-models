# Plan: pod placement policy alignment

Archived: placement slice completed and folded into
`plans/active/live-e2e-ha-validation` as a live rollout/e2e check.

## 1. Phase

Phase 1/2 operations hardening. Change is limited to module-owned deployment
placement templates.

## 2. Orchestration

Mode: `solo`.

Reason: current request does not explicitly allow delegation, and the slice is
template-only with clear local references in DMCR/DVCR-style placement.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep. It remains an executable live validation
  workstream.
- `artifact-storage-quota-design` — archived to
  `plans/archive/2026/artifact-storage-quota-design`; previous slice is done.
- `pod-placement-policy` — archived after completion.

## 4. Slices

### Slice 1. Reference and placement decision

Status: done.

Files:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/templates/_helpers.tpl`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/templates/dvcr/deployment.yaml`
- `templates/dmcr/deployment.yaml`

Validation:

- live read-only check in `k8s.apiac.ru`: no `system` nodes, DMCR on workers,
  controller/upload-gateway on masters before this slice.

### Slice 2. Deployment templates

Status: done.

Files:

- `templates/controller/deployment.yaml`
- `templates/upload-gateway/deployment.yaml`
- `fixtures/render/managed-system-role.yaml`
- `tools/helm-tests/validate-renders.py`

Validation:

- `make helm-template`
- rendered grep for controller/upload-gateway placement.

### Slice 3. Verification

Status: done.

Validation:

- `make helm-template`
- `make kubeconform`
- `git diff --check -- templates fixtures/render tools/helm-tests plans/active/pod-placement-policy`
- targeted render checks for `managed-baseline` and `managed-system-role`.

## 5. Rollback

Revert two deployment template substitutions and the render guardrail fixture.
No runtime state rollback is required.
