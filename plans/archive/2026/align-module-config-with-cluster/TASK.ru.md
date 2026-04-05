# Выровнять module config ai-models под реальный кластер и phase-1 runtime

## Контекст

Модуль `ai-models` уже имеет user-facing contract для `postgresql` и
`artifacts`, а также runtime wiring для managed/external PostgreSQL и
S3-compatible storage.

Теперь нужно привязать этот contract к реальному кластеру
`k8s.apiac.ru` через kubeconfig `/Users/myskat_90/.kube/k8s-config`: понять,
какие platform capabilities уже есть, как лучше задать managed PostgreSQL для
phase-1, и как подключать S3-compatible backend через Ceph RGW в окружении PVE.

## Постановка задачи

Нужно собрать production-oriented baseline для module config:
- проверить текущий `ai-models` config surface;
- свериться с patterns из `n8n-d8`;
- посмотреть cluster capabilities для `managed-postgres` и storage;
- определить минимальный рабочий contract для PostgreSQL и S3;
- при необходимости подготовить repo changes и/или concrete cluster config.

## Scope

- `plans/active/align-module-config-with-cluster/`
- read-only inspection `ai-models` repo
- read-only inspection `n8n-d8`
- read-only inspection target cluster via provided kubeconfig
- при необходимости: `openapi/config-values.yaml`, `openapi/values.yaml`,
  `templates/*`, `docs/*`, fixtures

## Non-goals

- не делать phase-2 API (`Model` / `ClusterModel`);
- не внедрять новые backend features вне PostgreSQL/S3 wiring;
- не менять cluster state без явной необходимости и без понятного rollback.

## Затрагиваемые области

- module config contract;
- runtime templates для PostgreSQL и artifacts;
- cluster prerequisites и platform integration assumptions.

## Критерии приёмки

- понятен текущий рабочий baseline для PostgreSQL в этом кластере;
- понятен способ интеграции с Ceph RGW как S3-compatible backend;
- если repo contract требует правок, они минимальны и соответствуют cluster reality;
- есть concrete next steps для применения module config в кластере.

## Риски

- реальный cluster state может не совпасть с assumptions в templates;
- RGW endpoint/credentials могут требовать ручной подготовки вне модуля;
- managed-postgres CRD и class policy в кластере могут ограничивать размер и shape инстанса.
