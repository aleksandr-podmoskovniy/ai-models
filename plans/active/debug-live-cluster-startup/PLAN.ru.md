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
    - после исправления DB init path новый ближайший blocker находится уже в
      backend container: он уходит в `OOMKilled` после старта;
    - текущий runtime profile поднимает лишние для phase-1 процессы:
      `uvicorn` по умолчанию с `4` workers и `Huey` job runner/consumers;
    - для phase-1 managed backend это лишний footprint: нужны tracking/UI/registry,
      но не genai job execution runtime.
    - после сужения runtime profile backend уже стартует, но внешние запросы через
      ingress получают `Invalid Host header`, потому что upstream security
      middleware остаётся в default `localhost-only` mode без явных
      `allowed-hosts` и `cors-allowed-origins` для публичного домена модуля.
    - после перевода browser-login на `mlflow-oidc-auth` новый rollout падает в
      init container `auth-bootstrap`: auth plugin использует собственные
      alembic migrations и по умолчанию тот же `alembic_version`, что и MLflow
      tracking/workspace store; при общем PostgreSQL это даёт collision на
      `Can't locate revision identified by '<mlflow-revision>'`.
    - с точки зрения upstream-looking contract более чистый путь — отдельная
      logical auth database в том же PostgreSQL instance: `OIDC_USERS_DB_URI`
      должен указывать не в tracking/workspace DB, а в dedicated auth DB;
      при `managed-postgres` модуль может создавать её сам, а при `External`
      existing PostgreSQL должен её предоставлять.

### Slice 2. Починить ближайший blocker в модуле
- Цель: устранить следующую реальную причину падения, если она находится в
  repo/module contract.
- Области:
  - `templates/_helpers.tpl`
  - `templates/backend/configmap.yaml`
  - `templates/backend/deployment.yaml`
  - `docs/CONFIGURATION*.md`
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - узкие локальные проверки
  - `make verify`
- Артефакт:
  - init/upgrade flow, который корректно обрабатывает и пустую БД, и
    существующую схему;
  - backend runtime profile, который укладывается в phase-1 scope и не
    завышает footprint без необходимости;
  - backend security profile, который допускает public ingress host и
    same-origin browser access без отключения upstream security middleware.
  - OIDC auth store, который использует отдельную auth DB в том же PostgreSQL
    instance и не делит database namespace с MLflow tracking/workspace store.

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
