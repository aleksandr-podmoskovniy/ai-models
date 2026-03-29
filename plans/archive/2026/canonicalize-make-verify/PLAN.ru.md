# План работ: canonicalize make verify

## Current phase

Этап 1: внутренний managed backend inside the `ai-models` module.

## Slice 1. Зафиксировать scope policy-cleanup

### Цель

Найти все repo-facing упоминания `task verify` и определить, что именно надо
считать policy, а что лишь вспомогательной automation.

### Изменяемые области

- `plans/canonicalize-make-verify/`

### Проверки

- поиск по repo на `task verify`

### Артефакт

Понятно, какие файлы надо менять, чтобы policy стала однозначной.

## Slice 2. Перевести repo rules на make verify

### Цель

Убрать из обязательных правил репозитория двусмысленность между `make verify`
и `task verify`.

### Изменяемые области

- `AGENTS.md`

### Проверки

- поиск по repo на `task verify`

### Артефакт

Repo rules явно указывают `make verify` как canonical repo-level loop.

## Rollback point

После Slice 1. Scope уже понятен, но рабочее дерево ещё не менялось.

## Final validation

- `make verify`
