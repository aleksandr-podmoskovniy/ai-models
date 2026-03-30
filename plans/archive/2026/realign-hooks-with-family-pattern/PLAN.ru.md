# PLAN: вернуть hooks ai-models к family-pattern DKP модулей

## Current phase

Этап 1. Внутренний managed backend. Задача касается только module shell/build layout и hooks delivery для platform HTTPS/certificate integration.

## Slices

### Slice 1. Зафиксировать канонический hooks pattern для ai-models

- Цель: перевести hooks runtime code под `images/hooks` и убрать временный shell-hook путь.
- Файлы/каталоги:
  - `images/hooks`
  - `hooks`
  - `werf.yaml`
  - `.werf/stages/bundle.yaml`
- Проверки:
  - `werf config render --dev --env dev`
- Артефакт результата:
  - `go-hooks-artifact` импортируется в `/hooks/go`, shell-hook workaround удалён.

### Slice 2. Выровнять docs и repo guidance под новый layout

- Цель: синхронизировать docs с тем, как hooks реально доставляются в модуль.
- Файлы/каталоги:
  - `DEVELOPMENT.md`
  - `docs/development/REPO_LAYOUT.ru.md`
- Проверки:
  - `make lint`
- Артефакт результата:
  - docs не описывают shell-hook workaround и не противоречат module layout.

### Slice 3. Подтвердить repo-level verify loop

- Цель: убедиться, что новый hooks flow не ломает render/verify pipeline.
- Файлы/каталоги:
  - при необходимости `plans/active/realign-hooks-with-family-pattern/REVIEW.ru.md`
- Проверки:
  - `make test`
  - `make helm-template`
  - `make verify`
- Артефакт результата:
  - validate loop проходит, есть короткий review по остаточным рискам.

## Rollback point

До удаления shell-hook workaround и перед заменой bundle wiring. На этом шаге можно вернуться к текущему dirty state без изменения runtime templates phase-1.

## Orchestration mode

`solo`

Задача опирается на прямое выравнивание с уже существующими family-pattern references (`gpu-control-plane`, `virtualization`) и не требует дополнительной delegation в этой среде.

## Final validation

- `make lint`
- `make test`
- `make helm-template`
- `make verify`
- `werf config render --dev --env dev`
