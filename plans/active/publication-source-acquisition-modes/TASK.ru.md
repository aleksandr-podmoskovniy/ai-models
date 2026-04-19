## 1. Заголовок

Единый `source acquisition mode` в publication path: напрямую или через
durable intermediate mirror

## 2. Контекст

Текущий publication runtime уже умеет два фактических варианта получения байтов
для remote `HuggingFace` source:

- `remote source -> native OCI publish`;
- `remote source -> controller-owned source mirror -> native OCI publish`.

Но сверху этот выбор сейчас оформлен неправильно:

- контракт назван как provider-specific acquisition toggle, хотя concern на
  самом деле про acquisition boundary, а не про конкретного provider;
- upload path живёт на своей staging boundary, но runtime surface это не
  отражает как часть единой политики получения исходных байтов;
- values, controller flags и worker args разъехались в provider-specific
  naming, хотя operator должен мыслить одним cluster-level режимом acquisition.

В результате код уже умеет два bounded path, но платформа описывает их как
частный toggle для `HuggingFace`, а не как единый publication contract.

Это continuation workstream от
`plans/active/phase2-model-distribution-architecture/`, но уже в
implementation surface текущего native `OCI` publisher, а не старого
`KitOps` baseline.

## 3. Постановка задачи

Нужно сделать явный и проверяемый cluster-level `source acquisition mode` для
publication path:

- режим `mirror`:
  remote source байты сначала проходят через controller-owned durable
  intermediate copy, если такой adapter-boundary поддерживается;
- режим `direct`:
  publication worker использует canonical source boundary напрямую, без лишней
  промежуточной полной копии.

При этом нужно:

- провести режим через values/OpenAPI/controller runtime/sourceworker args;
- не тащить его в public `Model.spec`;
- сохранить upload/staged-source semantics без regressions;
- не делать provider-specific public knobs для acquisition policy;
- обновить docs/evidence так, чтобы current byte path и trade-offs были описаны
  как единый acquisition contract для всех source path.

## 4. Scope

- новый cluster-level runtime knob для выбора `source acquisition mode`;
- wiring этого режима через:
  - `openapi/values.yaml`
  - controller flags/config
  - `catalogstatus/sourceworker`
  - `publish-worker/sourcefetch`;
- валидация режима и default `direct`;
- тесты для обоих acquisition modes;
- актуализация operator-facing docs и test evidence;
- фиксация next-step boundary для более глубокого будущего шага:
  direct write into `DMCR` backing storage under `DMCR` control.

## 5. Non-goals

- не реализовывать в этом bundle прямую запись слоёв в backend object storage
  `DMCR` в обход текущего registry upload protocol;
- не менять OCI artifact format, layer layout или current `tar`-based object
  source packaging;
- не проектировать здесь весь `DMZ` distribution tier;
- не менять public `Model` / `ClusterModel` API;
- не менять upload-session/staged upload contract кроме необходимого wiring;
- не вводить provider-specific public runtime knobs для acquisition policy.

## 6. Затрагиваемые области

- `plans/active/publication-source-acquisition-modes/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `openapi/values.yaml`
- `templates/controller/deployment.yaml`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- Появляется единый cluster-level `source acquisition mode` с двумя
  значениями: `mirror` и `direct`.
- Default становится `direct`.
- В public values, templates, controller flags/env и worker args больше нет
  provider-specific acquisition naming drift.
- В режиме `mirror` remote source path, который поддерживает durable mirror,
  по-прежнему получает controller-owned intermediate copy и `publish-worker`
  реально идёт через source mirror.
- В режиме `direct` remote source path идёт без лишней промежуточной полной
  копии и без локального materialization fallback.
- Upload/staged source path остаётся defendable частью того же acquisition
  contract: source of truth для него — staged object boundary, а не локальный
  workspace.
- `values`, templates и docs честно объясняют оба режима и их trade-off:
  - `mirror` = durable intermediate copy и resumable remote acquisition;
  - `direct` = быстрее и без лишней полной копии в object storage.
- `TEST_EVIDENCE` и runtime docs больше не описывают acquisition policy как
  `HuggingFace`-частный контракт.
- Есть узкие тесты на:
  - controller/sourceworker args для обоих режимов;
  - validation/defaulting;
  - behavior для remote source и upload source под единым contract surface.
- Перед завершением проходит `make verify`.

## 8. Риски

- можно случайно оставить provider-specific drift в naming, даже если
  поведение уже стало generic;
- можно провести knob только через values/templates, но не через runtime
  wiring, и получить misleading config contract;
- можно неявно поломать `upload-session` или staged upload path, если новая
  validation начнёт трактовать upload как remote mirror candidate;
- можно перегрузить bundle попыткой одновременно переделать `DMCR` upload
  protocol; это нужно держать как отдельный follow-up.
