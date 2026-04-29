# План: prod preflight hardening 2

## Current Phase

Phase 1/2 publication/runtime baseline перед production rollout.

## Orchestration

`solo`: текущий turn не авторизовал subagents, поэтому проход делается локально
как audit + narrow fixes. Если будет найден архитектурный конфликт по API/RBAC
или CSI boundary, его надо вынести в отдельный `full` slice.

## Active Bundle Disposition

- `live-e2e-ha-validation` — keep: executable e2e после rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельная workload migration.

## Slices

1. Template/RBAC/security audit.
   - Files: `templates/`, rendered manifests.
   - Goal: find token automount drift, wildcard RBAC, accidental aggregation,
     missing restricted contexts.
   - Checks: `rg`, `make helm-template`, render validator.
   - Status: completed. Found storage-accounting RBAC mismatch for
     `publication-worker`: it can create the accounting Secret through
     `storageaccounting.Store`, but the dedicated Role only granted
     `get/update`. `upload-gateway` already had the needed verb set and got an
     explicit render guardrail.

2. Runtime/API audit.
   - Files: `api/`, `crds/`, `images/controller/internal/*`.
   - Goal: find public Secret/status leaks, unsafe runtime writes, token usage
     without matching RBAC, Kubernetes convention drift.
   - Checks: targeted `rg`, narrow Go tests.
   - Status: completed. Confirmed the mismatch from runtime code:
     `storageaccounting.Store.update` calls `client.Create` when the ledger
     Secret does not exist.

3. Narrow fixes and guardrails.
   - Files: based on findings only.
   - Goal: fix confirmed issues without new topology/UX.
   - Checks: package tests and render guardrails.
   - Status: completed. Added `create` to module-local Secret verbs for
     `upload-gateway` and `publication-worker`, and extended render validator
     coverage.

4. Final validation and archive.
   - Goal: update evidence, run repo checks, review-gate, archive bundle.
   - Checks: `git diff --check`, `make verify`.
   - Status: completed.

## Evidence

- `python3 -m py_compile tools/helm-tests/validate-renders.py`
- `python3 -m unittest tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `make kubeconform`
- `git diff --check`
- `rg -n 'resources:\s*\["\*"\]|verbs:\s*\["\*"\]|apiGroups:\s*\["\*"\]' templates tools/kubeconform/renders`
- `make verify`

## Findings Closed

- `upload-gateway` render guardrail now requires `get/create/update` on
  module-namespace Secrets. This matches upload session updates and first-use
  storage accounting ledger creation.
- `ai-models-publication-worker` Role now allows `get/create/update` on
  module-namespace Secrets. This matches direct-upload state update plus
  storage accounting reservation creation for remote-source publication.
- Render validator now rejects missing `create` on both runtime Secret paths.

## Residual Risks

- `create` on Secrets cannot be constrained by `resourceNames` in Kubernetes
  RBAC. The risk is bounded by namespace scope and dedicated runtime
  ServiceAccounts; human-facing roles remain unchanged.
- Secret upload URLs in status remain intentional, matching virtualization's
  `status.imageUploadURLs` pattern.

## Rollback Point

Revert only this bundle's narrow patches and remove the bundle.

## Final Validation

- `git diff --check`
- `make helm-template`
- `make kubeconform`
- `make verify`
