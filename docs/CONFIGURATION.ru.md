---
title: "Конфигурация"
menuTitle: "Конфигурация"
weight: 60
---

<!-- SCHEMA -->

Текущий конфигурационный контракт `ai-models` намеренно короткий.
На уровне модуля наружу выставляются только стабильные настройки:

- `logLevel`;
- `artifacts`;
- `nodeCache`.

Режим HA, HTTPS policy, ingress class, controller/runtime wiring, внутренний
`DMCR`, upload-gateway, publication worker, source-fetch policy и GC cadence
остаются во global Deckhouse settings и internal module values. В
user-facing contract нет:

- retired backend auth/workspace и metadata-database knobs;
- browser SSO knobs;
- backend-only secrets;
- внешнего publication registry contract;
- backend-specific `artifacts.pathPrefix`.
- настроек реализации `DMCR`;
- выбора source-fetch транспорта;
- внутренних имён node-cache storage objects.

`artifacts` задаёт общий S3-compatible storage для byte-path внутри ai-models.
Разделение внутри bucket фиксировано самим runtime:

- `raw/` для controller-owned upload staging и временных source objects там,
  где они нужны самому модулю;
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

Публичный runtime path для моделей теперь controller-owned:

- `Model` / `ClusterModel` remote `source.url` ingestion использует
  module-owned policy. Текущий default — direct streaming из canonical source
  boundary; модуль может использовать временные source objects внутри себя,
  если конкретному adapter'у это нужно для resume или безопасности;
- `spec.source.upload` использует controller-owned upload-session path и
  остаётся на своей отдельной staged object boundary; временные secret upload
  URL публикуются в status по прямому UX из virtualization;
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
- internal service-account RBAC не aggregate'ится в человеческие роли; `DMCR`
  garbage collection читает только module-private cleanup/GC `Secret` и
  `Lease` в namespace модуля, а не user-facing `Model` или `ClusterModel`.

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
`spec.source`; формат, normalized endpoint/features и source capability
evidence вычисляются controller'ом из фактического содержимого модели и
проецируются в `status.resolved`.

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
- `nodeCache.size` — единственное решение по per-node cache capacity. Модуль
  использует это же значение для managed thin-pool budget, per-node shared
  cache PVC и runtime eviction budget;
- по умолчанию ai-models выбирает ноды и `BlockDevice` с label
  `ai.deckhouse.io/model-cache=true`;
- `nodeCache.nodeSelector` и `nodeCache.blockDeviceSelector` можно
  переопределить только если в кластере уже есть более строгая схема
  лейблинга;
- `LocalStorageClass`, `LVMVolumeGroupSet`, VG и thin-pool names — внутренние
  constants ai-models, а не public ModuleConfig knobs.

При `nodeCache.enabled=true` SharedDirect является annotation-only контрактом
workload:

- metadata workload'а задаёт только `ai.deckhouse.io/model`,
  `ai.deckhouse.io/clustermodel` или `ai.deckhouse.io/model-refs`;
- controller сам добавляет inline CSI volume с driver
  `node-cache.ai-models.deckhouse.io`, stable mount, runtime env и internal
  artifact attributes;
- для одной модели managed volume name — `ai-models-node-cache`; для
  `ai.deckhouse.io/model-refs` каждый alias получает
  `ai-models-node-cache-<alias>`;
- controller не добавляет node selectors, labels, affinity или scheduling
  policy workload'а. Placement остаётся ответственностью workload'а,
  ai-inference или внешнего scheduler'а;
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
- ai-models не держит CRD-specific adapters для внешних контроллеров вроде
  KubeRay. Надо рендерить поддержанный Kubernetes workload с model annotation
  на metadata workload'а либо отдавать рендеринг workload'а ai-inference;
- контроллер теперь дополнительно пишет в `PodTemplateSpec` управляемые
  аннотации с выбранным режимом доставки и причиной этого выбора, поэтому
  node-cache runtime может находить desired artifacts по live Pod'ам на своей
  ноде;
- метрики `runtimehealth` теперь также агрегируют управляемые прикладные
  объекты по namespace, виду, режиму доставки и причине выбора, поэтому
  оператор видит, какие workload управляются через SharedDirect и какая
  причина доставки активна, без ручного обхода объектов;
- ai-models теперь держит отдельный per-node shared cache plane как
  controller-owned stable runtime Pod плюс stable PVC поверх managed
  `LocalStorageClass`; размер этого shared volume задаётся через
  `nodeCache.size`, а storage identity больше не теряется при restart
  node-agent pod'а;
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
- CSI NodePublish fail-closed проверяет `podInfoOnMount` и controller-only
  HMAC signature поверх resolved delivery annotations: mount разрешён только
  для того live Pod на той же ноде, который controller пометил как managed
  SharedDirect Pod с тем же digest;
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
- если workload попал на ноду без ready node-cache runtime и подходящего local
  storage, CSI mount будет fail/wait через kubelet events. ai-models не
  выводит и не inject'ит placement workload'а; placement должен задать сам
  workload, ai-inference или внешний scheduler;
- если managed node-cache delivery выключен или набор моделей не помещается в
  configured per-node cache size, ai-models сохраняет свой scheduling gate и
  пишет понятный condition/event.

Явно заданные workload cache volumes больше не являются delivery fallback:
controller отклоняет такие шаблоны понятным condition/event и ждёт managed
SharedDirect CSI contract. Пользовательские манифесты должны задавать только
аннотацию модели. Уже существующий mount на `/data/modelcache` допустим только
если он использует node-cache CSI driver; иначе он отклоняется, а не
конвертируется молча.

При этом публичного cleanup/TTL knob пока нет: workload-facing shared mount уже
есть, но eviction policy остаётся internal runtime behavior, а не обещанным
user-facing SLA.
