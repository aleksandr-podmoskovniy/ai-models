---
title: "Конфигурация"
menuTitle: "Конфигурация"
weight: 60
---

<!-- SCHEMA -->

Текущий конфигурационный контракт `ai-models` намеренно короткий.
На уровне модуля наружу выставляются только стабильные настройки:

- `logLevel`;
- `artifacts`.
- `dmcr`.
- `nodeCache`.

Режим HA, HTTPS policy, ingress class, controller/runtime wiring, внутренний
`DMCR`, upload-gateway и publication worker остаются во global Deckhouse
settings и internal module values. В user-facing contract больше нет:

- retired backend auth/workspace и metadata-database knobs;
- browser SSO knobs;
- backend-only secrets;
- внешнего publication registry contract;
- backend-specific `artifacts.pathPrefix`.

`artifacts` задаёт общий S3-compatible storage для byte-path внутри ai-models.
Разделение внутри bucket фиксировано самим runtime:

- `raw/` для controller-owned upload staging и, если включён режим
  `artifacts.sourceFetchMode=Mirror`, для временного source mirror;
- `dmcr/` для опубликованных OCI-артефактов во внутреннем `DMCR`;
- отдельные будущие append-only данные модуля могут жить только под
  отдельными фиксированными префиксами.

Учётные данные для artifact storage задаются только через
`credentialsSecretName`. Secret должен жить в `d8-system` и содержать фиксированные
ключи `accessKey` и `secretKey`. Сам модуль копирует только эти ключи в свой
namespace перед рендером runtime workload'ов, поэтому пользователь не управляет
storage credentials напрямую в `d8-ai-models`.

Custom trust для S3-compatible endpoint задаётся через `artifacts.caSecretName`.
Этот Secret должен жить в `d8-system` и содержать `ca.crt`. Если
`caSecretName` пустой, ai-models сначала reuse'ит `credentialsSecretName`,
если тот же Secret тоже содержит `ca.crt`, а иначе fallback'ится на platform CA,
который уже discovered module runtime или скопирован из global HTTPS
`CustomCertificate` path.

`dmcr.gc.schedule` выставляет наружу productized cadence для stale sweep во
внутреннем registry. По умолчанию ai-models ставит один stale cleanup cycle
каждые 20 минут. Пустая строка отключает периодический sweep, но не убирает
operator-facing inspection surface: внутри Pod'а `DMCR` по-прежнему можно
запустить `dmcr-cleaner gc check` и получить report по stale published
repository prefix, source-mirror prefix и orphan direct-upload prefix без
`.dmcr-sealed` reference, который пережил bounded stale-age window.

Если scheduled loop стартует уже после настроенного schedule tick, он делает
report-only startup check и повторяет его при transient failures. Если stale
cleanup candidates уже есть и нет другого active или queued GC request, loop
ставит обычный scheduled request; он не запускает destructive cleanup напрямую
и всё равно проходит через штатный coalescing debounce. Cleanup cycle использует
внутренний zero-rollout maintenance gate с runtime ack quorum, а не изменение
Pod template `DMCR`.

Публичный runtime path для моделей теперь controller-owned:

- `Model` / `ClusterModel` используют один cluster-level
  `artifacts.sourceFetchMode`:
  - `Mirror`:
    remote `source.url` сначала идёт через controller-owned source mirror;
  - `Direct`:
    remote `source.url` идёт напрямую из canonical remote source boundary;
- `spec.source.upload` использует controller-owned upload-session path и
  остаётся на своей отдельной staged object boundary; upload URL публикуются в
  status, а raw Bearer header value хранится в указанном token Secret;
- все пути публикуют OCI `ModelPack` артефакты во внутренний `DMCR`;
- потоковые multi-file remote входы публикуются как одна ограниченная bundle-
  упаковка для мелких companion-файлов плюс отдельные raw-слои для крупных
  model payload, без монолитной перепаковки всей модели в один tar-слой;
- single-file direct и staged-object входы по-прежнему публикуются одним
  raw-слоем;
- archive-входы остаются на archive-source streaming path и не создают
  распакованное success-only дерево checkpoint'а.

RBAC разделён по Deckhouse `user-authz` и `rbacv2`:

- legacy `user-authz` даёт read-only видимость `Model` / `ClusterModel` на
  уровне `User`, write-доступ к namespaced `Model` на уровне `Editor`, а
  write-доступ к cluster-wide `ClusterModel` только на уровне `ClusterEditor`;
