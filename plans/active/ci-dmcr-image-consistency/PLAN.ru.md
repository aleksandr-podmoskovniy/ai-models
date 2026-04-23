# План: согласованность DMCR image digest в CI

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

### Slice 2. Проверка опубликованного module image

Цель: добавить воспроизводимую проверку, что `images_digests.json` указывает
на DMCR image с ожидаемыми бинарями.

Файлы:

- `tools/ci/verify-module-image-payload.sh`

Проверки:

- `bash -n tools/ci/verify-module-image-payload.sh`;
- негативная проверка против текущего сломанного
  `ghcr.io/aleksandr-podmoskovniy/modules/ai-models:main`.

Артефакт:

- standalone script, который может запускаться и локально, и в GitHub Actions.

### Slice 3. GitHub Actions gate

Цель: включить проверку в publish path и release path.

Файлы:

- `.github/workflows/build.yaml`
- `.github/workflows/deploy.yaml`

Проверки:

- `make lint`;
- `make verify`.

Артефакт:

- build workflow проверяет опубликованный module image после `modules-actions/build`;
- deploy workflow проверяет module image перед `modules-actions/deploy`.

## Rollback point

Безопасный откат: вернуть изменения в `images/dmcr/werf.inc.yaml`, удалить
`tools/ci/verify-module-image-payload.sh` и убрать новые workflow steps.

Такой откат не меняет runtime-код и не требует миграций в кластере.

## Final validation

- `bash -n tools/ci/verify-module-image-payload.sh`
- негативная проверка против текущего сломанного `main` tag
- `make verify`
