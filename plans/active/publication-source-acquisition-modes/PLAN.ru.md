## 1. Current phase

Этап 2. Это bounded continuation publication/distribution workstream:
public `Model.spec` не меняется, но меняется cluster-level runtime contract
получения удалённых байтов перед публикацией в canonical internal `DMCR`.

## 2. Orchestration

`full`

Причина:

- задача меняет больше одной области репозитория;
- она затрагивает storage/runtime wiring, values/OpenAPI и operator-facing docs;
- она является implementation follow-up на архитектурный verdict из
  `phase2-model-distribution-architecture`, но не должна снова размыть его.

Примечание по исполнению:

- по правилам репозитория read-only subagent review здесь был бы обязателен;
- в текущей сессии инструмент delegation ограничен системной политикой, поэтому
  implementation идёт локально, а architectural discipline фиксируется прямо в
  bundle и финальном review.

## 3. Slices

### Slice 1. Зафиксировать runtime contract и провести knob через controller wiring

Цель:

- ввести cluster-level acquisition mode и провести его через
  controller/sourceworker/publish-worker shell.

Файлы/каталоги:

- `plans/active/publication-source-acquisition-modes/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller ./internal/adapters/k8s/sourceworker ./internal/controllers/catalogstatus`

Артефакт результата:

- runtime config умеет `mirror|direct`;
- `HuggingFace` sourceworker args зависят от режима, upload path — нет.

### Slice 2. Замкнуть publish-worker/sourcefetch boundary и доказать оба path

Цель:

- убедиться, что режим реально меняет acquisition path внутри publish-worker,
  а не только рендерит разные аргументы.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/sourcefetch/*`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/... ./internal/adapters/sourcefetch/...`

Артефакт результата:

- `mirror` и `direct` оба покрыты тестами и используют разные live shells.

### Slice 3. Открыть values/template/docs surface

Цель:

- сделать режим управляемым на уровне модуля и честно задокументировать trade-off.

Файлы/каталоги:

- `openapi/values.yaml`
- `templates/controller/deployment.yaml`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- module values/docs/render знают про оба режима и safe default.

### Slice 4. Финальная синхронизация и repo-level verification

Цель:

- убедиться, что bundle landed системно, а не как случайный набор правок.

Файлы/каталоги:

- все изменённые surfaces текущего bundle

Проверки:

- `make fmt`
- `make test`
- `make verify`
- `git diff --check`

Артефакт результата:

- landed continuation slice с проверяемым runtime contract для двух режимов.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: runtime knob и wiring уже
зафиксированы, но values/docs surface ещё не открыт наружу.

После Slice 2 можно безопасно откатиться до текущего default `mirror`, не
ломая upload path и без partial public config drift.

## 5. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
