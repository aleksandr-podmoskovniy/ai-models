# PLAN: убрать PyYAML-зависимость из render validator

## Current phase

Этап 1. Задача ограничена local/CI verify tooling.

## Slice

### Slice 1. Переписать validator на stdlib-only parsing
- Цель: сохранить текущую semantic-проверку `Postgres.users[].password` без
  внешних python пакетов.
- Области:
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `make verify`

## Rollback point

До изменения `validate-renders.py`. В худшем случае можно временно вернуться к
предыдущему варианту и чинить CI через dependency install.

## Orchestration mode

solo
