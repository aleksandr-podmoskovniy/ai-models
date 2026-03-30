# План работ: fix werf hooks gitArchive stage

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Сравнить hooks image pattern с reference-модулями

### Цель

Подтвердить минимальный structural diff между текущим hooks image и working
pattern из `gpu-control-plane`.

### Изменяемые области

- `plans/active/fix-werf-hooks-gitarchive-stage/`

### Проверки

- чтение `images/hooks/werf.inc.yaml`
- сравнение с reference repos

### Артефакт

Понятен минимальный rewrite hooks image без изменения runtime semantics.

## Slice 2. Перевести hooks image на src-artifact/import pattern

### Цель

Сделать hooks build устойчивым для local werf build.

### Изменяемые области

- `images/hooks/werf.inc.yaml`
- при необходимости `DEVELOPMENT.md`

### Проверки

- local `werf build --dev --platform=linux/amd64` до следующего этапа
- `make verify`

### Артефакт

`go-hooks-artifact` больше не падает на зависимости собственного `gitArchive`.

## Rollback point

После Slice 1. Проблема локализована, но hooks image ещё не переписан.

## Final validation

- `make verify`
