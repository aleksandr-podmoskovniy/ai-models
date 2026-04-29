# План: service account token mount hardening

## Current Phase

Phase 1/2 publication/runtime baseline перед production rollout.

## Orchestration

`solo`: пользователь не авторизовал subagents в текущем turn; slice узкий и
проверяется unit/render guardrails.

## Active Bundle Disposition

- `live-e2e-ha-validation` — keep: executable e2e после rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельная workload migration.

## Slices

1. Audit token consumers.
   - Files: templates and generated runtime Pod builders.
   - Goal: identify Pods that need API token vs must not have it.
   - Status: completed.
   - Finding: DMCR already disables token; controller, upload-gateway,
     publication worker and node-cache runtime use Kubernetes API and should be
     explicit `true` rather than default-dependent.

2. Make automount explicit.
   - Files: controller/upload templates, sourceworker and nodecacheruntime pod
     builders.
   - Status: completed.

3. Add guardrails.
   - Files: unit tests and render validator.
   - Status: completed.
   - Checks:
     - `python3 -m unittest tools/helm-tests/validate_renders_test.py`
     - `go test ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/nodecacheruntime`
     - `git diff --check -- templates/controller/deployment.yaml templates/upload-gateway/deployment.yaml images/controller/internal/adapters/k8s/sourceworker images/controller/internal/adapters/k8s/nodecacheruntime tools/helm-tests/validate-renders.py tools/helm-tests/validate_renders_test.py plans/active/prod-preflight-service-account-token-hardening`

4. Validate and archive.
   - Checks: targeted go tests, render tests, `make helm-template`,
     `make kubeconform`, `make verify`, `git diff --check`.
   - Status: completed.
   - Checks passed:
     - `python3 -m unittest tools/helm-tests/validate_renders_test.py`
     - `go test ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/nodecacheruntime`
     - `git diff --check -- templates/controller/deployment.yaml templates/upload-gateway/deployment.yaml images/controller/internal/adapters/k8s/sourceworker images/controller/internal/adapters/k8s/nodecacheruntime tools/helm-tests/validate-renders.py tools/helm-tests/validate_renders_test.py plans/active/prod-preflight-service-account-token-hardening`
     - `make helm-template`
     - `make kubeconform`
     - `make verify`

## Rollback Point

Revert this bundle's template, PodSpec and validator changes.
