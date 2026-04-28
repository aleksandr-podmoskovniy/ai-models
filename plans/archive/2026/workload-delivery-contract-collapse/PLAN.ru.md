# Plan: workload delivery contract collapse

## Phase

Phase 1/2 runtime baseline hardening: architecture cleanup without behavior or
public API change.

## Orchestration

Mode: `solo`.

Reason: one bounded dependency-direction cleanup; no new product semantics and
no public schema/template/RBAC change.

## Active bundle disposition

- `live-e2e-ha-validation` — keep: executable live validation workstream after
  code rollout.
- `workload-delivery-contract-collapse` — current narrow implementation slice.

## Slices

### Slice 1. Shared workload delivery contract

Files:

- `images/controller/internal/workloaddelivery/`
- `images/controller/internal/nodecache/desired_artifacts.go`
- `images/controller/internal/adapters/k8s/modeldelivery/options.go`
- `images/controller/internal/monitoring/runtimehealth/workload_delivery.go`
- focused tests.

Checks:

- `cd images/controller && go test -count=1 ./internal/workloaddelivery ./internal/nodecache ./internal/adapters/k8s/modeldelivery ./internal/monitoring/runtimehealth`
- `git diff --check`

## Validation evidence

- `cd images/controller && go test -count=1 ./internal/workloaddelivery
  ./internal/nodecache ./internal/adapters/k8s/modeldelivery
  ./internal/monitoring/runtimehealth` — passed.
- `git diff --check` — passed.
- `rg -n "internal/adapters/k8s/modeldelivery" images/controller/internal/monitoring
  images/controller/internal/nodecache -g'*.go'` — no matches.
- `make verify` — passed.
