## 1. Current phase

Этап 1, corrective hardening внутри publication/runtime baseline.

Это не новый distribution workstream, а точечное исправление security/UX
контракта для уже landed upload-session path.

## 2. Orchestration

`solo`

Причина:

- задача узкая и локализована вокруг одного auth-контракта;
- архитектурная цель ясна: убрать токен из URL и удержать рабочий Bearer-only
  path без разрастания решения.

## 3. Slices

### Slice 1. Зафиксировать новый status/upload contract

Цель:

- перевести `ModelUploadStatus` на URL без токена и отдельное Bearer-поле.

Файлы/каталоги:

- `api/core/v1alpha1/*`
- `api/README.md`

Проверки:

- `go test ./api/...`

Артефакт результата:

- новый публичный status-контракт upload-session.

### Slice 2. Перевести controller-side uploadsession shaping и recovery

Цель:

- строить status по новому контракту и уметь повторно извлекать токен без
  query string.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/ports/publishop/*`

Проверки:

- `go test ./internal/adapters/k8s/uploadsession ./internal/domain/publishstate ./internal/ports/publishop`

Артефакт результата:

- controller-side upload-session handle и status projection без `?token=`.

### Slice 3. Перевести upload-session runtime на Bearer-only auth

Цель:

- сделать Bearer-заголовок единственным рабочим способом аутентификации.

Файлы/каталоги:

- `images/controller/internal/dataplane/uploadsession/*`

Проверки:

- `go test ./internal/dataplane/uploadsession`

Артефакт результата:

- upload-gateway, который валидирует только Bearer-токен.

### Slice 4. Синхронизировать projection tests и docs

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

- согласованный docs/tests surface без URL token narrative.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: целевой контракт уже зафиксирован,
но runtime ещё не переведён.

## 5. Final validation

- `gofmt -w` на изменённых Go-файлах
- `go test ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./internal/domain/publishstate ./internal/application/publishobserve ./internal/controllers/catalogstatus ./internal/ports/publishop`
