# План: upload URL по паттерну virtualization

## 1. Current phase

Этап 1: publication/runtime baseline. Slice упрощает upload UX без изменения
publication backend semantics.

## 2. Orchestration

`solo`: изменение узкое и прямо задано требованием сравнять UX с
virtualization. Сабагенты не запускаются в этом turn, потому что пользователь
не просил delegation; риск закрывается явным task bundle, совместимостью старого
header path и узкими тестами.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep: следующий executable workstream после новой
  выкладки.
- `observability-signal-hardening` — keep: отдельный observability workstream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельный workload cutover
  workstream.
- `upload-virtualization-secret-url` — current: закрывается этим slice.

## 4. Slices

### Slice 1. Status secret URL

- Цель: строить `status.upload.*URL` со встроенным upload token.
- Файлы: `images/controller/internal/adapters/k8s/uploadsession/*`.
- Проверка: package tests для K8s uploadsession adapter.
- Артефакт: handle status содержит direct secret URL.

### Slice 2. Gateway auth compatibility

- Цель: gateway принимает `PUT /v1/upload/<session>/<token>` и
  `POST /v1/upload/<session>/<token>/<action>`, сохраняя bearer-header path.
- Файлы: `images/controller/internal/dataplane/uploadsession/*`.
- Проверка: route/auth/direct upload tests.
- Артефакт: прямой `curl -T "$UPLOAD_URL"` работает без header.

### Slice 3. Docs/API notes

- Цель: убрать инструкции с ручным чтением token Secret из пользовательского
  пути.
- Файлы: `docs/*`, `api/README.md`.
- Проверка: `make lint-docs`, `git diff --check`.
- Артефакт: документация объясняет secret URL, TTL и совместимость.

## 5. Rollback point

Можно откатить только изменения этого bundle: status URL вернётся к
`/v1/upload/<session>`, gateway продолжит старую bearer-header авторизацию.

## 6. Final validation

- `gofmt` для изменённых Go-файлов.
- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession`
- `make lint-docs`
- `git diff --check`
