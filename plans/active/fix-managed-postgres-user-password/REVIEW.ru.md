# REVIEW

## Findings

Критичных замечаний по текущему diff нет.

## Что подтверждено

- найден точный источник cluster failure: managed `Postgres` теперь требует
  `spec.users[].password` или `hashedPassword`, одного `storeCredsToSecret`
  недостаточно;
- пароль для managed PostgreSQL теперь стабильно берётся из уже существующего
  `ai-models-postgresql` Secret через `lookup`, а при первом install
  генерируется один раз в render path;
- `Postgres` продолжает использовать `storeCredsToSecret`, поэтому backend
  остаётся на прежнем secret contract;
- local render matrix теперь явно валидирует этот custom-resource contract;
- `make verify` проходит.

## Residual risks

- первый install по-прежнему зависит от того, что managed-postgres оператор
  создаст `ai-models-postgresql` Secret до того, как backend начнёт реально
  использовать DB credentials;
- если upstream contract `Postgres` CRD снова изменится, repo-local semantic
  assertions придётся расширить.
