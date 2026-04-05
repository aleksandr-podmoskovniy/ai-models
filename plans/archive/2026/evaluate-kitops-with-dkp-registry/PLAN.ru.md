# План: оценка KitOps/ModelPack в связке с dkp-registry

## Current phase

Этап 1. Managed backend inside the module. Задача не меняет deployment shape `ai-models`, а критически уточняет более релевантный v0/v1 path для packaging/distribution и serving.

## Slices

### Slice 1. Проверить оба проекта `dkp-registry`

Цель:
- понять реальные границы `bundle-registry` и `payload-registry`.

Файлы/области:
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/bundle-registry/README.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/payload-registry/README.md`
- docs/templates/openapi `payload-registry`

Проверки:
- чтение README/docs/templates/openapi.

Результат:
- список фактов о storage/auth/RBAC/namespace semantics.

### Slice 2. Сверить `payload-registry` с потребностями `KitOps`

Цель:
- понять, насколько это хороший target для `ModelKit` push/pull и in-cluster auth.

Файлы/области:
- docs `AUTH.md`, `KUBERNETES-API.md`, `IMAGES-IN-CLUSTER.md`
- templates `registry/*`

Проверки:
- чтение auth model и registry shape.

Результат:
- вывод, какие registry/auth свойства реально полезны для ModelKit/KServe.

### Slice 3. Сверить с официальным `KitOps`

Цель:
- проверить, что предлагаемая связка совпадает с upstream contract.

Файлы/области:
- `kitops.org/docs/overview/`
- `kitops.org/docs/integrations/mlflow/`
- `kitops.org/docs/integrations/kserve/`
- `modelpack.org`

Проверки:
- поиск и чтение официальных источников.

Результат:
- подтверждённая граница: `MLflow` как lifecycle metadata backend, `KitOps` как packaging/distribution layer.

### Slice 4. Сформулировать рекомендацию для ai-models

Цель:
- дать практическое решение для v0/v1, а не просто обзор технологий.

Файлы/области:
- `plans/active/evaluate-kitops-with-dkp-registry/*`

Проверки:
- итоговая инженерная сводка.

Результат:
- ответ:
  - нужен ли `MLflow` в v0 serving path;
  - нужен ли `KitOps`;
  - подходит ли `payload-registry`;
  - нужен ли `bundle-registry`.

## Rollback point

Если по итогам проверки окажется, что `payload-registry` operationally не подходит под `ModelKit`, остановиться на выводе без архитектурного решения и без code changes.

## Final validation

- Проверить, что вывод не подменяет phase-1 готовой phase-2 API-моделью.
- Проверить, что recommendation опирается на реальные свойства `dkp-registry` и официальные `KitOps` docs.
