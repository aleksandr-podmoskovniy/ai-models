## 1. Заголовок

Managed node-local cache substrate и runtime delivery continuation поверх
`sds-node-configurator`

## 2. Контекст

Архитектурный predecessor
`plans/active/phase2-model-distribution-architecture/*` уже зафиксировал
целевую phase-2 форму:

- canonical publication остаётся controller-owned и заканчивается во внутреннем
  `DMCR`;
- current runtime delivery всё ещё materialize'ит артефакт в
  workload-owned `/data/modelcache`;
- node-local cache должен стать отдельной boundary поверх уже опубликованного
  OCI artifact;
- `sds-node-configurator` должен использоваться как storage substrate, а не как
  cache manager.

Пользователь ужесточил требования для continuation slice:

- local cache должен жить на node-local disk;
- модуль должен сам готовить local storage substrate и не требовать ручного
  `LocalStorageClass`;
- у cache plane должен быть размерный budget;
- runtime tree должен двигаться к целевой структуре без giant package и без
  смешения storage substrate с delivery semantics.

## 3. Постановка задачи

Нужно открыть implementation continuation для node-local cache delivery и
посадить первый реальный slice на managed substrate plane:

- ai-models controller должен уметь держать managed
  `LVMVolumeGroupSet` для cache substrate поверх `sds-node-configurator`;
- ai-models controller должен автоматически держать
  `LocalStorageClass` для этого substrate, чтобы пользователь не создавал её
  вручную;
- cluster-level config должен задавать:
  - enablement;
  - cache size budget для substrate thin-pool;
  - node selector и block-device selector для substrate;
- runtime/delivery tree должен получить отдельную boundary под managed
  node-cache substrate, а не разрастись внутри текущего `modeldelivery`.

При этом нужно сохранить честную границу:

- current workload delivery path через `materialize-artifact` остаётся рабочим
  fallback;
- при включённом managed node cache workload не должен требовать ручного
  `PersistentVolumeClaim` или заранее подготовленного `/data/modelcache` mount;
- eviction/unused cleanup policy для самого cache plane не надо имитировать до
  появления реального node-cache runtime.

## 4. Scope

- новый compact continuation bundle для node-local cache workstream;
- cluster-level module config для managed node-cache substrate;
- controller-owned reconciliation for:
  - `storage.deckhouse.io/v1alpha1` `LVMVolumeGroupSet`;
  - `storage.deckhouse.io/v1alpha1` `LocalStorageClass`;
- отдельный K8s adapter boundary для shaping внешних storage CR;
- bootstrap/config/RBAC wiring нового controller;
- managed local-volume fallback для current workload delivery path поверх
  ai-models-owned `LocalStorageClass`;
- отдельная internal boundary для shared node-cache layout, marker,
  coordination и eviction planning;
- docs/structure/evidence under current-state wording.

## 5. Non-goals

- не реализовывать в этом bundle сам node-cache daemon/CSI node plugin;
- не переводить workload delivery на новый mount path до появления реального
  cache plane;
- не выводить наружу cleanup/TTL knob до появления реального node-cache
  runtime, который действительно умеет его исполнять;
- не вводить новый public `Model.spec` contract для runtime storage;
- не выдавать per-pod local fallback volume за уже готовый shared node cache
  service;
- не делать destructive automatic cleanup для уже созданного substrate при
  disable path;
- не притворяться, что `maxUnusedAge` уже работает без отдельного cache
  runtime.

## 6. Затрагиваемые области

