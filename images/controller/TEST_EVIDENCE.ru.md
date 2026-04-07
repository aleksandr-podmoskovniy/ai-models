# Test Evidence

Этот файл — единственный канонический inventory decision-coverage для
`images/controller`.

Мы больше не держим локальные `BRANCH_MATRIX.ru.md` в части пакетов и не
размазываем одинаковый паттерн по дереву. Evidence остаётся одной точкой
правды рядом с controller runtime.

## `internal/domain/publishstate`

- Decision surface:
  - publication phases и terminal semantics
  - upload status equality
  - worker/session observation decisions
  - status/condition projection
- Primary evidence:
  - `operation_test.go`
  - `runtime_decisions_test.go`
  - `status_test.go`
- Residual gaps:
  - replay/retry и malformed runtime result остаются на
    `controllers/catalogstatus`, а не в domain

## `internal/application/publishplan`

- Decision surface:
  - execution-mode selection
  - source-worker planning
  - upload-session issuance policy
- Primary evidence:
  - `start_publication_test.go`
  - `plan_source_worker_test.go`
  - `issue_upload_session_test.go`
- Residual gaps:
  - source acceptance и reconcile short-circuit rules остаются adapter-local в
    `controllers/catalogstatus`
  - concrete replay/IO races остаются в adapter package tests

## `internal/adapters/modelformat`

- Decision surface:
  - source-agnostic input-format validation policy
  - automatic format detection when `spec.inputFormat` is empty
  - file allowlist / rejectlist policy
  - benign-extra stripping before `ModelPack` packaging
  - required file and required asset enforcement
  - single-file `GGUF` acceptance alongside archive-based inputs
- Primary evidence:
  - `validation_test.go`
- Residual gaps:
  - future extra formats beyond `Safetensors` and `GGUF` will need their own
    rule families and tests

## `internal/adapters/modelprofile`

- Decision surface:
  - endpoint type mapping from `task`
  - precision and quantization inference
  - `parameterCount` estimation
  - `minimumLaunch` estimation
  - format-specific profile extraction for `Safetensors` and `GGUF`
- Primary evidence:
  - `common/profile_test.go`
  - `safetensors/profile_test.go`
  - `gguf/profile_test.go`
  - `domain/publishstate/status_test.go`
- Residual gaps:
  - future richer formats will need their own profile adapters

## `internal/application/deletion`

- Decision surface:
  - cleanup finalizer policy
  - delete-time cleanup decision table
- Primary evidence:
  - `ensure_cleanup_finalizer_test.go`
  - `finalize_delete_test.go`
- Residual gaps:
  - adapter-level create-race/status replay остаются в
    `controllers/catalogcleanup` tests
