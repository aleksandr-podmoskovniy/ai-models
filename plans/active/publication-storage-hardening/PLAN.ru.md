## 1. Current phase

Этап 2. `Model` / `ClusterModel`: задача меняет controller publication runtime,
status UX, values/runtime wiring и operator-facing documentation.

## 2. Orchestration

`solo`

Текущий continuation slice узкий и bounded: перевести defaults на PVC, убрать
misleading operator-facing `raw ingest` wording и синхронизировать docs/tests
без перепроектирования всего byte-path.

## 3. Slices

### Slice 1. Зафиксировать live byte-path и target scenario

Цель:

- убрать неопределённость, что у нас сейчас реально происходит:
  source mirror, local workspace, raw-stage naming drift и storage contract.

Файлы/каталоги:

- `plans/active/publication-storage-hardening/*`
- при необходимости `images/controller/README.md`
- при необходимости `docs/CONFIGURATION*.md`

Проверки:

- `rg -n "raw ingest|source mirror|publicationRuntime|workVolume|ephemeral-storage" images/controller docs openapi templates`

Артефакт результата:

- согласованный target scenario в bundle:
  fail-fast + explicit operator guidance + source-mirror naming cleanup.

### Slice 2. Перевести default workspace на PVC-backed path

Цель:

- убрать implicit default `EmptyDir 50Gi` и сделать PVC-backed publication
  workspace platform default с default StorageClass fallback.

Файлы/каталоги:

- `images/controller/cmd/ai-models-controller/*`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/publication-work-pvc.yaml`

Проверки:

- `go test ./images/controller/cmd/ai-models-controller/...`
- `make helm-template`
- `make kubeconform`

Артефакт результата:

- controller CLI/runtime, OpenAPI и templates согласованно считают PVC-backed
  workspace default path.

### Slice 3. Ввести size-aware workspace guardrail

Цель:

- до старта или на раннем owner/runtime этапе deterministically отлавливать
  remote source, который не помещается в configured local workspace contract.

Файлы/каталоги:

- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/domain/publishstate/*`
- возможно `images/controller/internal/adapters/sourcefetch/*`

Проверки:

- `go test ./images/controller/internal/application/publishplan/...`
- `go test ./images/controller/internal/controllers/catalogstatus/...`
- `go test ./images/controller/internal/domain/publishstate/...`

Артефакт результата:

- owner/status path, который выдаёт clear failure/warning surface вместо
  неявной runtime ставки на `EmptyDir 50Gi`.

### Slice 4. Упростить remote path around source mirror

Цель:

- удалить или изолировать мёртвый remote raw-stage path и выровнять audit /
  provenance naming по фактическому `.mirror` source of truth.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `go test ./images/controller/internal/dataplane/publishworker/...`
- `go test ./images/controller/internal/adapters/sourcefetch/...`
- `go test ./images/controller/internal/controllers/catalogstatus/...`

Артефакт результата:

- runtime и evidence без misleading `raw ingest` wording там, где publish
  реально идёт через source mirror.

### Slice 5. Синхронизировать values, templates и docs

Цель:

- сделать storage expectations видимыми и настраиваемыми без чтения кода.

Файлы/каталоги:

- `openapi/values.yaml`
- `templates/controller/*`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`
- `README.ru.md`
- `images/controller/README.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- docs и values, где оператор заранее видит:
  - что `EmptyDir` = node ephemeral storage;
  - когда нужен PVC;
  - какой default limit и чем он опасен для крупных моделей.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: target scenario и naming drift уже
зафиксированы, но runtime behavior ещё не изменён.

После Slice 2 rollback ещё возможен без structural drift: defaults уже станут
PVC-backed, но guardrail и byte-path cleanup ещё не обязаны быть завершены.

## 5. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
