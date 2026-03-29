# Phase 1: DKP-native runtime для managed MLflow в модуле ai-models

## Контекст

В репозитории уже есть:
- DKP module root для `ai-models`;
- upstream-like build shell для собственного образа `mlflow`;
- минимальные `config-values` / `values` для `postgresql` и `artifacts`.

Следующий шаг больше нельзя откладывать: модуль должен перестать быть shell-проектом и получить рабочий phase-1 runtime, который можно собрать, опубликовать как модуль и развернуть в кластере.

Bitnami chart из `/Users/myskat_90/flant/aleksandr-podmoskovniy/charts/bitnami/mlflow` используется как donor по runtime semantics, а не как dependency или готовый chart runtime. DKP library и проектные паттерны берутся из `deckhouse_lib_helm`, `deckhouse/modules/*` и `n8n-d8`.

## Постановка задачи

Нужно реализовать DKP-native runtime для `ai-models` phase 1:
- поднять внутренний `MLflow Tracking/UI` как рабочий backend модуля;
- подключить его к managed или external PostgreSQL;
- подключить его к S3-compatible artifact storage;
- сделать ingress/HTTPS по global Deckhouse settings;
- подключить login в UI через Dex SSO по паттернам Deckhouse;
- подготовить модуль к сборке/publish/deploy без использования donor chart как runtime dependency.

## Scope

- доработать `config-values` / `values` под рабочий runtime contract для PostgreSQL и S3;
- собрать DKP-native templates для `Deployment`, `Service`, `Ingress`, `ServiceAccount`, `ConfigMap`, `PDB`, `ServiceMonitor`, `DexAuthenticator`, `managed-postgres` ресурсов;
- перенести из donor chart только поведенческую семантику запуска `mlflow server`, DB upgrade и S3 wiring;
- подключить `deckhouse_lib_helm` helpers для image, HTTPS, ingress class, HA, labels и security context;
- добавить module hook для copy custom certificate, чтобы global HTTPS mode не ломал ingress TLS wiring;
- обновить bundle/release wiring, чтобы hooks попадали в bundle;
- обновить docs так, чтобы текущий модуль описывался как deployable phase-1 capability.

## Non-goals

- не внедрять `Model`, `ClusterModel` и phase-2 controller/API;
- не тащить Bitnami chart как subchart, dependency или vendored runtime;
- не включать `mlflow run` component как обязательную часть phase-1 baseline;
- не делать distroless, supply-chain hardening и другие phase-3 задачи;
- не вводить отдельные user-facing overrides для HA, ingress class, certificate source или Dex auth policy поверх global DKP settings;
- не превращать phase-1 runtime в публичный контракт сырого MLflow.

## Затрагиваемые области

- `plans/phase-1-mlflow-runtime/`
- `openapi/`
- `templates/`
- `fixtures/`
- `hooks/`
- `.werf/stages/`
- `werf.yaml`
- `Makefile`
- `Taskfile.yaml`
- `docs/`
- `README*`
- `DEVELOPMENT.md`

## Критерии приёмки

- модуль рендерит рабочий phase-1 runtime для `MLflow Tracking/UI`;
- образ `mlflow` подключается через `helm_lib_module_image`, а deployment shape совместим с текущим upstream build shell;
- user-facing contract для `postgresql` и `artifacts` остаётся коротким, но достаточным для реального deploy;
- managed PostgreSQL wiring и external PostgreSQL wiring поддерживаются без ad-hoc ручных правок templates;
- S3-compatible storage wiring поддерживает bucket, endpoint, region, TLS policy и credentials secret;
- ingress использует Deckhouse global HTTPS settings и ingress class, а UI может быть защищён через Dex SSO по DKP patterns;
- global `CustomCertificate` mode не ломает модульный ingress и bundle;
- repo-level проверки снова проходят, а runtime templates не ломают текущий verify loop.

## Риски

- `managed-postgres`, `DexAuthenticator`, `Certificate` и `ServiceMonitor` — это CRD-based ресурсы, их рендер и валидация должны быть аккуратно загейтованы по доступным API;
- слишком точное копирование Bitnami chart быстро превратит модуль в чужую архитектуру вместо DKP-native runtime;
- слишком слабая validation story для custom resources может дать «зелёный» CI при сломанном runtime;
- неаккуратный auth path может смешать ingress SSO и внутреннюю auth-механику MLflow и усложнить эксплуатацию.