- `PrivilegedUser`, `Admin` и `ClusterAdmin` не добавляют лишних ai-models
  verbs, пока в модуле нет безопасного user-facing ресурса для этих уровней;
- `rbacv2/use` ограничен namespaced `Model`, поэтому namespaced RoleBinding не
  становится неявным доступом к cluster-scoped `ClusterModel`;
- `rbacv2/manage` является cluster-persona путём для `Model`, `ClusterModel` и
  `ModuleConfig` `ai-models`;
- human-facing роли модуля намеренно не дают `status`, `finalizers`, доступ к
  Secret, pod logs, exec, attach, port-forward и internal runtime objects.

Default — `artifacts.sourceFetchMode=Direct`.

Trade-off между режимами такой:

- `Mirror` сохраняет durable промежуточную копию в object storage, упрощает
  повторные публикации и resume на границе remote ingest;
- `Direct` убирает эту лишнюю копию и ускоряет первую remote загрузку;
- для `spec.source.upload` effective source boundary уже является staged
  object, поэтому режим не создаёт вторую промежуточную копию поверх upload
  staging.

Отдельного выбора транспорта для публикуемых слоёв больше нет.
Канонический byte path теперь один:

- `publish-worker -> DMCR direct-upload v2 session -> physical multipart object -> DMCR trusted S3 full-object digest, если доступен, иначе verification read -> canonical digest metadata/link`.

`DMCR` остаётся владельцем аутентификации, финализации blob/link и итогового
артефактного контракта, но толстый поток байтов больше не идёт через registry
`PATCH` path. Это убирает сам `DMCR` из роли сетевого узкого места на
пути публикации крупных байтов.

Прямой вспомогательный процесс работает в режиме позднего digest: контроллер
открывает сессию без итогового digest, отгружает части слоя, а финализирует
слой на `complete(session, expectedDigest, size, parts)`. Для raw-слоёв с
чтением по диапазонам это убирает старое полное предварительное чтение
исходника на стороне контроллера: байты модели из источника `publish-worker`
читает один раз. После завершения multipart-сборки `DMCR` делает отдельный
проверочный проход по уже собранному физическому объекту в объектном
хранилище, сам считает итоговый `sha256` и фактический размер, а
`expectedDigest` от контроллера использует только как дополнительную проверку.
Если S3 отдаёт `ChecksumSHA256` именно как full-object checksum, `DMCR`
использует его без второго чтения объекта. `ETag`, multipart part checksum и
composite checksum не считаются безопасным OCI `sha256` digest: при их
отсутствии или неподходящем типе остаётся полный проверочный read. Если digest
не совпал, публикация отклоняется и физический объект загрузки удаляется.
Если проверочный read временно падает после успешной multipart-сборки,
физический объект не удаляется: повторный `complete` может продолжить проверку
уже собранного объекта без повторной загрузки байтов модели.

Маленькие `config`/`manifest` записи и финальный remote inspect по-прежнему
идут через обычный registry API, чтобы внутренний контракт менялся по одному
слою ответственности за раз.

Текущая реализация делает один внутренний шаг запечатывания без второй полной
записи тяжёлого объекта. Multipart upload сначала собирается в физический
ключ объекта `_ai_models/direct-upload/objects/<session-id>/data`, затем
`DMCR` сначала пытается взять доверенный full-object SHA256 из object storage,
а если хранилище его не отдаёт, один раз читает объект для проверки. После
этого он пишет маленький `.dmcr-sealed` sidecar под каноническим blob path и
repository link по вычисленному digest. Published OCI contract снаружи
остаётся digest-based: repository link указывает на канонический digest, а
внутренний `sealeds3` driver прозрачно разворачивает этот digest в физический
ключ объекта.

На стороне контроллера direct-upload теперь также ведёт один компактный
owner-scoped checkpoint `Secret`. В фазе `Running` в нём лежат ключ текущего
слоя, session token, размер части, уже загруженные байты, продолжение
хэш-состояния и журнал уже зафиксированных слоёв. Если `sourceworker` Pod
падает, пока это состояние ещё находится в `Running`, контроллер пересоздаёт
Pod, а `publish-worker` может продолжить работу из сохранённого checkpoint
плюс `listParts()`, пока жива сама helper-сессия, вместо зависимости от одного
живого процесса. Публичный running-status теперь даёт не только bounded
progress, но и машинно-читаемые running-reason в conditions:
`PublicationStarted`, `PublicationUploading`, `PublicationResumed`,
`PublicationSealing`, `PublicationCommitted`. В `message` при этом по-прежнему
лежат байты текущего слоя, если они доступны, либо количество уже
зафиксированных слоёв.

