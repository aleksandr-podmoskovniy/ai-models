# План работ: fix werf giterminism env allowlist

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Инвентаризация env usage в werf config

### Цель

Найти все текущие `env(...)` в `werf` templates и сравнить их с allowlist.

### Изменяемые области

- `plans/active/fix-werf-giterminism-env-allowlist/`

### Проверки

- grep по `env(...)`
- чтение `werf-giterminism.yaml`

### Артефакт

Понятен минимальный diff по allowlist или template cleanup.

## Slice 2. Исправление allowlist и проверка

### Цель

Снять текущий giterminism blocker без изменения runtime semantics.

### Изменяемые области

- `werf-giterminism.yaml`
- при необходимости `images/**/werf.inc.yaml` или `.werf/stages/*.yaml`

### Проверки

- sanity check template/env usage
- `make verify`

### Артефакт

`werf` config больше не падает на первом запрещённом env.

## Rollback point

После Slice 1. Проблема локализована, но config ещё не менялся.

## Final validation

- `make verify`
