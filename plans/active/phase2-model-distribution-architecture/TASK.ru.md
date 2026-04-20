## 1. Заголовок

Архитектура distribution/runtime delivery для DMZ, streaming OCI publish и
node-local model cache mounts

## 2. Контекст

Текущий phase-2 runtime уже умеет:

- публиковать канонический OCI `ModelPack` artifact;
- держать durable source mirror в object storage;
- materialize'ить опубликованный artifact в workload через controller-owned
  `initContainer`;
- реиспользовать user-provided mount `/data/modelcache` и координировать shared
  cache root.

При этом live architecture всё ещё ориентирована на один publication path и
один delivery path:

- canonical publish target = internal `DMCR`;
- runtime delivery = `materialize-artifact` init container inside workload;
- publish path = ai-models-owned native OCI `ModelPack` publication over the
  current direct-upload/object-source baseline;
- runtime cache ownership = workload-owned volume, а не node-local shared cache.

Новый запрос пользователя требует уже не bounded fix, а отдельный архитектурный
workstream:

1. explicit scenario с proxy/mirror registry в `DMZ`;
2. streaming publication directly to OCI/registry without full local staging,
   если это реально defendable;
3. node-level cache service, который монтирует модели в workload без init
   container, реиспользует local disk cache на node, шарит одну и ту же модель
   между несколькими pod'ами и умеет eviction/refresh.

Это пересекает publication, distribution, runtime delivery и storage topology,
но не должно снова размыть public API `Model` / `ClusterModel`.

## 3. Постановка задачи

Нужно спроектировать следующий phase-2 continuation так, чтобы:

- canonical publication contract оставался простым и controller-owned;
- distribution topology стала отдельной boundary поверх уже опубликованного OCI
  artifact, а не смешивалась с source ingest;
- streaming publish в OCI был либо честно доказан как реальный новый adapter,
  либо явно отвергнут как лишний поверх уже landed native OCI/direct-upload
  baseline;
- runtime delivery эволюционировала от init-container materialization к
  optional node-local cache/mount service без ломки текущего workload contract;
- интеграция с `sds-node-configurator` использовалась только как storage
  substrate/provider для node cache, а не как неявный replacement K8s delivery
  semantics;
- пользователь не был вынужден указывать runtime metadata в `spec`, если её
  реально можно вычислить из source/artifact contents.

## 4. Scope

- зафиксировать target architecture split:
  - publication
  - distribution
  - runtime delivery
  - node-local cache management
- описать scenario с proxy/mirror registry в `DMZ`:
  - источник истины;
  - направление sync/replication;
  - auth/trust boundaries;
  - failure/lag semantics;
- проверить feasibility streaming publish directly to OCI:
  - current native OCI publisher constraints;
  - что требуется от нового publisher;
  - возможен ли direct source/upload -> OCI path без полного local model dir;
- описать target runtime delivery without init container:
  - node daemon / CSI-like mount contract;
  - node-local cache layout;
  - multi-pod same-model reuse;
  - eviction/TTL/reconciliation behavior;
  - per-node behavior for distributed inference;
- определить, какие knobs живут в:
  - module config / values;
  - internal runtime config;
  - status/conditions/metrics;
- определить bounded next implementation slices.

## 5. Non-goals

- не реализовывать в этом bundle весь node cache daemon или новый CSI driver;
- не вводить сейчас новый public `Model.spec` для distribution/runtime knobs;
- не обещать новый streaming surface, если уже landed native OCI/direct-upload
  baseline покрывает требуемый contract и спор остаётся только в terminology;
- не тащить `sds-node-configurator` API напрямую в public model contract;
- не смешивать `DMZ` distribution scenario с source ingest semantics;
- не удалять текущий init-container path до появления defendable replacement.

## 6. Затрагиваемые области

- `plans/archive/2026/phase2-runtime-followups/*`
- `plans/archive/2026/publication-storage-hardening/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/modelpack/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- при необходимости external reference:
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/sds-node-configurator`

## 7. Критерии приёмки

- Создан отдельный compact continuation bundle, который не раздувает archived
  `phase2-runtime-followups` и archived predecessor
  `publication-storage-hardening`.
- В bundle явно разделены четыре live boundary:
  - canonical publication to internal `DMCR`;
  - optional distribution to/through `DMZ` registry;
  - runtime delivery contract for workloads;
  - node-local cache manager / mount service.
- `DMZ` scenario описан как distribution seam над OCI artifact by digest, а не
  как изменение `Model.spec.source` или source-ingest contract.
- По streaming publish есть явный verdict:
  - либо нужен новый stream-capable publisher adapter;
  - либо current native OCI/direct-upload path признан sufficient baseline, а
    дополнительный streaming slice зафиксирован как separate enhancement.
- Bundle явно фиксирует, что live publication baseline уже принадлежит
  ai-models-owned native OCI publisher и не опирается на historical `KitOps`
  shell как source of truth.
- Node-local cache scenario описан как optional replacement/extension поверх
  текущего init-container delivery, а не как incidental side effect в
  `k8s/modeldelivery`.
- Bundle явно фиксирует, что `sds-node-configurator` даёт storage substrate
  (`LocalStorageClass` / local volumes), но не заменяет собой node cache
  service, mount propagation и cache eviction semantics.
- Не появляется новый public heavy spec: user-facing `Model` / `ClusterModel`
  остаются сконцентрированы на source intent, а runtime/distribution metadata
  остаются computed/configured outside the public source contract.
- Есть bounded next slices с проверяемыми файлами и validations для:
  - `DMZ` distribution;
  - stream-publish feasibility/implementation;
  - node-cache delivery.

## 8. Риски

- легко смешать canonical artifact publication и external distribution topology,
  после чего `Model.status` перестанет быть понятным;
- попытка "сделать streaming publish быстро" может закончиться только ещё одним
  wrapper'ом поверх уже landed native OCI publisher, а не реальным новым
  capability;
- попытка встроить node-local cache прямо в current `modeldelivery` adapter
  может создать новый giant package и сломать boundary discipline;
- прямое связывание ai-models с `sds-node-configurator` CRDs без отдельного
  cache-service слоя приведёт к storage-specific API leakage;
- premature removal текущего init-container delivery path оставит проект без
  рабочего runtime fallback.
