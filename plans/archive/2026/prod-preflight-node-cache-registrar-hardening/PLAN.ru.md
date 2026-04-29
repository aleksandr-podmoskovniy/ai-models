# План: node-cache registrar security hardening

## Current Phase

Phase 1/2 publication/runtime baseline перед production rollout.

## Orchestration

`solo`: slice узкий, без API/RBAC/storage redesign; проверяется unit test.

## Active Bundle Disposition

- `live-e2e-ha-validation` — keep: executable e2e после rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельная workload migration.

## Slices

1. Harden registrar security context.
   - Files: `images/controller/internal/adapters/k8s/nodecacheruntime/pod.go`.
   - Status: completed.

2. Add regression coverage.
   - Files: `images/controller/internal/adapters/k8s/nodecacheruntime/pod_test.go`.
   - Status: completed.

3. Validate and archive.
   - Checks: targeted go test, diff check, `make verify` if narrow checks pass.
   - Status: completed.
   - Checks passed:
     - `go test ./internal/adapters/k8s/nodecacheruntime`
     - `git diff --check -- images/controller/internal/adapters/k8s/nodecacheruntime plans/active/prod-preflight-node-cache-registrar-hardening`
     - `make verify`

## Rollback Point

Revert this bundle's node-cache registrar security-context change and test.
