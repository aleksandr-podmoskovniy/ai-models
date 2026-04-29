# План: workload delivery cleanup finalizer

## Current Phase

Phase 1/2 publication/runtime baseline перед production rollout.

## Orchestration

`solo`: текущий turn не авторизовал subagents; slice ограничен одним
controller boundary и закрывается unit tests.

## Active Bundle Disposition

- `live-e2e-ha-validation` — keep: executable e2e после rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельная workload migration.

## Slices

1. Add finalizer lifecycle.
   - Files: `images/controller/internal/controllers/workloaddelivery/*`.
   - Status: completed.

2. Cover cleanup behavior.
   - Files: workloaddelivery tests.
   - Status: completed.

3. Validate and archive.
   - Checks: targeted go test, diff check, `make verify`.
   - Status: completed.
   - Checks passed:
     - `go test ./internal/controllers/workloaddelivery`
     - `git diff --check -- images/controller/internal/controllers/workloaddelivery plans/active/prod-preflight-workload-delivery-finalizer`
     - `make verify`

## Rollback Point

Revert this bundle's workloaddelivery finalizer lifecycle changes and tests.
