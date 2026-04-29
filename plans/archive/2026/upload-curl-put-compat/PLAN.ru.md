# План: Upload через curl -T

## Current phase

Этап 1/2 boundary: publication/runtime baseline уже имеет upload session,
staging и publication handoff. Этот slice улучшает user-facing upload UX без
смены API `Model` / `ClusterModel`.

## Orchestration

`solo`: изменение узкое, риск понятен, delegation не требуется. Subagents не
используются, потому что пользователь не просил delegation в этом запросе, а
граница ограничена upload gateway + docs.

## Active bundle disposition

- `capacity-cache-admission-hardening` — keep: отдельный storage/cache
  admission workstream.
- `live-e2e-ha-validation` — keep: текущий e2e/HA runbook и evidence.
- `observability-signal-hardening` — keep: отдельный logs/metrics workstream.
- `pre-rollout-defect-closure` — keep: исторически активный defect-closure
  workstream, не трогаю в этом slice.
- `public-docs-virtualization-style` — keep: публичная документация, этот slice
  обновляет только upload UX sections и не архивирует весь bundle.
- `ray-a30-ai-models-registry-cutover` — keep: отдельный workload cutover.
- `source-capability-taxonomy-ollama` — keep: отдельный taxonomy/Ollama
  source workstream.

## Slices

### 1. Runtime API

- Добавить `PUT /v1/upload/<sessionID>` в `uploadsession` handler.
- Повторно использовать admission, reservation, staging and `MarkUploaded`.
- Поддержать `?filename=`, `X-Upload-Filename`, `Content-Disposition` и
  bounded magic-byte inference.

Проверка:

```bash
cd images/controller && go test ./internal/dataplane/uploadsession
```

### 2. Tests

- Покрыть happy path `curl -T` style upload.
- Покрыть отказ без `Content-Length` при включённых reservations.
- Убедиться, что invalid magic остаётся admission error.

Проверка:

```bash
cd images/controller && go test ./internal/dataplane/uploadsession
```

### 3. Docs

- Обновить user guide/examples/FAQ EN/RU.
- Основной пользовательский пример должен быть `curl -T`.
- Multipart оставить как низкоуровневый resumable contract.

Проверка:

```bash
make lint-docs
```

## Rollback point

Изменение можно откатить удалением `PUT` handler/tests/docs. Multipart upload
API и существующий publication path не меняются.

## Final validation

```bash
cd images/controller && go test ./internal/dataplane/uploadsession
make lint-docs
git diff --check
```

## Evidence

- `cd images/controller && go test ./internal/dataplane/uploadsession` passed.
- `cd images/controller && go test ./...` passed.
- `make lint-docs` passed.
- `make deadcode` passed.
- `git diff --check` passed.
