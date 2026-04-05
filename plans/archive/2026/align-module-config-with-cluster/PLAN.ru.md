# План работ: align module config with cluster

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Зафиксировать текущий config surface ai-models

### Цель

Понять, что уже реализовано в `openapi/` и `templates/` для PostgreSQL и
artifacts, чтобы не проектировать поверх несуществующей схемы.

### Изменяемые области

- `plans/active/align-module-config-with-cluster/`

### Проверки

- чтение `openapi/config-values.yaml`
- чтение `openapi/values.yaml`
- чтение relevant templates/helpers

### Артефакт

Понятно, какие значения уже являются module contract, а какие runtime-only.

## Slice 2. Свериться с референсом n8n-d8

### Цель

Взять production-oriented pattern для managed PostgreSQL и S3-compatible
configuration из рабочего DKP module.

### Изменяемые области

- без изменений кода, если хватает read-only анализа

### Проверки

- чтение relevant files в `n8n-d8`

### Артефакт

Понятен acceptable baseline для user-facing config и runtime wiring.

## Slice 3. Посмотреть capabilities target cluster

### Цель

Понять, что реально доступно в кластере `k8s.apiac.ru`: managed-postgres APIs,
PostgresClass, storage classes, возможно существующий RGW wiring.

### Изменяемые области

- без изменений репозитория

### Проверки

- `kubectl --kubeconfig /Users/myskat_90/.kube/k8s-config ...`

### Артефакт

Есть concrete facts по PostgreSQL prerequisites и S3/RGW integration points.

## Slice 4. Подготовить минимальный рабочий baseline

### Цель

Сформировать минимальный production-oriented config baseline для phase-1:
managed PostgreSQL и S3-compatible artifacts.

### Изменяемые области

- при необходимости `openapi/config-values.yaml`
- при необходимости `openapi/values.yaml`
- при необходимости `templates/*`
- при необходимости `docs/*`

### Проверки

- `make lint`
- `make helm-template`

### Артефакт

Repo contract и cluster-oriented guidance согласованы между собой.

## Rollback point

После Slice 3. Cluster facts собраны, но repo contract ещё не менялся.

## Final validation

- `make lint`
- `make helm-template`
