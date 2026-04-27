# Plan: Uploadsessionstate secret lifecycle collapse

## 1. Current phase

Работа относится к Phase 1 runtime hardening/codebase slimming. Изменение
затрагивает только controller K8s adapter internals for upload-session state.

## 2. Orchestration

Режим: `full`.

Причина: исходная задача широкая; read-only subagents уже сравнили hotspots and
recommended this package as a safe package-local cleanup.

Read-only subagents:

- `repo_architect` — выбрал `uploadsessionstate` как основной безопасный
  controller cleanup target and explicitly forbade cross-package secret-state
  helper.
- `backend_integrator` — выделил backend-only future candidates and confirmed
  they should not be mixed here.
- `integration_architect` — выделил larger runtime risks separately; this
  slice does not touch runtime byte paths.

## 3. Active Bundle Disposition

- `model-metadata-contract` — keep. Отдельный active workstream по public
  metadata/status contract and future `ai-inference`.
- `runtimehealth-workload-kind-collapse` — archived as rejected to
  `plans/archive/2026/runtimehealth-workload-kind-collapse`.
- `uploadsessionstate-secret-lifecycle-collapse` — current. Единственный write
  scope: `uploadsessionstate` package and adjacent tests.

## 4. Slice 1. Lifecycle mutators

Цель:

- move probe, multipart-clear and terminal mutation semantics into secret
  lifecycle mutators;
- make `Client` call those mutators and keep only load/update plumbing;
- preserve persisted Secret keys and phases.

Файлы:

- `images/controller/internal/adapters/k8s/uploadsessionstate/client.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/secret.go`
- optional adjacent lifecycle file.
- `images/controller/internal/adapters/k8s/uploadsessionstate/secret_test.go`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession`
- `cd images/controller && go test -count=3 ./internal/adapters/k8s/uploadsessionstate`
- `cd images/controller && go test -race ./internal/adapters/k8s/uploadsessionstate`
- `git diff --check`

Evidence:

- Added `lifecycle.go` as the package-local home for probe, multipart-clear
  and terminal secret mutation semantics.
- `Client` now delegates `SaveProbe`, `ClearMultipart`, `MarkFailed` and
  `MarkAborted` to secret lifecycle mutators and remains get/update plumbing.
- Reused `setRuntimePhase` for multipart/probing/publishing/completed
  transitions to keep failure/staged-handle clearing policy in one place.
- Added regression coverage for probe preservation after multipart cleanup.
- Added replay-critical staged-handle coverage: `MarkPublishing` preserves the
  handle; `SaveMultipart`, `ClearMultipart`, `MarkCompleted`, `MarkFailed` and
  `MarkAborted` clear it; terminal no-op behavior preserves current state.
- LOC after implementation: `client.go` 161, `lifecycle.go` 100,
  `lifecycle_test.go` 149, `secret.go` 334, `secret_codec.go` 258,
  `secret_test.go` 218. This is a boundary cleanup, not a net LOC reduction
  slice.
- `cd images/controller && go test ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession`
- `cd images/controller && go test -count=3 ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession`
- `cd images/controller && go test -race ./internal/adapters/k8s/uploadsessionstate`

## 5. Slice 2. Review gate

Цель:

- проверить diff против TASK scope;
- убедиться, что no schema/API/RBAC/template/runtime entrypoint drift;
- archive current bundle or record next executable slice.

Проверки:

- `review-gate` — completed locally.
- final `reviewer` — completed; medium test coverage finding fixed before
  handoff.
- `make verify`

## 6. Rollback Point

Rollback: revert changes under
`images/controller/internal/adapters/k8s/uploadsessionstate/` and this plan
bundle. No public API, RBAC, Helm templates, runtime entrypoints, Secret keys
or upload protocol are in scope.
