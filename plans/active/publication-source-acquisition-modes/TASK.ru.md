## 1. Заголовок

Явные режимы загрузки `HuggingFace` в publication path: через временное зеркало
и напрямую

## 2. Контекст

Текущий publication runtime уже умеет два фактических варианта получения байтов
для `HuggingFace`:

- `HF -> source mirror -> native OCI publish`;
- `HF -> direct remote object source -> native OCI publish`.

Но сверху эти режимы не разведены как явный runtime contract. Контроллер всегда
пробрасывает `raw-stage-bucket/raw-stage-key-prefix`, из-за чего source mirror
включается автоматически при любом `HuggingFace` publish. В результате:

- direct path по коду существует, но operator не может его честно выбрать;
- docs и values не объясняют trade-off между скоростью первой загрузки и
  наличием durable временного зеркала;
- обсуждение следующего шага про возможное исключение `DMCR` из толстого пути
  передачи байтов смешивается с более простым и уже defendable bounded slice:
  дать два рабочих режима acquisition boundary.

Это continuation workstream от
`plans/active/phase2-model-distribution-architecture/`, но уже в
implementation surface текущего native `OCI` publisher, а не старого
`KitOps` baseline.

## 3. Постановка задачи

Нужно сделать явный и проверяемый cluster-level переключатель для
`HuggingFace` publication path:

- режим `mirror`:
  `HF -> controller-owned source mirror in object storage -> native OCI publish`;
- режим `direct`:
  `HF -> direct remote object source -> native OCI publish`.

При этом нужно:

- провести режим через values/OpenAPI/controller runtime/sourceworker args;
- не тащить его в public `Model.spec`;
- сохранить staging/upload semantics без regressions;
- обновить docs/evidence так, чтобы current byte path и trade-offs были описаны
  честно.

## 4. Scope

- новый cluster-level runtime knob для выбора `HuggingFace` acquisition mode;
- wiring этого режима через:
  - `openapi/values.yaml`
  - controller flags/config
  - `catalogstatus/sourceworker`
  - `publish-worker/sourcefetch`;
- валидация режима и безопасный default;
- тесты для обоих publish modes;
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
- не менять upload-session/staged upload contract кроме необходимого wiring.

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

- Появляется явный cluster-level режим `HuggingFace` acquisition с двумя
  значениями: `mirror` и `direct`.
- Safe default сохраняет текущий runtime behavior и не ломает existing
  clusters.
- В режиме `mirror` sourceworker по-прежнему получает
  `--raw-stage-bucket/--raw-stage-key-prefix`, а `publish-worker` реально идёт
  через source mirror.
- В режиме `direct` sourceworker больше не пробрасывает raw-stage args для
  `HuggingFace`, а `publish-worker` реально идёт через direct remote object
  source без локального materialization fallback.
- Upload/staged source path не деградирует и не зависит от нового knob.
- `values`, templates и docs объясняют оба режима и их trade-off:
  - `mirror` = durable intermediate copy и resumable acquisition;
  - `direct` = быстрее и без лишней полной копии в object storage.
- `TEST_EVIDENCE` и runtime docs больше не описывают source mirror как
  единственный `HuggingFace` path.
- Есть узкие тесты на:
  - controller/sourceworker args для обоих режимов;
  - validation/defaulting;
  - direct-vs-mirror selection в `publish-worker` shell.
- Перед завершением проходит `make verify`.

## 8. Риски

- можно случайно сломать текущий default `mirror` path и regress'нуть live
  publish в existing clusters;
- можно провести knob только через templates, но не через runtime wiring, и
  получить misleading config contract;
- можно неявно поломать `upload-session` или staged upload path, если новая
  validation смешает `HuggingFace` и upload semantics;
- можно перегрузить bundle попыткой одновременно переделать `DMCR` upload
  protocol; это нужно держать как отдельный follow-up.
