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
  - replay races и persisted corruption остаются на `controllers/publishrunner`
    boundary tests, а не в domain

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
