## 1. Current phase

Этап 1, corrective hardening внутри publication/runtime baseline.

Это не новый distribution workstream, а исправление security/UX контракта для
уже landed upload-session path: raw bearer не должен быть частью публичного
`Model.status`.

## 2. Orchestration

`solo`

Причина:

- текущий implementation slice локализован вокруг upload-session auth/status;
- broad request про "весь проект" оформлен как audit/cleanup slice, где удалять
  можно только доказанно неиспользуемый код;
- архитектурная цель ясна: raw bearer живёт в owner-scoped Secret, публичный
  status хранит только URL, expiry, repository и Secret reference.

## 3. Slices

### Slice 1. Зафиксировать Secret-reference status/upload contract

Цель:

- перевести `ModelUploadStatus` на URL без токена и Secret reference вместо
  raw Bearer-поля.

Файлы/каталоги:

- `api/core/v1alpha1/*`
- `api/README.md`

Проверки:

- `go test ./api/...`

Артефакт результата:

- новый публичный status-контракт upload-session без raw bearer.

### Slice 2. Добавить token handoff Secret и перевести controller shaping

Цель:

- строить status по новому контракту;
- хранить raw authorization header в отдельном Secret;
- уметь повторно извлекать token из Secret, не используя публичный status.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/ports/publishop/*`

Проверки:

- `go test ./internal/adapters/k8s/uploadsession ./internal/domain/publishstate ./internal/ports/publishop`

Артефакт результата:

- controller-side upload-session handle и status projection без raw bearer.

### Slice 3. Перевести upload-session runtime на Bearer-only auth

Цель:

- сделать Bearer-заголовок единственным рабочим способом аутентификации.

Файлы/каталоги:

- `images/controller/internal/dataplane/uploadsession/*`

Проверки:

- `go test ./internal/dataplane/uploadsession`

Артефакт результата:

- upload-gateway, который валидирует только Bearer-токен.

### Slice 4. Cleanup owner-scoped token Secret

Цель:

- удалять token handoff Secret вместе с upload-session runtime state.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/controllers/catalogcleanup/*`

Проверки:

- `go test ./internal/adapters/k8s/uploadsession ./internal/controllers/catalogcleanup`

Артефакт результата:

- deletion/finalizer path не оставляет user-facing upload token Secret.

### Slice 5. Удалить legacy upload-token recovery и проверить usage

Цель:

- убрать compatibility recovery из `?token=` URL;
- удалить только доказанно неиспользуемый код в upload-session boundary.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/dataplane/uploadsession/*`

Проверки:

- `rg -n "\\?token=|tokenFromLegacyUploadURL|authorizationHeaderValue" api images/controller docs`
- `go test ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession`

Артефакт результата:

- upload-session boundary не содержит старый query-token/status-token fallback.

### Slice 6. Синхронизировать projection tests и docs

Цель:

- убрать drift между API, runtime и документацией.

Файлы/каталоги:

- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `go test ./internal/application/publishobserve ./internal/controllers/catalogstatus`

Артефакт результата:

- согласованный docs/tests surface без URL token/raw status narrative.

### Slice 7. Project cleanup audit

Цель:

- просмотреть проект на legacy/monolith/dead-code признаки и удалить только
  код, который подтверждён `rg`, `deadcode` или узкими тестами как
  неиспользуемый.

Файлы/каталоги:

- по результатам audit, не заранее широким списком.

Проверки:

- `make deadcode`
- targeted `rg` по legacy markers
- узкие `go test` по затронутым модулям

Артефакт результата:

- либо отдельный small cleanup diff, либо notes с причинами, почему найденные
  legacy markers пока являются documentation/test evidence, а не удаляемым
  runtime-кодом.

## 4. Rollback point

После Slice 2 можно безопасно остановиться: публичный status уже не несёт raw
bearer, а runtime session Secret остаётся hash-only.

## 5. Final validation

- `gofmt -w` на изменённых Go-файлах
- `go generate ./core/...` внутри `api`
- `go test ./...` внутри `api`
- `go test ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./internal/domain/publishstate ./internal/application/publishobserve ./internal/controllers/catalogstatus ./internal/controllers/catalogcleanup ./internal/ports/publishop` внутри `images/controller`
- `make deadcode`
- `make verify`
