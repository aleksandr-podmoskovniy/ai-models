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

- `raw/` для controller-owned upload staging и source mirror;
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

- `Model` / `ClusterModel` с `spec.source.url` забирают remote bytes через
  controller-owned source mirror path;
- `spec.source.upload` использует controller-owned upload-session path;
- оба пути публикуют OCI `ModelPack` артефакты во внутренний `DMCR`.

Publication workspace по умолчанию теперь `PersistentVolumeClaim`, а не
`EmptyDir`. Если `storageClassName` не задан, сгенерированный PVC использует
default `StorageClass` кластера. Значит публикация больших моделей теперь
требует достаточной persistent storage capacity, а не опоры на node ephemeral
storage.

Публичный model API тоже намеренно минимален. Пользователь задаёт только
`spec.source`; формат, task и остальная model metadata вычисляются controller'ом
из фактического содержимого модели и проецируются в `status.resolved`.
