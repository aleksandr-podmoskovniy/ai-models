# PLAN: исправить install-path лимит на файл Go hooks

## Current phase

Этап 1. Внутренний managed backend. Задача затрагивает только external module packaging и hooks delivery.

## Slices

### Slice 1. Локализовать источник лимита chart file
- Цель: найти точный код/contract, где `ai-models-module-hooks` режется лимитом 5 MiB.
- Области:
  - локальные Deckhouse/operator sources
  - reference repos `gpu-control-plane`, `virtualization`
  - текущий `ai-models` hooks packaging
- Проверки:
  - локальный поиск по коду
- Артефакт:
  - зафиксированное понимание install path и отличия от соседних модулей.

### Slice 2. Исправить hooks packaging/wiring
- Цель: привести `ai-models` к рабочему path без oversized chart file.
- Области:
  - `images/hooks`
  - `werf.yaml`
  - `.werf/stages/*`
  - при необходимости docs
- Проверки:
  - `werf config render --dev --env dev`
  - узкие проверки hooks build path
- Артефакт:
  - hooks packaging совместим с external module install path.

### Slice 3. Подтвердить verify loop и зафиксировать review
- Цель: проверить, что repo не сломан и diff не patchwork.
- Проверки:
  - `make lint`
  - `make test`
  - `make helm-template`
  - `make verify`
- Артефакт:
  - короткий review и согласованный diff.

## Rollback point

После Slice 1, до изменения hooks packaging. На этом шаге можно безопасно вернуться к текущему state и не трогать runtime templates.

## Orchestration mode

solo

## Final validation

- `werf config render --dev --env dev`
- `make verify`
