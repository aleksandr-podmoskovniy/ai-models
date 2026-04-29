# План: prod security и API conventions

## Current phase

Phase 1/2 publication/runtime baseline перед rollout в live cluster.

## Orchestration

`solo`: текущий проход узкий и правит один пользовательский upload/API seam.
Full review с subagents нужен для следующего широкого pre-release audit, но в
этом slice нет нового storage/runtime topology.

## Active bundle disposition

- `live-e2e-ha-validation` — keep: executable e2e после rollout.
- `observability-signal-hardening` — keep: отдельный observability stream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельный workload migration stream.
- `upload-secret-url-only` — archived: закрытый predecessor.

## Slices

1. API naming alignment.
   - Files: `api/core/v1alpha1`, generated CRD, tests/docs.
   - Goal: replace `externalURL` / `inClusterURL` with `external` /
     `inCluster`, matching virtualization-style nested URL object.
   - Check: controller tests, CRD verify.
   - Status: done.

2. Gateway HTTP hardening.
   - Files: `images/controller/internal/dataplane/uploadsession`.
   - Goal: no-store/no-sniff response headers for upload URL surface.
   - Check: dataplane uploadsession tests.
   - Status: done.

3. Gateway RBAC hardening.
   - Files: `templates/upload-gateway/rbac.yaml`,
     `internal/adapters/k8s/storageaccounting`, `internal/bootstrap`.
   - Goal: remove Secret create from upload-gateway SA; controller ensures
     storage accounting Secret before gateway reservations need it.
   - Check: storageaccounting/bootstrap tests, helm render.
   - Status: done.

4. Security/API review evidence.
   - Files: current bundle notes.
   - Goal: record why status secret URL remains acceptable and what protects it.
   - Check: `git diff --check`.
   - Status: done.

5. Verify blocker cleanup.
   - Files: `internal/adapters/sourcefetch/ollama*.go`.
   - Goal: split oversized Ollama sourcefetch files tripping
     `lint-controller-size` without changing behavior.
   - Check: sourcefetch tests, `make verify`.
   - Status: done.

## Rollback point

Revert this bundle to restore previous status field names and response headers.

## Final validation

- `cd api && go test ./...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./...`
- `make lint-docs`
- `git diff --check`

## Evidence

- `cd api && go test ./...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./internal/adapters/k8s/storageaccounting ./internal/bootstrap ./internal/dataplane/uploadsession ./internal/adapters/k8s/uploadsession ./internal/domain/publishstate ./internal/application/publishobserve ./internal/controllers/catalogstatus`
- `cd images/controller && go test ./...`
- `make lint-docs`
- `make helm-template`
- `make kubeconform`
- `cd images/controller && go test ./internal/adapters/sourcefetch`
- `make verify`
