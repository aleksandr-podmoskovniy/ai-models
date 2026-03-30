# Исправить managed-postgres user password contract

## Контекст

На живом Deckhouse startup `ai-models` падает на создании `Postgres` с ошибкой:
`spec.users[0]: Invalid value: "object": either password or hashedPassword must be specified`.

Текущий template передаёт только `storeCredsToSecret`, чего теперь недостаточно
для CRD `managed-services.deckhouse.io/v1alpha1/Postgres`.

## Постановка задачи

Нужно привести managed PostgreSQL wiring к рабочему контракту без нового
user-facing секрета: модуль должен стабильно формировать пароль пользователя для
`Postgres.spec.users[].password`, переиспользовать уже созданный secret и
обновить local verify loop, чтобы эта регрессия больше не проходила.

## Scope

- исправить template `Postgres` для managed mode;
- добавить стабильный helper для пароля managed PostgreSQL;
- усилить local render checks для `Postgres` custom resource;
- прогнать релевантные проверки.

## Non-goals

- не менять user-facing contract `postgresql.mode=Managed`;
- не вводить новый обязательный secret в `ModuleConfig`;
- не менять wiring external PostgreSQL.

## Затрагиваемые области

- `templates/database/managed-postgres.yaml`
- `templates/_helpers.tpl`
- `tools/helm-tests/*`
- `plans/active/fix-managed-postgres-user-password/*`

## Критерии приёмки

- managed `Postgres` рендерится с непустым `spec.users[].password`;
- пароль переиспользуется из уже существующего `ai-models-postgresql` secret,
  если он есть;
- `make verify` проходит;
- local render validation явно ловит отсутствие `password` у managed `Postgres`.

## Риски

- `lookup`-based logic может повести себя иначе в offline `helm template`, если
  не учесть отсутствие cluster secret;
- если сделать пароль нестабильным, upgrade сломает доступ backend к базе.
