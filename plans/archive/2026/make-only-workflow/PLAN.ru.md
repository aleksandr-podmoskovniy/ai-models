# План работ: make-only workflow

## Current phase

Этап 1: внутренний managed backend inside the `ai-models` module.

## Slice 1. Найти live references на task workflow

### Цель

Определить, где `task` ещё фигурирует именно как живой developer workflow, а не
как часть исторических планов или терминов вроде task bundle.

### Изменяемые области

- `plans/make-only-workflow/`

### Проверки

- поиск по repo на `task`, `Taskfile`, `task init`, `task verify`

### Артефакт

Понятен точный cleanup scope.

## Slice 2. Перевести repo surface на make-only

### Цель

Убрать второй entrypoint и сделать `make` единственным supported способом
выполнять проверки и локальные операции.

### Изменяемые области

- `AGENTS.md`
- `DEVELOPMENT.md`
- `Taskfile.yaml`
- `Taskfile.init.yaml`

### Проверки

- поиск по живой repo surface на `task`/`Taskfile`

### Артефакт

Живая документация и файловая структура репозитория больше не предлагают `task`
как альтернативный workflow.

## Rollback point

После Slice 1. Scope уже ясен, но рабочее дерево ещё не менялось.

## Final validation

- `make verify`
