# План работ: cleanup unused directories

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Подтвердить список лишних каталогов

### Цель

Отделить structural мусор от локальных cache/tool directories.

### Изменяемые области

- `plans/active/cleanup-unused-directories/`

### Проверки

- инвентаризация empty dirs
- поиск repo references на candidate paths

### Артефакт

Понятно, какие каталоги удаляются, а какие остаются.

## Slice 2. Удалить и перенести лишние каталоги

### Цель

Почистить repo layout без изменения рабочего contract.

### Изменяемые области

- `.agents/skills/`
- `controllers/`
- `hooks/`
- `plans/active/`
- `plans/archive/2026/`

### Проверки

- `make verify`

### Артефакт

Repo больше не содержит пустых structural directories и stale active bundles.

## Rollback point

После Slice 1. Список candidate paths подтверждён, но физические удаления и
переносы ещё не выполнены.

## Final validation

- `make verify`
