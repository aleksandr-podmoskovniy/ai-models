# План: пересборка DMCR image из актуальных исходников

## Current phase

Этап 1: publication/runtime baseline.

Задача относится к сборочному и эксплуатационному quality gate вокруг DMCR,
без расширения API и без переноса задач этапов 2-3.

## Orchestration

Режим: `light`.

Причина: задача затрагивает build/publish boundary и GitHub Actions, но
реализация узкая и не меняет runtime API.

Read-only review:

- `integration_architect` запрошен для проверки гипотезы по build/publish
  boundary. Результат не вернулся за отведённое время до начала реализации;
  текущая реализация опирается на фактические проверки registry и сравнение с
  существующими werf-паттернами controller/hooks.

## Slices

### Slice 1. DMCR source/build split

Цель: убрать нестандартную сборочную форму DMCR и привести её к уже принятому
паттерну `*-src-artifact` -> `*-build-artifact`.

Файлы:

- `images/dmcr/werf.inc.yaml`

Проверки:

- визуальная сверка с `images/controller/werf.inc.yaml`;
- `make lint`;
- `make verify` на финальном шаге.

Артефакт:

- DMCR build artifact получает исходники только через source artifact import.

## Rollback point

Безопасный откат: вернуть изменения в `images/dmcr/werf.inc.yaml`.

Такой откат не меняет runtime-код и не требует миграций в кластере.

## Final validation

- `make verify`
