## 1. Current phase

Этап 1, corrective hardening внутри publication/runtime baseline.

Это не новый distribution workstream, а упрочнение публичного upload UX для
`source.upload`.

## 2. Orchestration

`solo`

Причина:

- задача архитектурно ясна и локализована вокруг одного публичного status
  контракта;
- изменения затрагивают несколько пакетов, но решение остаётся узким:
  controller-owned upload-session state -> top-level `status.progress`.

## 3. Slices

### Slice 1. Зафиксировать публичный `status.progress` контракт

Цель:

- добавить top-level progress field и printcolumn по образцу
  `virtualization`.

Файлы/каталоги:

- `api/core/v1alpha1/*`
- `api/README.md`

Проверки:

- `go test ./...` в `api/`

Артефакт результата:

- публичный API с единым `status.progress`.

### Slice 2. Довести upload-session state до вычислимого процента

Цель:

- принять `sizeBytes` в `probe`, сохранить его в session state и вычислять
  progress из persisted multipart parts.

Файлы/каталоги:

- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/ports/publishop/*`

Проверки:

- `go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession ./internal/ports/publishop`

Артефакт результата:

- controller-visible upload-session handle с честным процентом локальной
  загрузки; финальный вариант не зависит от отдельного client-side polling
  против upload gateway, потому что controller сам синхронизирует multipart
  uploaded parts из staging перед status projection.

### Slice 3. Протянуть progress в status projection и docs

Цель:

- проецировать upload progress в `Model.status.progress` и убрать drift между
  кодом, тестами и документацией.

Файлы/каталоги:

- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `go test ./internal/application/publishobserve ./internal/domain/publishstate ./internal/controllers/catalogstatus`

Артефакт результата:

- единый публичный progress UX для локальной загрузки.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: публичный API уже расширен, но
runtime-логика ещё не переведена.

## 5. Final validation

- `gofmt -w` на изменённых Go-файлах
- `go test ./...` в `api/`
- `go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession ./internal/application/publishobserve ./internal/domain/publishstate ./internal/controllers/catalogstatus ./internal/ports/publishop` в `images/controller/`
- `bash api/scripts/verify-crdgen.sh`
