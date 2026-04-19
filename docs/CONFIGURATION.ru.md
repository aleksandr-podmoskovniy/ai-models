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
  `artifacts.sourceAcquisitionMode=Mirror`, для временного source mirror;
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

- `Model` / `ClusterModel` используют один cluster-level
  `artifacts.sourceAcquisitionMode`:
  - `Mirror`:
    remote `source.url` сначала идёт через controller-owned source mirror;
  - `Direct`:
    remote `source.url` идёт напрямую из canonical remote source boundary;
- `spec.source.upload` использует controller-owned upload-session path и
  остаётся на своей staged object boundary под тем же acquisition contract;
- все пути публикуют OCI `ModelPack` артефакты во внутренний `DMCR`.

Default — `artifacts.sourceAcquisitionMode=Direct`.

Trade-off между режимами такой:

- `Mirror` сохраняет durable промежуточную копию в object storage, упрощает
  повторные публикации и resume на границе remote ingest;
- `Direct` убирает эту лишнюю копию и ускоряет первую remote загрузку;
- для `spec.source.upload` effective source boundary уже является staged
  object, поэтому режим не создаёт вторую промежуточную копию поверх upload
  staging.

Для публикации тяжёлых layer blobs внутрь `DMCR` отдельного выбора больше нет.
Канонический byte path теперь один:

- `publish-worker -> DMCR direct-upload helper -> backing storage DMCR`.

`DMCR` остаётся владельцем аутентификации, финализации blob/link и итогового
артефактного контракта, но толстый поток байтов больше не идёт через registry
`PATCH` path. Это убирает сам `DMCR` из роли сетевого узкого места на
тяжёлом upload path.

Текущий bounded scope прямого транспорта касается тяжёлых layer blobs. `config`
blob, `manifest` publish и финальный remote inspect остаются на обычном
registry path, чтобы контракт менялся по одному слою ответственности за раз.

Успешный publication worker path больше не использует локальный workspace/PVC.
`HuggingFace` в обоих режимах и staged upload публикуются через
object-source/archive-source streaming semantics. Локальный bounded storage
contract для publish-worker теперь только один: `ephemeral-storage` requests и
limits контейнера для writable layer и логов.

Публичный model API тоже намеренно минимален. Пользователь задаёт только
`spec.source`; формат, task и остальная model metadata вычисляются controller'ом
из фактического содержимого модели и проецируются в `status.resolved`.
