## 1. Current phase

Этап 2. Каноническая публикация в `DMCR` уже зафиксирована. Текущий workstream
закрывает именно runtime topology: быстрый узловой путь доставки модели и
запасной путь через том прикладного объекта.

## 2. Orchestration

`full`

Причина:

- задача одновременно затрагивает runtime delivery, storage substrate,
  controller wiring, values/OpenAPI, эксплуатационные сигналы и документацию;
- здесь легко снова размыть границы между публикацией, узловым кэшем и
  доставкой модели в прикладной объект;
- финальная форма должна быть defendable как production-ready contract, а не
  как ещё один временный обходной слой.

Read-only reviews, обязательные перед большими implementation slices:

- `repo_architect`
  - проверить, что новый путь не превращает `images/controller` в giant mixed
    runtime tree;
- `integration_architect`
  - проверить границы между storage substrate, узловым runtime plane,
    доставкой в прикладной объект и наблюдаемостью;
- `api_designer`
  - проверить, что runtime/storage policy не протаскивается в публичный
    `Model` / `ClusterModel` контракт.

## 3. Slices

### Slice 1. Перефиксировать workstream и заморозить целевую схему

Цель:

- оставить один канонический active bundle, который ясно описывает:
  - текущее состояние;
  - целевую двурежимную схему;
  - инварианты, которые нельзя потом размыть в коде.

Файлы/каталоги:

- `plans/active/node-local-cache-runtime-delivery/*`

Проверки:

- manual consistency review against:
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/internal/controllers/workloaddelivery/*`
  - `images/controller/internal/controllers/nodecacheruntime/*`

Артефакт результата:

- compact bundle с одной целевой картиной и без смешения fast path и fallback
  path.

### Slice 2. Вычистить vocabulary drift и остаточные ложные seams

Цель:

- привести код, docs и bundle к одному словарю:
  - `узловой общий кэш`;
  - `запасной том прикладного объекта`;
  - `стабильный workload-facing runtime-контракт`;
- удалить или локализовать surfaces, которые больше не участвуют в живом пути
  и только создают архитектурный шум.

Файлы/каталоги:

- `images/controller/internal/nodecache*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `rg -n "materialize-artifact|nodecacheintent|shared mount|AI_MODELS_MODEL_PATH|fallback" images/controller docs/CONFIGURATION.ru.md`
- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/controllers/nodecacheruntime ./internal/nodecache`

Артефакт результата:

- один словарь и один объяснимый contract surface без старых полумёртвых
  narrative seams.

### Slice 3. Довести быстрый режим через узловой общий кэш

Цель:

- сделать так, чтобы при доступном узловом кэше прикладной объект получал
  модель из node-owned shared plane, а не через отдельную полную раскладку в
  собственный том.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime ./internal/nodecache ./cmd/ai-models-artifact-runtime`

Артефакт результата:

- workload-facing shared delivery path, который переиспользует одну и ту же
  модель на ноде и не требует per-workload full materialization при готовом
  узловом кэше.

### Slice 4. Упростить и стабилизировать запасной путь

Цель:

- сохранить рабочий fallback path и сделать его строго детерминированным:
  когда именно он выбирается, что именно контроллер инжектит и как снимает
  эту мутацию, когда быстрый путь недоступен.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/cmd/ai-models-controller/*`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `make helm-template`

Артефакт результата:

- запасной путь остаётся рабочим, понятным и не маскируется под основную
  целевую схему.

### Slice 5. Дотянуть сигналы наблюдаемости и эксплуатационный контур

Цель:

- дать оператору ясный ответ:
  - какой режим выбран;
  - почему включился fallback;
  - в каком состоянии узловой runtime plane;
  - можно ли считать быстрый путь реально готовым на конкретной ноде.

Файлы/каталоги:

- `images/controller/internal/monitoring/runtimehealth/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

Проверки:

- `cd images/controller && go test ./internal/monitoring/runtimehealth ./internal/controllers/workloaddelivery ./internal/controllers/nodecacheruntime`

Артефакт результата:

- production-readable signals по режиму доставки и состоянию узлового слоя.

### Slice 6. Синхронизировать docs, values и repo surface с итоговой схемой

Цель:

- привести values, OpenAPI, docs и controller structure к одному финальному
  описанию без обещаний того, чего ещё нет, и без legacy wording drift.

Файлы/каталоги:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- repo-local surfaces совпадают с тем, что реально делает код.

## 4. Rollback point

После Slice 2 можно безопасно остановиться:

- bundle уже канонически фиксирует цель;
- словарь и boundaries вычищены;
- текущий рабочий fallback path ещё не ломался;
- быстрый путь ещё не частично внедрён.

Это последняя безопасная точка перед изменением live runtime semantics.

## 5. Final validation

- узкие `go test` по затронутым пакетам после каждого slice;
- `make helm-template`;
- `make kubeconform`;
- `make verify`.
