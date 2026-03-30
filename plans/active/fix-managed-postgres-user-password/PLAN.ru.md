# PLAN: исправить managed-postgres user password contract

## Current phase

Этап 1. Внутренний managed backend. Задача ограничена wiring для managed
PostgreSQL и локальным verify loop.

## Slices

### Slice 1. Добавить стабильный managed PostgreSQL password helper
- Цель: рендерить `Postgres.spec.users[].password` и не ломать upgrade path.
- Области:
  - `templates/_helpers.tpl`
  - `templates/database/managed-postgres.yaml`
- Проверки:
  - `make helm-template`
- Артефакт:
  - managed `Postgres` получает непустой пароль и продолжает писать creds в
    `ai-models-postgresql`.

### Slice 2. Усилить render verification
- Цель: локально ловить отсутствие `password` у managed `Postgres`.
- Области:
  - `tools/helm-tests/helm-template.sh`
  - при необходимости новый helper script в `tools/helm-tests/`
- Проверки:
  - `make helm-template`
- Артефакт:
  - local render matrix валидирует critical contract для custom `Postgres`.

### Slice 3. Подтвердить repo state и review
- Цель: убедиться, что fix не ломает остальной module shell.
- Проверки:
  - `make verify`
- Артефакт:
  - короткий review и согласованный diff.

## Rollback point

После Slice 1, до изменения verify tooling. На этом шаге можно откатить render
assertions и оставить только template fix.

## Orchestration mode

solo

## Final validation

- `make helm-template`
- `make verify`
