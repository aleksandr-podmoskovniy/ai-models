## 1. Current phase

Этап 2. `Model` / `ClusterModel`: задача меняет controller publication runtime,
status UX, values/runtime wiring и operator-facing documentation.

## 2. Orchestration

`solo`

Текущий continuation slice узкий и bounded: live byte path уже streaming-first,
поэтому нужно убрать stale publication work-volume shell и синхронизировать
runtime/docs/tests без перепроектирования всего publish pipeline.

## 3. Slices

### Slice 1. Зафиксировать live byte-path и target scenario

Цель:

- убрать неопределённость, что у нас сейчас реально происходит:
  source mirror/object-source publish уже живой, а publication work PVC стал
  shell drift.

Файлы/каталоги:

- `plans/active/publication-storage-hardening/*`
- при необходимости `images/controller/README.md`
- при необходимости `docs/CONFIGURATION*.md`

Проверки:

- `rg -n "raw ingest|source mirror|publicationRuntime|workVolume|ephemeral-storage" images/controller docs openapi templates`

Артефакт результата:

- согласованный target scenario в bundle:
  no publication work volume for sourceworker + smaller bounded
  `ephemeral-storage` + docs cleanup.

### Slice 2. Удалить publication work-volume shell из sourceworker runtime

Цель:

- убрать stale work volume/TMPDIR/PVC contract из controller runtime,
  sourceworker adapter и render surface.

Файлы/каталоги:

- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- возможно `images/controller/internal/adapters/k8s/workloadpod/*`

Проверки:

- `go test ./images/controller/cmd/ai-models-controller/...`
- `go test ./images/controller/internal/adapters/k8s/sourceworker/...`

Артефакт результата:

- sourceworker runtime, pod render и controller config больше не требуют
  work-volume options для live publish path.

### Slice 3. Удалить stale PVC/template/OpenAPI surface

Цель:

- убрать из render/docs старый publication workspace contract.

Файлы/каталоги:

- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- rendered module без `ai-models-publication-work` PVC и без work-volume
  validation shell.

### Slice 4. Синхронизировать docs/evidence

Цель:

- убрать PVC-first/`workloadpod` narrative и зафиксировать текущий exact byte
  path: successful publish больше не требует local workspace.

Файлы/каталоги:

- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- docs и evidence, где operator-facing storage contract совпадает с live
  streaming implementation.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: target scenario и naming drift уже
зафиксированы, но runtime behavior ещё не изменён.

После Slice 2 rollback ещё возможен без structural drift: runtime shell уже
перестанет требовать work volume, но render/docs cleanup ещё не обязаны быть
завершены.

## 5. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
