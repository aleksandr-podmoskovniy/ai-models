# PLAN

## Current phase
Этап 1. Внутренний managed backend внутри модуля.

## Режим orchestration
`solo`

## Slice 1. Сделать repo-wide layering audit
- цель: отделить реальные stale/dead paths от живых compatibility слоёв;
- области:
  - `plans/active/*`
  - `docs/*`
  - `templates/*`
  - `images/*`
  - `tools/*`
- проверки:
  - targeted repo search
- артефакт:
  - список cleanup-кандидатов с понятной причиной.

## Slice 2. Убрать подтверждённые stale bundles и dead paths
- цель: вычистить только то, что уже superseded текущим baseline;
- области:
  - `plans/active/*`
  - repo files with confirmed dead references
- проверки:
  - narrow file checks
  - `make verify`
- артефакт:
  - active tree и repo wiring без очевидных наслоений.

## Slice 3. Закрепить cleanup в docs/review
- цель: зафиксировать, что удалено как stale/legacy, и явно отметить removal последнего compatibility shim;
- области:
  - task bundle review
  - docs wording if needed
- проверки:
  - `make verify`
- артефакт:
  - cleanup не выглядит как случайная косметика.

## Rollback point
До изменений в `plans/active/*`, `docs/*`, `templates/*`, `images/*`, `tools/*`.

## Final validation
- `make verify`
