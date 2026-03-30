# PLAN: разобрать живой startup ai-models

## Current phase

Этап 1. Внутренний managed backend. Задача ограничена startup/debug path
external DKP module на живом кластере.

## Slices

### Slice 1. Снять live state startup
- Цель: собрать реальные ошибки из кластера.
- Области:
  - `kubectl get/describe/logs/events`
- Проверки:
  - фактические логи и статусы ресурсов
- Артефакт:
  - список конкретных blocker'ов startup sequence.
  - Текущий вывод:
    - wiring модуля, Ingress/TLS, Dex и managed PostgreSQL уже проходят дальше
      прежних admission blocker'ов;
    - ближайший runtime blocker находится в init container `db-upgrade`;
    - для пустой БД запускается `ai-models-backend db upgrade`, но upstream
      `MLflow` ожидает на first-start path через
      `mlflow.store.db.utils._safe_initialize_tables()`;
    - из-за этого `alembic` пытается выполнить `ALTER TABLE metrics ...` до
      создания initial tables и падает на `relation "metrics" does not exist`.

### Slice 2. Починить ближайший blocker в модуле
- Цель: устранить следующую реальную причину падения, если она находится в
  repo/module contract.
- Области:
  - `templates/backend/configmap.yaml`
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - узкие локальные проверки
  - `make verify`
- Артефакт:
  - init/upgrade flow, который корректно обрабатывает и пустую БД, и
    существующую схему.

### Slice 3. Подтвердить новое состояние
- Цель: сверить repo state и сформулировать следующий cluster retry step.
- Проверки:
  - `make verify`
- Артефакт:
  - понятный handoff для повторного deploy/retry.

## Rollback point

После Slice 1, до внесения правок в repo. На этом шаге можно остановиться с
чистым diagnostic output без изменения модуля.

## Orchestration mode

solo

## Final validation

- `make verify` если в repo вносились изменения
