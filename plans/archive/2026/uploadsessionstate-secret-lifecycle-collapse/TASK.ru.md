# Uploadsessionstate secret lifecycle collapse

## 1. Заголовок

Убрать lifecycle mutation policy из Kubernetes client layer
`uploadsessionstate` и оставить `Client` thin CRUD plumbing.

## 2. Контекст

`uploadsessionstate` хранит upload-session state в Kubernetes Secret. Сейчас
часть state transitions живёт в secret mutators (`MarkUploadedSecret`,
`MarkPublishingSecret`, `MarkCompletedSecret`), а часть размазана по
`Client.SaveProbe` / `Client.ClearMultipart` / terminal wrappers.

Это package-local adapter cleanup. Public API, RBAC, templates, upload protocol
и persisted Secret schema не меняются.

## 3. Постановка задачи

- Перенести probe/multipart-clear/terminal mutation semantics из `Client` в
  package-local secret lifecycle mutators.
- Оставить `Client` ответственным только за get/update/retry-on-conflict.
- Сохранить persisted keys, phases, cleanup handle semantics and
  idempotency/terminal behavior.

## 4. Scope

- `images/controller/internal/adapters/k8s/uploadsessionstate/client.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/secret.go`
- new adjacent package-local lifecycle file if needed.
- adjacent tests only for lifecycle regression.
- `plans/active/uploadsessionstate-secret-lifecycle-collapse/`

## 5. Non-goals

- Не создавать shared secret-state abstraction across packages.
- Не менять `directuploadstate`.
- Не менять upload-session HTTP API, controller API, RBAC or Helm templates.
- Не менять Secret keys, annotations, phase names or cleanup handle schema.

## 6. Критерии приёмки

- `Client` no longer owns probe/multipart-clear/terminal mutation logic.
- Secret lifecycle mutators preserve existing behavior.
- Files stay LOC<350.
- Package tests pass.
- Repeated and race tests pass for touched package.
- `git diff --check` passes.
- `make verify` passes before handoff if feasible.

## 7. Риски

- Ошибка phase transition can break upload replay after runtime restart.
- Wrong staged-handle clearing can leak or prematurely drop cleanup state.
- Over-generalizing can create a fake shared secret-state layer.
