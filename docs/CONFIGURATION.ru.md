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
ежедневно в 02:00. Пустая строка отключает периодический sweep, но не убирает
operator-facing inspection surface: внутри Pod'а `DMCR` по-прежнему можно
запустить `dmcr-cleaner gc check` и получить report по stale published
repository prefix и source-mirror prefix.

Публичный runtime path для моделей теперь controller-owned:

- `Model` / `ClusterModel` используют один cluster-level
  `artifacts.sourceFetchMode`:
  - `Mirror`:
    remote `source.url` сначала идёт через controller-owned source mirror;
  - `Direct`:
    remote `source.url` идёт напрямую из canonical remote source boundary;
- `spec.source.upload` использует controller-owned upload-session path и
  остаётся на своей отдельной staged object boundary;
- все пути публикуют OCI `ModelPack` артефакты во внутренний `DMCR`;
- потоковые multi-file remote входы публикуются как одна ограниченная bundle-
  упаковка для мелких companion-файлов плюс отдельные raw-слои для крупных
  model payload, без монолитной перепаковки всей модели в один tar-слой;
- single-file direct и staged-object входы по-прежнему публикуются одним
  raw-слоем;
- archive-входы остаются на archive-source streaming path и не создают
  распакованное success-only дерево checkpoint'а.

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

- `publish-worker -> DMCR direct-upload v2 session -> physical multipart object -> DMCR verification read -> canonical digest metadata/link`.

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
Если digest не совпал, публикация отклоняется и физический объект загрузки
удаляется.
Если проверочный read временно падает после успешной multipart-сборки,
физический объект не удаляется: повторный `complete` может продолжить проверку
уже собранного объекта без повторной загрузки байтов модели.

Маленькие `config`/`manifest` записи и финальный remote inspect по-прежнему
идут через обычный registry API, чтобы внутренний контракт менялся по одному
слою ответственности за раз.

Текущая реализация делает один внутренний шаг запечатывания без второй полной
записи тяжёлого объекта. Multipart upload сначала собирается в физический
ключ объекта `_ai_models/direct-upload/objects/<session-id>/data`, затем
`DMCR` один раз читает этот объект для проверки, пишет маленький
`.dmcr-sealed` sidecar под каноническим blob path и repository link по
вычисленному digest. Published OCI contract снаружи остаётся digest-based:
repository link указывает на канонический digest, а внутренний `sealeds3`
driver прозрачно разворачивает этот digest в физический ключ объекта.

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

Публичный model API тоже намеренно минимален. Пользователь задаёт только
`spec.source`; формат, task и остальная model metadata вычисляются controller'ом
из фактического содержимого модели и проецируются в `status.resolved`.

`nodeCache` — это первый landed slice для node-local cache workstream. В
текущем состоянии он владеет managed local-storage substrate и current local
materialize-bridge volume contract:

- ai-models может держать один managed `LVMVolumeGroupSet` поверх
  `sds-node-configurator`;
- ai-models может держать один managed `LocalStorageClass`, который строится по
  текущему списку ready managed `LVMVolumeGroup`;
- при включении этого slice ручное создание такого `LocalStorageClass` больше
  не нужно.

Текущий bounded contract такой:

- `nodeCache.enabled` включает managed substrate controller;
- `nodeCache.maxSize` становится per-node thin-pool budget;
- `nodeCache.fallbackVolumeSize` задаёт размер managed local ephemeral volume,
  который current workload delivery автоматически подкладывает на
  `/data/modelcache` для переходного materialize-пути, если annotated workload
  не принёс свой cache volume сам;
- `nodeCache.sharedVolumeSize` задаёт размер per-node shared cache volume,
  который controller-owned stable runtime Pod/PVC запрашивает поверх managed
  `LocalStorageClass`;
- `nodeCache.storageClassName`, `nodeCache.volumeGroupSetName`,
  `nodeCache.volumeGroupNameOnNode` и `nodeCache.thinPoolName` задают
  ai-models-owned имена substrate-объектов;
- `nodeCache.nodeSelector` и `nodeCache.blockDeviceSelector` — это
  `matchLabels` maps для выбора узлов и `BlockDevice`.

Этот slice всё ещё не заменяет live workload delivery path workload-facing
node-shared mount service'ом. Workload'ы по-прежнему materialize'ятся через
controller-owned `materialize-artifact` в `/data/modelcache`, но теперь:

- ai-models может сам inject'ить local generic ephemeral volume поверх managed
  `LocalStorageClass`, если workload не принёс свою cache topology;
- вместе с OCI read auth/CA контроллер теперь также проецирует в namespace
  workload'а `imagePullSecret` для самого bridge runtime, поэтому
  `materialize-artifact` больше не зависит от ручного создания отдельного
  pull-secret рядом с каждым consumer workload'ом;
- workload теперь получает один стабильный runtime-facing contract через
  `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST` и
  `AI_MODELS_MODEL_FAMILY`; при этом per-pod доставка по-прежнему проецирует
  стабильный путь `/data/modelcache/model`, а shared PVC bridge topology
  теперь проецирует digest-специфичный путь внутри общего store вместо
  глобальной ссылки `current` в корне кэша;
- контроллер теперь дополнительно пишет в `PodTemplateSpec` управляемые
  аннотации с выбранным режимом доставки и причиной этого выбора, поэтому
  переходный materialize-путь больше не остаётся скрытым поведением;
- метрики `runtimehealth` теперь также агрегируют управляемые прикладные
  объекты по namespace, виду, режиму доставки и причине выбора, поэтому
  оператор видит, где workload ещё живёт на переходном materialize-пути, а где
  уже используется shared PVC bridge, без ручного обхода объектов;
- `runtimehealth` теперь также публикует число managed Pod'ов, число ready
  Pod'ов и причины ожидания `init`-контейнера `materialize-artifact`, поэтому
  `ImagePullBackOff` и похожие сбои bridge path становятся видны машинно, без
  ручного разбора Pod events;
- ai-models теперь держит отдельный per-node shared cache plane как
  controller-owned stable runtime Pod плюс stable PVC поверх managed
  `LocalStorageClass`; размер этого shared volume задаётся через
  `nodeCache.sharedVolumeSize`, а storage identity больше не теряется при
  restart node-agent pod'а;
- `node-cache-runtime` сам получает набор опубликованных артефактов, реально
  нужных live managed Pod'ам на текущей ноде только для будущего true
  shared-direct режима; current shared PVC bridge этот per-node plane ещё не
  потребляет, поэтому `prefetch` в узловой общий store пока не считается
  workload-facing delivery path.

При этом публичного cleanup/TTL knob пока нет: workload-facing shared mount
contract ещё не landed, поэтому eviction policy остаётся internal runtime
behavior, а не обещанным user-facing SLA.
