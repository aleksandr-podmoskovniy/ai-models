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

`artifacts` определяет S3-compatible backend для артефактов ai-models: bucket,
path prefix, endpoint URL, region, TLS policy, addressing style и Secret с
учётными данными доступа.

Режим доступности, HTTPS policy, выбор сертификатов, ingress behavior и Dex SSO
берутся из global Deckhouse configuration и internal module wiring.
Текущий runtime ожидает:

- настроенный `global.modules.publicDomainTemplate`;
- глобально включённый HTTPS;
- модуль `user-authn` для module SSO;
- модуль `managed-postgres`, если `postgresql.mode=Managed`.

`Model` и `ClusterModel` пока не входят в текущий user-facing контракт.
Они появятся позже, когда для этого будет готов стабильный module-level API.
