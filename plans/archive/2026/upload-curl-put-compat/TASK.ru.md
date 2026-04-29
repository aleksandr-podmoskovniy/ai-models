# Upload через curl -T

## Контекст

Upload path уже работает через controller-owned upload session и multipart API:
`probe -> init -> parts -> complete`. Это корректно для будущего SDK/CLI, но
для оператора и пользователя UX слишком низкоуровневый.

В проектах DKP/virtualization ожидаемый простой сценарий выглядит как:

```bash
curl -T ./model.gguf -H "$AUTH_HEADER" "$UPLOAD_URL"
```

где URL и секретная авторизация выдаются платформой, а пользователь не пишет
multipart orchestration руками.

## Постановка задачи

Добавить совместимый простой upload path в текущий upload gateway:

- `PUT /v1/upload/<sessionID>` принимает тело файла напрямую;
- существующий multipart API остаётся без изменений для resumable клиентов;
- загрузка проходит ту же валидацию формата, storage reservation и staged
  handoff, что и multipart path;
- публичные docs показывают простой путь как основной.

## Scope

- HTTP API upload-session runtime.
- Тесты upload-session dataplane.
- Пользовательская документация upload examples/FAQ.
- Task bundle и evidence.

## Non-goals

- Не делать полноценный SDK/CLI.
- Не менять `Model` / `ClusterModel` API.
- Не менять upload-token Secret contract.
- Не удалять multipart API.
- Не добавлять resumable upload semantics поверх `curl -T`.

## Затрагиваемые области

- `images/controller/internal/dataplane/uploadsession`
- `docs/USER_GUIDE*.md`
- `docs/EXAMPLES*.md`
- `docs/FAQ*.md`
- `plans/active/upload-curl-put-compat`

## Критерии приёмки

- `PUT /v1/upload/<sessionID>` с валидным Bearer token принимает GGUF/archive
  payload и переводит сессию в `uploaded`.
- При включённом storage capacity limit `PUT` требует `Content-Length` и
  возвращает понятный отказ до записи байтов, если размер неизвестен или не
  помещается.
- Невалидный формат отклоняется тем же admission contract, что и `/probe`.
- Multipart API продолжает проходить существующие тесты.
- Docs описывают `curl -T` как основной путь, а multipart как низкоуровневый
  resumable contract.

## Риски

- Долгая single-request загрузка может занимать upload-session handler дольше,
  чем короткие multipart control calls. Это приемлемо для простого UX path;
  для тяжёлых production-клиентов остаётся multipart/CLI slice.
- Для `curl -T "$FILE" "$URL"` имя файла не передаётся HTTP-протоколом. Gateway
  должен выводить тип по magic bytes или принимать `?filename=` /
  `X-Upload-Filename`.