- `plans/active/node-local-cache-runtime-delivery/*`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/bootstrap/*`
- `images/controller/internal/controllers/*`
- `images/controller/internal/adapters/k8s/*`
- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `templates/node-cache-runtime/*`
- `tools/helm-tests/*`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

## 7. Критерии приёмки

- Есть отдельный continuation bundle, который явно продолжает
  `phase2-model-distribution-architecture`, а не плодит новый sibling source of
  truth.
- ai-models получает отдельный controller-owned substrate boundary для managed
  node-local cache storage.
- При включении managed substrate controller:
  - держит `LVMVolumeGroupSet` с predictable ai-models-owned labels;
  - держит `LocalStorageClass` по текущему списку managed `LVMVolumeGroup`;
  - не требует ручного создания `LocalStorageClass`.
- Cache size budget действительно влияет на substrate shape через thin-pool
  size, а не остаётся мёртвым config knob.
- При включённом `nodeCache` current workload delivery умеет сам подложить
  managed local ephemeral volume на `/data/modelcache`, используя ai-models
  owned `LocalStorageClass`, если workload не принёс свой cache volume сам.
- Shared node-cache layout, marker и coordination больше не живут как
  command-local business logic inside `materialize-artifact`, а вынесены в
  отдельную reusable internal boundary.
- Shared node-cache digest store больше не смешан в одном helper surface с
  workload-local `current` symlink contract: consumer materialization layout
  явно отделён от общего node-wide cache store.
- Для будущего node-cache runtime уже есть bounded scan/planning surface,
  которая умеет строить eviction candidates по реальному cache-root state без
  публичного обещания, что cleanup policy уже активна в кластере.
- Есть module-owned node-cache runtime plane на стандартном `DaemonSet` +
  generic ephemeral volume поверх ai-models-owned `LocalStorageClass`, который
  исполняет bounded maintenance loop над shared cache-root без custom per-node
  controller/PVC ownership shell, не забирает весь substrate budget у current
  fallback path service-volume request'ом и не выдаёт current fallback path за
  уже готовый shared mount service.
- Managed workload delivery проецирует в `PodTemplateSpec` immutable published
  artifact identity, а ai-models держит отдельный internal node-cache intent
  plane, который:
  - собирает per-node desired artifact set по реально запланированным managed
    `Pod`;
  - проецирует этот set в module-owned per-node `ConfigMap`;
  - позволяет `node-cache-runtime` реально prefetch'ить shared cache entries в
    node-local store без нового public API.
- Shared cache entries, которые входят в текущий desired node intent set, не
  выталкиваются idle/size eviction policy по умолчанию, пока intent остаётся
  актуальным.
- Runtime delivery fallback через current workload-owned `/data/modelcache`
  остаётся совместимым: user-provided cache topology по-прежнему поддержан.
- Тесты систематически покрывают:
  - config validation;
  - desired object shaping;
  - reconcile on empty/live managed LVG set;
  - no-op disabled path;
  - managed fallback volume injection/removal;
  - coexistence with user-provided cache topology;
  - cache-root layout/marker parsing;
  - coordination lock reuse and stale-lock recovery;
  - bounded eviction planning over ready and malformed cache entries.
  - node-cache runtime DaemonSet shaping and render guardrails;
  - bounded node-cache maintenance loop over malformed/idle entries.
  - pod-to-node intent extraction and per-node ConfigMap shaping;
  - per-node intent reconcile over scheduled/removed managed Pods;
  - shared-store prefetch and protected-digest eviction behavior.
- Перед завершением проходит `make verify`.

## 8. Риски

- легко протащить storage-specific shape прямо в `modeldelivery` и получить
  giant mixed package;
- destructive cleanup при disable path может задеть уже созданные local cache
  volumes, поэтому этот slice должен fail-safe freeze, а не aggressively
  delete;
- без отдельного controller-owned `LocalStorageClass` maintainer loop придётся
  либо требовать ручной storage setup, либо делать brittle render-time magic;
- если оставить node maintenance plane на custom per-node reconcile, следующий
  CSI-like slice придётся насаживать на уже устаревший ownership shell вместо
  стандартного node-agent `DaemonSet`;
- можно преждевременно вывести user-facing cleanup policy без реального cache
  runtime, что создаст ложный продуктовый контракт.
- если начать prefetch'ить shared store без отдельного node intent plane,
  `DaemonSet` либо будет тянуть все артефакты на все ноды, либо получит
  неявный ad-hoc источник истины вместо bounded controller-owned contract.
