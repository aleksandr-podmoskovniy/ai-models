---
title: "Конфигурация"
menuTitle: "Конфигурация"
weight: 60
---

<!-- SCHEMA -->

Текущий конфигурационный контракт `ai-models` намеренно короткий.
На уровне модуля наружу выставляются только стабильные ai-models-specific настройки:
логирование, настройки Deckhouse SSO, wiring для PostgreSQL и S3-compatible artifact
storage.

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
Для browser SSO и MLflow permissions модуль также использует отдельную logical
auth database в том же PostgreSQL instance. В режиме `Managed` модуль создаёт
эту вторую БД автоматически по шаблону `<database>-auth`. В режиме `External`
existing PostgreSQL должен уже содержать такую вторую БД.

`artifacts` определяет S3-compatible backend для артефактов ai-models: bucket,
path prefix, endpoint URL, region, TLS policy, addressing style и credentials.

Учётные данные для artifact storage можно задавать двумя способами:

- через `credentialsSecretName`, указывающий на уже существующий Secret в
  `d8-ai-models` с фиксированными ключами `accessKey` и `secretKey`;
- через inline `accessKey` и `secretKey` в ModuleConfig, после чего модуль сам
  создаёт внутренний Secret в `d8-ai-models`.

Custom CA для S3-compatible endpoint задаётся отдельно через
`artifacts.caSecretName`. Этот Secret должен находиться в `d8-ai-models` и
содержать ключ `ca.crt`. Если `caSecretName` пустой, ai-models сначала
автоматически reuse'ит `credentialsSecretName`, если тот же Secret также
содержит `ca.crt`, а иначе fallback'ится на общий platform CA, который уже
discovered для Dex или скопирован из global HTTPS `CustomCertificate` path.

`bucket`, `pathPrefix`, `endpoint`, `region` и флаги addressing/TLS не считаются
секретами и остаются обычной частью module configuration contract.

Режим доступности и HTTPS policy берутся из global Deckhouse configuration и
internal module wiring.
Текущий runtime ожидает:

- настроенный `global.modules.publicDomainTemplate`;
- глобально включённый HTTPS через Deckhouse module HTTPS policy
  (`CertManager` или `CustomCertificate`);
- модуль `managed-postgres`, если `postgresql.mode=Managed`.

Browser login теперь идёт через Deckhouse Dex OIDC SSO внутри самого MLflow.
Модуль автоматически настраивает:

- `DexClient` в `d8-ai-models` с redirect URI `https://<public-host>/callback`;
- public Dex discovery URL `https://dex.<cluster-domain>/.well-known/openid-configuration`;
- автоматическое platform CA trust wiring из discovered Dex CA или global HTTPS
  `CustomCertificate` path для TLS OIDC и S3;
- вход в MLflow через `mlflow-oidc-auth`;
- upstream-native MLflow workspaces.

Настройки `auth.sso.allowedGroups` и `auth.sso.adminGroups` определяют, какие
Deckhouse группы вообще могут заходить в ai-models и какие из них становятся
MLflow administrators после SSO login. Базовый default намеренно
консервативен: внутрь допускается только группа Deckhouse `admins`, и она же
становится MLflow admin group.

Модуль всегда создаёт внутренний auth Secret со:

- internal machine username в ключе `machineUsername`;
- стабильным сгенерированным machine password в ключе `machinePassword`;
- стабильным session secret для MLflow auth runtimes.

Этот Secret теперь остаётся только machine-only путём для `ServiceMonitor`,
in-cluster import Jobs и break-glass operations, а browser users идут через Dex SSO.

Из-за этого raw backend service больше не защищён только на ingress-уровне.
Даже при прямом доступе к service нужны MLflow machine credentials, а логическая
сегментация по-прежнему идёт через native MLflow workspaces.

Большие machine-oriented import flows теперь используют direct artifact access
вместо server-side artifact proxying. Backend запускается с
`--no-serve-artifacts`, а in-cluster import Jobs ходят в MLflow metadata APIs,
но пишут artifacts напрямую в S3. Backend и import Jobs используют один и тот
же merged trust bundle для Dex OIDC и S3 CA overrides, поэтому
`artifacts.insecure: true` остаётся только временным troubleshooting path, а не
целевым steady-state режимом.

Текущий phase-1 runtime profile намеренно консервативен:
каждый backend pod запускает один MLflow web worker, а MLflow server job
execution отключён. High availability backend достигается через Deckhouse
module HA и несколько pod replicas, а не через лишние in-process workers и
genai job consumers.

Backend также оставляет включённым upstream security middleware MLflow.
Модуль вычисляет `allowed-hosts` и same-origin CORS policy от публичного
ingress domain и при этом сохраняет private-network/service паттерны, нужные
для внутрикластерного доступа. Health probes используют upstream
неаутентифицированный `/health`, а `ServiceMonitor` ходит в `/metrics` через
внутренний machine account.

Модуль также создаёт внутренний Secret со стабильным значением
`MLFLOW_CRYPTO_KEK_PASSPHRASE` для upstream crypto-backed runtime features
MLflow. Это убирает небезопасный upstream default passphrase из shared cluster
deployments и при этом не выводит KEK в user-facing contract модуля.

`Model` и `ClusterModel` пока не входят в текущий user-facing контракт.
Они появятся позже, когда для этого будет готов стабильный module-level API.