Для потоковых multi-file источников внутренняя OCI-раскладка теперь смешанная:
мелкие companion-файлы могут укладываться в один ограниченный tar-layer под
стабильным корнем `model/`, а крупные model payload остаются отдельными
raw-слоями. Это только внутреннее решение publisher/materializer.
Потребительский materialized contract остаётся стабильным: multi-file модель
по-прежнему приходит под корнем `model/`, а single-file direct input сохраняет
свой single-file entrypoint.

Успешный publication worker path больше не использует локальный workspace/PVC.
`HuggingFace` в обоих режимах и staged upload публикуются через
raw object-source или archive-source streaming semantics. Локальный bounded storage
contract для publish-worker теперь только один: `ephemeral-storage` requests и
limits контейнера для writable layer и логов.

Internal default для publication runtime рассчитан на streaming path:
одновременно до `4` worker Pods, memory request `1Gi`, memory limit `2Gi` на
worker, CPU request `1`, CPU limit `4`, ephemeral-storage request/limit `1Gi`.
Эти значения остаются internal module values, а не public `ModuleConfig`.

Публичный model API тоже намеренно минимален. Пользователь задаёт только
`spec.source`; формат, task и остальная model metadata вычисляются controller'ом
из фактического содержимого модели и проецируются в `status.resolved`.

`nodeCache` владеет managed local-storage substrate и workload-facing
SharedDirect delivery contract:

- ai-models может держать один managed `LVMVolumeGroupSet` поверх
  `sds-node-configurator`;
- ai-models может держать один managed `LocalStorageClass`, который строится по
  текущему списку ready managed `LVMVolumeGroup`;
- при включении этого slice ручное создание такого `LocalStorageClass` больше
  не нужно.
- при включении этого slice render теперь fail-fast, если в
  `global.enabledModules` нет `sds-node-configurator-crd`,
  `sds-node-configurator`, `sds-local-volume-crd` и `sds-local-volume`.

Текущий bounded contract такой:

- `nodeCache.enabled` включает managed substrate controller;
- `nodeCache.maxSize` становится per-node thin-pool budget;
- `nodeCache.sharedVolumeSize` задаёт размер per-node shared cache volume,
  который controller-owned stable runtime Pod/PVC запрашивает поверх managed
  `LocalStorageClass`;
- runtime eviction budget для node-cache runtime ограничен этим же
  `nodeCache.sharedVolumeSize`, а `nodeCache.sharedVolumeSize` не должен быть
  больше `nodeCache.maxSize`;
- `nodeCache.storageClassName`, `nodeCache.volumeGroupSetName`,
  `nodeCache.volumeGroupNameOnNode` и `nodeCache.thinPoolName` задают
  ai-models-owned имена substrate-объектов;
- `nodeCache.nodeSelector` и `nodeCache.blockDeviceSelector` — это
  обязательные `matchLabels` maps для выбора узлов и `BlockDevice`.

При `nodeCache.enabled=true` workload'ы, которые не принесли явный cache
volume, переводятся на node-cache SharedDirect path:

- контроллер inject'ит inline CSI volume
  `node-cache.ai-models.deckhouse.io` в `/data/modelcache`;
- injected CSI volume несёт только immutable artifact attributes
  (URI, digest, optional family), без storage-specific полей в публичном API;
- контроллер переносит `nodeCache.nodeSelector` в managed workload pod
  template и fail'ится при конфликте, вместо запуска на неподходящей ноде;
- контроллер дополнительно требует динамический node label
  `ai.deckhouse.io/node-cache-runtime-ready=true` и держит managed scheduling
  gate, пока ни одна выбранная нода не имеет ready runtime Pod и bound shared
  cache PVC;
- namespace workload'а больше не получает projected DMCR read Secret/CA и
  bridge runtime imagePullSecret для managed SharedDirect path;
