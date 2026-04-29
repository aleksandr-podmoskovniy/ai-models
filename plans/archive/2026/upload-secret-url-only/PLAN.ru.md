# План: upload только через secret URL

## Current phase

Phase 1/2 publication/runtime baseline. Slice сужает upload auth contract.

## Orchestration

`solo`: изменение узкое, продолжает только что закрытый upload UX slice.

## Slices

1. Remove bearer-header auth from gateway.
   - Files: `images/controller/internal/dataplane/uploadsession/*`
   - Check: package tests.
   - Status: done.

2. Make token handoff Secret internal/raw-token shaped.
   - Files: `images/controller/internal/adapters/k8s/uploadsession/*`
   - Check: package tests.
   - Status: done.

3. Align docs/notes.
   - Files: `images/controller/README.md`, current plan bundle.
   - Check: `git diff --check`.
   - Status: done.

## Rollback

Revert this bundle to restore bearer-header compatibility.

## Final validation

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession`
- `cd images/controller && go test ./...`
- `git diff --check`

## Evidence

- `cd images/controller && go test ./internal/dataplane/uploadsession ./internal/adapters/k8s/uploadsession`
- `cd images/controller && go test ./...`
