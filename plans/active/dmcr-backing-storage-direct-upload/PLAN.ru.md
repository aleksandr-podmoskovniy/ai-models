## 1. Current phase

Этап 2. Это bounded continuation publication/distribution workstream: public
source intent не меняется, но меняется upload contract между native `OCI`
publisher и internal `DMCR`.

## 2. Orchestration

`full`

Причина:

- задача меняет больше одной области репозитория;
- она одновременно затрагивает storage, registry runtime, values/OpenAPI,
  controller wiring и operator-facing docs;
- по правилам репозитория сюда просится read-only delegation, но в текущей
  сессии инструмент delegation ограничен системной политикой, поэтому
  архитектурная дисциплина фиксируется прямо в bundle и финальном review.

## 3. Slices

### Slice 1. Зафиксировать новый upload contract и wiring surface

Цель:

- убрать cluster-level upload mode и оставить один канонический direct-upload
  contract для тяжёлых blob-слоёв на всём controller/runtime пути.

Файлы/каталоги:

- `plans/active/dmcr-backing-storage-direct-upload/*`
- `openapi/values.yaml`
- `openapi/config-values.yaml`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/dataplane/publishworker/*`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller ./internal/adapters/k8s/sourceworker ./internal/dataplane/publishworker`

Артефакт результата:

- runtime shell больше не знает альтернативного upload mode;
- publish-worker получает только helper endpoint и trust wiring.

### Slice 2. Добавить `DMCR`-owned direct upload helper

Цель:

- дать `DMCR` внутренний HTTPS API для direct multipart upload в backing
  storage без in-memory-only session coupling.

Файлы/каталоги:

- `images/dmcr/cmd/*`
- `images/dmcr/internal/*`
- `images/dmcr/werf.inc.yaml`
- `templates/dmcr/*`

Проверки:

- `cd images/dmcr && go test ./...`

Артефакт результата:

- helper умеет:
  - открыть upload session;
  - presign upload part;
  - list uploaded parts;
  - complete multipart upload;
  - materialize repository blob link;
  - abort session.

### Slice 3. Переключить native `OCI` publisher на canonical direct transport

Цель:

- убрать runtime branch по upload mode и сделать direct blob upload
  каноническим transport для тяжёлых layer blobs.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`

Артефакт результата:

- canonical heavy-layer path пишет blobs через helper/storage direct и не
  использует registry `PATCH`;
- `config` blob и `manifest` publish остаются на обычном registry path.

### Slice 4. Синхронизировать docs, evidence и render surface

Цель:

- честно описать новый byte path и не смешать acquisition mode с mandatory
  direct upload transport.

Файлы/каталоги:

- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `templates/controller/*`
- `templates/dmcr/*`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- render/docs/evidence синхронизированы с direct-only upload contract.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: obsolete mode surface уже снята,
но publisher transport ещё не везде дочищен.

После Slice 2 можно откатиться до старого publisher transport без public config
drift: helper уже существует, но native `OCI` publisher ещё можно вернуть на
старый blob path локально в коде, не меняя public config contract.

## 5. Final validation

- `cd images/dmcr && go test ./...`
- `cd images/controller && go test ./cmd/ai-models-controller ./internal/adapters/k8s/sourceworker ./internal/dataplane/publishworker ./internal/adapters/modelpack/oci`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