- workload получает стабильный runtime-facing contract. Legacy annotations
  `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel` по-прежнему
  отдают primary model через `AI_MODELS_MODEL_PATH`,
  `AI_MODELS_MODEL_DIGEST` и `AI_MODELS_MODEL_FAMILY`; для нескольких моделей
  используется `ai.deckhouse.io/model-refs` со значением вроде
  `main=Model/gemma,embed=ClusterModel/bge`, после чего каждая модель видна по
  стабильному alias path `/data/modelcache/models/<alias>`, а workload получает
  `AI_MODELS_MODELS_DIR`, `AI_MODELS_MODELS` со списком
  alias/path/digest/family и per-alias env
  `AI_MODELS_MODEL_<ALIAS>_{PATH,DIGEST,FAMILY}`;
- контроллер теперь дополнительно пишет в `PodTemplateSpec` управляемые
  аннотации с выбранным режимом доставки и причиной этого выбора, поэтому
  node-cache runtime может находить desired artifacts по live Pod'ам на своей
  ноде;
- метрики `runtimehealth` теперь также агрегируют управляемые прикладные
  объекты по namespace, виду, режиму доставки и причине выбора, поэтому
  оператор видит, где workload ещё живёт на явном legacy bridge storage, а где
  уже используется SharedDirect, без ручного обхода объектов;
- ai-models теперь держит отдельный per-node shared cache plane как
  controller-owned stable runtime Pod плюс stable PVC поверх managed
  `LocalStorageClass`; размер этого shared volume задаётся через
  `nodeCache.sharedVolumeSize`, а storage identity больше не теряется при
  restart node-agent pod'а;
- runtime Pod кэша не использует прямой `spec.nodeName`: он привязан к нужной
  ноде через node affinity по `kubernetes.io/hostname`, чтобы Kubernetes
  scheduler мог корректно выбрать local LVM volume для PVC с
  `WaitForFirstConsumer`;
- controller передаёт в runtime Pod образ `node-driver-registrar` из Deckhouse
  common CSI image set, а сам runtime Pod поднимает kubelet-facing CSI socket
  `/var/lib/kubelet/csi-plugins/node-cache.ai-models.deckhouse.io/csi.sock`;
- runtime container использует отдельный internal `nodeCacheRuntime`
  distroless image, а не общий publication/materialize runtime image;
- CSI NodePublish делает read-only bind mount готового digest store из
  per-node shared cache PVC в kubelet target path; если digest ещё не
  materialized, CSI возвращает transient `Unavailable`, а kubelet повторяет
  mount после prefetch;
- CSI NodePublish fail-closed проверяет `podInfoOnMount`: mount разрешён только
  для того live Pod на той же ноде, который controller уже пометил как
  managed SharedDirect Pod с тем же digest;
- `node-cache-runtime` сам получает набор опубликованных артефактов, реально
  нужных live SharedDirect managed Pod'ам на текущей ноде, и prefetch'ит их в
  shared node-local digest store;
- transient ошибки prefetch/materialization retry'ятся per digest с in-memory
  backoff, поэтому один недоступный artifact не рестартит runtime Pod и не
  блокирует другие digest'ы.

Отказные сценарии:

- если SDS-модули не включены в `global.enabledModules`, render падает до
  выкатки модуля;
- если SDS CRD фактически отсутствуют в кластере, substrate controller не
  сможет создать `LVMVolumeGroupSet` / `LocalStorageClass`, и node-cache слой
  не станет готовым;
- если на выбранной ноде нет подходящего local block device по
  `nodeCache.blockDeviceSelector`, managed `LVMVolumeGroup` для этой ноды не
  станет ready, `LocalStorageClass` не получит эту ноду, а runtime PVC/Pod для
  кэша останутся unscheduled или pending;
- пока ни одна выбранная нода не имеет ready runtime Pod и bound shared cache
  PVC, managed SharedDirect workload templates сохраняют ai-models scheduling
  gate вместо rollout Pod'ов, которые зависнут на CSI mount.

Явно заданные workload cache volumes пока остаются legacy bridge path: они
по-прежнему используют `materialize-artifact` и digest-scoped shared-PVC
bridge logic там, где применимо. После cutover это не managed default path.

При этом публичного cleanup/TTL knob пока нет: workload-facing shared mount уже
есть, но eviction policy остаётся internal runtime behavior, а не обещанным
user-facing SLA.
