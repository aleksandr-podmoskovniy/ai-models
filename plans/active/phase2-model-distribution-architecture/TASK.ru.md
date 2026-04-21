## 1. Заголовок

Архитектура distribution/runtime delivery для DMZ, нового `DMCR direct-upload`
и
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
2. новый `DMCR direct-upload` протокол без предварительно известного digest,
   где источник читается один раз, digest считается на лету, а байты сразу
   уходят в временную upload-session;
3. node-level cache service, который монтирует модели в workload без init
   container, реиспользует local disk cache на node, шарит одну и ту же модель
   между несколькими pod'ами и умеет eviction/refresh.

Это пересекает publication, distribution, runtime delivery и storage topology,
но не должно снова размыть public API `Model` / `ClusterModel`.

Continuation note:

- отдельный corrective workstream по zero-trust ingest во внутренний `DMCR`
  вынесен в `plans/active/dmcr-zero-trust-ingest/*`;
- этот bundle остаётся зонтичным architecture bundle для publication /
  distribution / runtime split и не должен дублировать низкоуровневый ingest
  contract как второй источник истины.

## 3. Постановка задачи

Нужно спроектировать следующий phase-2 continuation так, чтобы:

- canonical publication contract оставался простым и controller-owned;
- published contract оставался тем же даже при смене внутренней раскладки
  слоёв и runtime/distribution topology;
- distribution topology стала отдельной boundary поверх уже опубликованного OCI
  artifact, а не смешивалась с source ingest;
- единственным правильным publication protocol стал новый `DMCR direct-upload`
  с digest в конце сессии, а текущий digest-first helper больше не считался
  целевой архитектурой;
- runtime/distribution topology имела одну целевую схему, а не набор
  долгоживущих fallback-вариантов;
- runtime delivery переходила к node-local shared cache/mount service без ломки
  published contract;
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
- зафиксировать единственный целевой publication protocol:
  - publication session собирает published artifact из набора независимых
    blob-сессий;
  - крупные исходные файлы публикуются как отдельные raw blob-ы без
    монолитной перепаковки всей модели;
  - мелкие файлы при необходимости укладываются в ограниченные служебные
    bundle-слои;
  - published artifact завершается маленьким `config/manifest`, который
    описывает materialized tree и список sealed blob-ов;
  - для trusted internal sources (`HF`, mirror, controller-owned upload worker)
    тяжёлые байты идут напрямую в object storage, а `DMCR` координирует сессии,
    sealing и публикацию manifest;
  - для untrusted user upload допускается временный staging-object с
    последующим seal-in-place: один дополнительный read без полной переписки
    тех же байтов в новый объект;
- явно отделить published contract от internal artifact layout:
  - что считается стабильным для consumers;
  - что может меняться внутри publisher/materializer без нового user-facing
    контракта;
- зафиксировать, что internal artifact layout и runtime/distribution topology
  можно менять радикально, если снаружи сохраняются:
  - OCI artifact by digest;
  - stable materialized model entrypoint;
  - consumer-facing workload contract;
- описать target runtime delivery without init container:
  - node daemon / CSI-like mount contract;
  - node-local cache layout;
  - multi-pod same-model reuse;
  - eviction/TTL/reconciliation behavior;
  - per-node behavior for distributed inference;
- закрыть текущие продовые щели переходного init-container delivery path,
  которые уже мешают живым smoke/workload сценариям:
  - runtime delivery должен уметь тянуть собственный runtime image через
    явный `imagePullSecret`, а не предполагать публичную доступность образа;
  - `catalogstatus` не должен сыпать conflict-ошибками на обычном
    `status`-reconcile path;
  - publication/runtime путь должен давать минимально defendable
    machine-readable сигнал по текущему этапу и результату доставки, а не
    оставлять оператора только с сырыми логами init-container;
- определить, какие knobs живут в:
  - module config / values;
  - internal runtime config;
  - status/conditions/metrics;
- определить bounded next implementation slices.

## 5. Non-goals

- не реализовывать в этом bundle весь node cache daemon или новый CSI driver;
- не вводить сейчас новый public `Model.spec` для distribution/runtime knobs;
- не закреплять текущий digest-first `DMCR` helper как acceptable long-term
  baseline;
- не тащить `sds-node-configurator` API напрямую в public model contract;
- не смешивать `DMZ` distribution scenario с source ingest semantics;
- не превращать временный migration bridge в второй постоянный product
  contract.

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
- Bundle явно фиксирует единственный целевой publication protocol:
  - published artifact is a manifest over sealed blob-set, а не один
    гигантский монолитный blob;
  - large model files stay independent blob units;
  - full-model repack into one heavy archive is not the target architecture;
  - final publish step writes only small manifest/config metadata.
- Bundle явно фиксирует внутренний storage verdict:
  - sealed blob is the reusable storage unit for publish and runtime cache;
  - artifact digest remains the stable published contract above the blob-set;
  - object storage carries heavy bytes, while `DMCR` owns session/index state.
- Bundle явно фиксирует stable published contract:
  - immutable OCI artifact by digest;
  - stable consumer-facing materialized model entrypoint;
  - без утечки internal layer split/order/compression decisions в public
    contract.
- `DMZ` scenario описан как distribution seam над OCI artifact by digest, а не
  как изменение `Model.spec.source` или source-ingest contract.
- По streaming publish есть явный verdict:
  - нужен новый stream-capable publication path поверх `DMCR direct-upload`;
  - текущий digest-first helper не описывается как финальная архитектура.
- По zero-restage publication есть явный verdict:
  - trusted internal ingest не должен требовать ни full local restage, ни
    монолитного server-side data plane через `DMCR`;
  - untrusted user upload не должен требовать второй полной записи тех же
    байтов в object storage.
- Bundle явно фиксирует, что live publication baseline уже принадлежит
  ai-models-owned native OCI publisher и не опирается на historical `KitOps`
  shell как source of truth.
- Node-local cache scenario описан как единственная целевая runtime topology
  поверх published contract, а текущий init-container path описан только как
  временный migration bridge.
- Bundle отдельно фиксирует, что internal layer layout и runtime/distribution
  topology могут меняться эволюционно, пока сохраняются:
  - published artifact digest/reference semantics;
  - stable workload-facing runtime contract;
  - backward-compatible materialization result.
- В target picture нет второго равноправного runtime delivery mode:
  long-lived fallback contract не фиксируется как часть финальной схемы.
- Bundle явно фиксирует, что `sds-node-configurator` даёт storage substrate
  (`LocalStorageClass` / local volumes), но не заменяет собой node cache
  service, mount propagation и cache eviction semantics.
- Не появляется новый public heavy spec: user-facing `Model` / `ClusterModel`
  остаются сконцентрированы на source intent, а runtime/distribution metadata
  остаются computed/configured outside the public source contract.
- Для текущего bridge delivery path закрыты три эксплуатационных дыры:
  - workload materializer может быть запущен в namespace с приватным runtime
    образом без ручного дописывания `imagePullSecrets` пользователем;
  - обычный `Model.status` reconcile не падает на
    `the object has been modified` при штатной конкуренции controller writes;
  - оператор получает хотя бы bounded signal о том, что publication завершён,
    runtime delivery применён и materialization ещё не завершилась или уже
    упёрлась в `ImagePull`/`Init` failure.
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
