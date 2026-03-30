# PLAN: исправить managed-postgres RFC1123 names и topology

## Current phase

Этап 1. Внутренний managed backend. Задача ограничена managed PostgreSQL wiring
и verify loop.

## Slices

### Slice 1. Привести defaults к RFC1123-safe именам
- Цель: убрать `_` из default database/user names.
- Области:
  - `openapi/config-values.yaml`
  - `openapi/values.yaml`
  - `templates/_helpers.tpl`
- Проверки:
  - `make helm-template`

### Slice 2. Сделать topology адаптивным к PostgresClass
- Цель: не хардкодить `Zonal`, а брать `defaultTopology` выбранного
  `PostgresClass` или использовать безопасный fallback.
- Области:
  - `templates/_helpers.tpl`
  - `templates/database/managed-postgres.yaml`
  - `templates/database/postgresclass.yaml`
- Проверки:
  - `make helm-template`

### Slice 3. Усилить local render assertions
- Цель: ловить invalid DB/user names и плохой topology contract локально.
- Области:
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `make verify`

## Rollback point

После Slice 1. Если topology fix окажется спорным, можно оставить только
RFC1123 rename и отдельно добивать class-aware topology.

## Orchestration mode

solo
