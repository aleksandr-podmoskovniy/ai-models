# Plan: Controller backend prefix consolidation

## 1. Current phase

Работа относится к Phase 1 runtime hardening/codebase slimming. Изменение
затрагивает только controller-owned publication/cleanup dataplane internals.

## 2. Orchestration

Режим: `full`.

Причина: slice находится на границе publication backend runtime и cleanup
compatibility. Перед изменением кода были нужны read-only проверки boundaries.

Read-only subagents:

- `backend_integrator` — проверить DMCR/direct-upload-specific constraints,
  persisted cleanup handle compatibility and forbidden cross-image coupling.
- `repo_architect` — проверить package boundary, naming и anti-patchwork shape.

Subagent conclusions:

- `backend_integrator`: duplicated logic is only
  `reference -> repository path -> DMCR metadata prefix`. Cleanup-specific
  policy is not duplicated and must remain local: stored
  `RepositoryMetadataPrefix` wins, fallback derive from `Backend.Reference`
  stays for upgrade compatibility. Do not change cleanup handle schema, do not
  move into `support/cleanuphandle`, do not import publishworker and
  artifactcleanup into each other.
- `repo_architect`: use a narrow controller-local dataplane package rather than
  a repo-wide helper. `cleanuphandle` owns persisted annotation schema, not
  backend object-storage layout. Keep `backendSourceMirrorPrefix` local to
  artifactcleanup.

## 3. Active Bundle Disposition

- `model-metadata-contract` — keep. Отдельный active workstream по public
  metadata/status contract and future `ai-inference`.
- `dmcr-gc-s3-consolidation` — archived to
  `plans/archive/2026/dmcr-gc-s3-consolidation`. DMCR slice completed,
  reviewed and should not remain active while controller work continues.
- `controller-backend-prefix-consolidation` — current. Единственный write
  scope: controller dataplane backend prefix helper and adjacent tests.

## 4. Slice 1. Shared controller-local helper

Цель:

- add narrow `internal/dataplane/backendprefix` package;
- move pure repository metadata prefix derivation there;
- fail closed for empty or registry-less references.

Файлы:

- `images/controller/internal/dataplane/backendprefix/prefix.go`
- `images/controller/internal/dataplane/backendprefix/prefix_test.go`

Проверки:

- `cd images/controller && go test ./internal/dataplane/backendprefix`

Evidence:

- `RepositoryMetadataPrefixFromReference` owns only pure string derivation.
- `RepositoryPathFromReference` rejects registry-less references using Docker
  registry-component semantics.

## 5. Slice 2. Publish and cleanup integration

Цель:

- publish path uses shared helper when writing cleanup handle backend metadata;
- cleanup path keeps stored-prefix-first compatibility and uses shared helper
  only for fallback;
- source mirror cleanup prefix remains local to artifactcleanup.

Файлы:

- `images/controller/internal/dataplane/publishworker/support.go`
- `images/controller/internal/dataplane/artifactcleanup/backend_prefix.go`
- adjacent tests.

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/dataplane/backendprefix`
- targeted `go test -count=3` over backend prefix tests.
- `cd images/controller && go test -race ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/dataplane/backendprefix`
- `git diff --check`

Evidence:

- `publishworker` now calls `backendprefix.RepositoryMetadataPrefixFromReference`
  when writing backend cleanup metadata.
- `artifactcleanup` keeps stored-prefix-first compatibility and uses the shared
  helper only for legacy handles without `RepositoryMetadataPrefix`.
- Added cleanup run regression proving a derived backend repository prefix is
  pruned when the stored field is absent.
- Added helper table tests for digest, tag, registry port, whitespace, empty
  and registry-less references.
- LOC after implementation: `backendprefix/prefix.go` 70,
  `backendprefix/prefix_test.go` 91,
  `artifactcleanup/backend_prefix.go` 53,
  `artifactcleanup/backend_prefix_test.go` 75,
  `artifactcleanup/run_test.go` 128,
  `publishworker/support.go` 79.
- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/dataplane/backendprefix`
- `cd images/controller && go test -count=3 -run 'TestBuildBackendResultSetsRepositoryMetadataPrefix|TestAttachBackendSourceMirror|TestBackendRepositoryMetadataPrefixFallsBackToReference|TestBackendRepositoryMetadataPrefixPrefersStoredValue|TestBackendObjectStoragePrefixesIncludesSourceMirror|TestRunPrunesBackendRepositoryMetadataPrefix|TestRunPrunesDerivedBackendRepositoryMetadataPrefix|TestRunPrunesSourceMirrorPrefix|TestRepositoryMetadataPrefixFromReference' ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/dataplane/backendprefix`
- `cd images/controller && go test -race ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/dataplane/backendprefix`
- `git diff --check`
- Reviewer finding closed: parser now rejects URL schemes, path traversal,
  empty path segments and backslash paths before deriving a cleanup prefix.

## 6. Slice 3. Review gate

Цель:

- проверить diff против TASK scope;
- убедиться, что active bundles не содержат historical log;
- зафиксировать next executable slice or archive current bundle.

Проверки:

- `review-gate` — completed locally.
- final `reviewer` — completed; one medium parser finding and one low plan
  hygiene finding were fixed before archive.

Evidence:

- No API/RBAC/template/runtime-entrypoint drift in scoped diff.
- Cleanup handle stored-prefix compatibility preserved.
- Scoped files remain under LOC<350.
- Current bundle archived after this review instead of remaining as a
  historical active log.

## 7. Rollback Point

Rollback: revert `images/controller/internal/dataplane/backendprefix/` and the
bounded imports/tests in `publishworker` and `artifactcleanup`. No API, RBAC,
Helm templates, runtime entrypoints, cleanup handle schema or DMCR key layout
are in scope.
