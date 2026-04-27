# Plan: DMCR GC S3 helper consolidation

## 1. Current phase

Работа относится к Phase 1/2 runtime hardening: DMCR уже является live
publication backend, а GC должен оставаться replay-safe и проверяемым.

## 2. Orchestration

Режим: `full`.

Причина: задача затрагивает storage/GC/HA recovery path. Перед изменением кода
нужны read-only проверки границ.

Read-only subagents:

- `repo_architect` — проверить, что helper consolidation не создаёт новый
  generic/patchwork boundary и остаётся package-local.
- `integration_architect` — проверить storage/HA semantics: pagination,
  multipart cleanup, lease/gate/quorum и fail-closed behavior.
- `backend_integrator` — проверить DMCR/direct-upload-specific constraints и
  не допустить cross-image coupling с controller `modelpack/oci`.

Subagent conclusions:

- `repo_architect`: do not create a generic pagination abstraction across
  object, multipart-upload and part list paths; marker shapes and error
  semantics differ. The safest pure LOC cut is a private multipart target
  argument normalizer in `storage_s3_multipart.go`.
- `integration_architect`: the important correctness gap is missing
  forward-progress validation in S3 pagination loops. Keep distinct pagers for
  objects, multipart uploads and parts, each owning its exact AWS cursor type
  and failing closed on truncated pages with empty or unchanged next cursor.
  Keep caller `ctx`, lease/gate/quorum boundaries and destructive prefix
  semantics unchanged.
- `backend_integrator`: no cross-image helper with controller S3/upload code,
  no GC policy based on controller `DirectUploadState`, and no public/status
  leakage of `_ai_models/direct-upload`, sealed metadata suffixes, multipart
  upload IDs or DMCR root-directory conventions.

## 3. Active Bundle Disposition

- `model-metadata-contract` — keep. Отдельный active workstream по public
  metadata/status contract и future `ai-inference`; этот slice его не меняет.
- `codebase-slimming-pass` — archived to
  `plans/archive/2026/codebase-slimming-pass`. Bundle завершён как historical
  handoff после slices 1-22 и полного `make verify`; дальнейшая работа должна
  идти через компактные continuation bundles.
- `dmcr-gc-s3-consolidation` — current. Единственный write scope:
  package-local DMCR GC S3 helper consolidation.

## 4. Slice 1. Read-only review and exact cut

Цель:

- зафиксировать конкретный повторяющийся S3 pagination/helper skeleton;
- явно перечислить behavior, который нельзя менять.

Файлы:

- `images/dmcr/internal/garbagecollection/storage_s3.go`
- `images/dmcr/internal/garbagecollection/storage_s3_multipart.go`
- package-local tests.

Проверки:

- read-only subagent conclusions recorded in this plan.

## 5. Slice 2. Package-local implementation

Цель:

- keep object, multipart-upload and multipart-part pagination as distinct
  package-local helpers;
- add forward-progress guards for truncated pages with missing or repeated next
  cursors;
- схлопнуть только доказуемый multipart target validation duplication;
- сохранить current error messages where useful for operations;
- не добавлять новый package.

Файлы:

- same bounded DMCR GC files.

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- targeted `go test -count=3` по S3 transport, inventory and pagination tests.
- `cd images/dmcr && go test -race ./internal/garbagecollection`
- `git diff --check`

Evidence:

- `ForEachObjectInfo` and `DeletePrefix` now share a package-local object page
  traversal helper while keeping read-prefix and destructive-prefix helpers
  separate.
- `ForEachMultipartUpload` and `CountMultipartUploadParts` now use distinct
  multipart upload / part page helpers with exact AWS cursor types.
- Truncated pages without next cursor or with repeated cursor return an error
  instead of looping until context timeout.
- `CountMultipartUploadParts` and `AbortMultipartUpload` share
  `cleanMultipartUploadTarget`; `NoSuchUpload` semantics are unchanged.
- Added AWS SDK transport-stub tests for object continuation tokens,
  multipart upload markers, part markers, repeated/missing cursor failures,
  `NoSuchUpload`, and `DeleteObjects` partial errors.
- LOC after implementation: `storage_s3.go` 299,
  `storage_s3_multipart.go` 196, `storage_s3_pagination_test.go` 254,
  `storage_s3_transport_test.go` 154.
- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/dmcr && go test -count=3 -run 'TestS3PrefixStore|TestDiscoverDirectUploadMultipartInventory|TestBuildReport.*Multipart|TestDeleteStalePrefixesAbortsMultipartUploads|TestNewS3PrefixStoreSupportsAWSCABundleEnv' ./internal/garbagecollection`
- `cd images/dmcr && go test -race ./internal/garbagecollection`
- `cd images/dmcr && go test ./internal/garbagecollection/...`
- `git diff --check`
- `make verify`

## 6. Slice 3. Review gate

Цель:

- проверить diff против scope;
- убедиться, что active bundle не стал историческим логом;
- зафиксировать next executable slice или закрыть bundle.

Проверки:

- `review-gate` — completed locally.
- final `reviewer` — completed, no high-severity runtime findings.

Evidence:

- Reviewer found no API/RBAC/template/runtime-entrypoint changes, no
  cross-image helper sharing and no controller/DMCR coupling.
- Reviewer requested extra cursor-guard tests; added coverage for repeated
  object continuation token, same-key multipart upload page without
  `NextUploadIdMarker`, and missing `NextPartNumberMarker`.
- Active bundle shortlist was reconciled with `model-metadata-contract`: only
  `model-metadata-contract` and `dmcr-gc-s3-consolidation` remain active.
- This bundle remains active only because next executable slice is defined
  below.

## 7. Next executable slice

### Controller backend repository prefix helper consolidation

Цель:

- collapse controller-local duplication around backend repository prefix
  derivation between publish result creation and artifact cleanup fallback;
- keep the change inside controller runtime boundaries;
- do not change cleanup handle schema or DMCR GC internals.

Файлы:

- `images/controller/internal/dataplane/publishworker/support.go`
- `images/controller/internal/dataplane/artifactcleanup/backend_prefix.go`
- adjacent package-local tests.

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup`
- targeted repository-prefix tests.
- `git diff --check`

## 8. Rollback Point

Rollback: revert this bundle plus the bounded changes under
`images/dmcr/internal/garbagecollection/`. No public API, RBAC, Helm templates,
runtime entrypoints, request schema, S3 key format or cleanup phases are in
scope.
