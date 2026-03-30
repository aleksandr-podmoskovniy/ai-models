---
title: "Конфигурация"
menuTitle: "Конфигурация"
weight: 60
---

<!-- SCHEMA -->

Текущий конфигурационный контракт `ai-models` намеренно короткий.
На уровне модуля наружу выставляются только стабильные ai-models-specific настройки:
логирование, wiring для PostgreSQL и S3-compatible artifact storage.

`postgresql.mode` поддерживает два phase-1 сценария:

- `Managed`: поднять внутренний PostgreSQL через Deckhouse `managed-postgres`;
- `External`: подключить ai-models к существующему PostgreSQL, используя пароль
  из уже созданного Secret.

Базовый managed profile намеренно маленький: по умолчанию он использует уже
существующий cluster-wide `PostgresClass`, запрашивает том на 5Gi и держит
минимальный resource profile для phase-1 metadata storage.
Имена database и user по умолчанию теперь `ai-models`, а HA topology для
managed `Postgres` берётся из `PostgresClass.defaultTopology`, а не
хардкодится на стороне модуля.

`artifacts` определяет S3-compatible backend для артефактов ai-models: bucket,
path prefix, endpoint URL, region, TLS policy, addressing style и credentials.

Учётные данные для artifact storage можно задавать двумя способами:

- через `credentialsSecretName`, указывающий на уже существующий Secret в
  `d8-ai-models` с фиксированными ключами `accessKey` и `secretKey`;
- через inline `accessKey` и `secretKey` в ModuleConfig, после чего модуль сам
  создаёт внутренний Secret в `d8-ai-models`.

`bucket`, `pathPrefix`, `endpoint`, `region` и флаги addressing/TLS не считаются
секретами и остаются обычной частью module configuration contract.

Режим доступности, HTTPS policy, ingress behavior и Dex SSO берутся из global
Deckhouse configuration и internal module wiring.
Текущий runtime ожидает:

- настроенный `global.modules.publicDomainTemplate`;
- глобально включённый HTTPS через Deckhouse module HTTPS policy
  (`CertManager` или `CustomCertificate`);
- модуль `user-authn` для module SSO;
- модуль `managed-postgres`, если `postgresql.mode=Managed`.

`Model` и `ClusterModel` пока не входят в текущий user-facing контракт.
Они появятся позже, когда для этого будет готов стабильный module-level API.
