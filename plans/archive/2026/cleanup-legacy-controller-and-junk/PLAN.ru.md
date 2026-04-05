# Plan

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication plane и platform UX.

## Orchestration

- mode: `full`
- read-only subagents before code changes:
  - legacy controller reachability audit
  - non-code junk / stale planning hygiene audit
- final substantial review:
  - `review-gate`
  - `reviewer`

## Slice 1. Reachability cleanup

Цель:

- подтвердить по импорту и runtime wiring, какие legacy пакеты реально мертвы;
- удалить их без затрагивания current live path.

Файлы/каталоги:

- `images/controller/internal/*`
- `images/controller/README.md`
- `plans/active/rebaseline-publication-plane-to-backend-artifact-plane/*`

Проверки:

- `go test ./...` в `images/controller`

Артефакт:

- active controller tree без мёртвых legacy пакетов и с честным README.

## Slice 2. Junk cleanup

Цель:

- убрать явный generated junk и другие низкорисковые мусорные артефакты.
- убрать низкорисковые closed incident/fix bundles из `plans/active/`.

Файлы/каталоги:

- `.VSCodeCounter/*`
- точечные одноразовые bundles в `plans/active/*`
- при необходимости другие audit-confirmed junk files

Проверки:

- `git diff --check`

Артефакт:

- в репозитории не остаётся очевидного generated junk, не являющегося частью
  workflow.
- в `plans/active/` не остаются low-signal incident/fix bundles, которые уже
  завершены и не нужны как архитектурный контекст.

## Rollback point

После Slice 1 current controller tree уже станет чище, но runtime и tests будут
по-прежнему опираться только на действующий publication path.

## Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
