# Исправить managed-postgres RFC1123 names и topology contract

## Контекст

На живом кластере `ai-models` падает на admission webhook `managed-postgres` с
тремя ошибками:

- `spec.databases[0].name: "ai_models"` не проходит RFC1123;
- `spec.users[0].name: "ai_models"` не проходит RFC1123;
- `spec.cluster.topology: Zonal` несовместим с текущим `PostgresClass/default`,
  у которого `allowedZones: []` и `defaultTopology: Ignored`.

## Постановка задачи

Нужно привести managed PostgreSQL wiring к реальному contract кластера:

- user-facing и internal defaults для database/user должны быть RFC1123-safe;
- topology для managed `Postgres` нельзя жёстко хардкодить как `Zonal`, нужно
  адаптироваться к выбранному `PostgresClass`;
- local verify loop должен ловить эти регрессии до deploy.

## Scope

- `templates/_helpers.tpl`
- `templates/database/managed-postgres.yaml`
- `templates/database/postgresclass.yaml`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `tools/helm-tests/*`
- docs по verify contract при необходимости

## Non-goals

- не добавлять новый user-facing knob для topology;
- не менять contract external PostgreSQL;
- не менять runtime backend beyond DB connection names.

## Критерии приёмки

- managed `Postgres` рендерится с database/user без `_`;
- при HA topology берётся из `PostgresClass.defaultTopology`, а без lookup
  используется безопасный fallback;
- `make verify` проходит;
- local render validation ловит отсутствие RFC1123-safe database/user names.
