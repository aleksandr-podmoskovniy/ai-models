# План работ: fix werf goproxy default

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Локализовать источник пустого GOPROXY

### Цель

Понять, где и как `werf` должен задавать `.GOPROXY` для image secrets.

### Изменяемые области

- `plans/active/fix-werf-goproxy-default/`

### Проверки

- чтение `werf.yaml`
- чтение `images/**/werf.inc.yaml`
- сравнение с reference-модулями

### Артефакт

Понятен минимальный fix path для корректного default.

## Slice 2. Задать default и перепроверить build path

### Цель

Сделать local `werf` build устойчивым без обязательного внешнего `GOPROXY`.

### Изменяемые области

- `werf.yaml`
- при необходимости `DEVELOPMENT.md`

### Проверки

- локальный `werf build --dev` до следующего этапа
- `make verify`

### Артефакт

`GOPROXY` больше не приходит как `<no value>`.

## Rollback point

После Slice 1. Проблема локализована, но build contract ещё не изменён.

## Final validation

- `make verify`
