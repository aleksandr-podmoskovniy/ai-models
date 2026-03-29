# План работ: remove MLflow from module surface

## Current phase

Этап 1: внутренний managed backend inside the `ai-models` module.

## Slice 1. Разделить наружную поверхность и internal implementation layer

### Цель

Определить, какие упоминания `MLflow` относятся к module-facing контракту, а
какие являются допустимыми внутренними деталями upstream build layer.

### Изменяемые области

- `plans/remove-mlflow-from-module-surface/`

### Проверки

- поиск по repo на `MLflow` / `mlflow`
- ручная классификация по группам: docs, values/OpenAPI, runtime naming, build internals

### Артефакт

Есть понятная граница cleanup scope.

## Slice 2. Убрать MLflow branding из module-facing docs и contract surface

### Цель

Сделать модуль и документацию самодостаточными как `ai-models`.

### Изменяемые области

- `module.yaml`
- `README*.md`
- `docs/`
- `AGENTS.md`
- `DEVELOPMENT.md`
- `openapi/`

### Проверки

- `make lint`

### Артефакт

Docs и values/OpenAPI больше не продвигают `MLflow` как лицо модуля.

## Slice 3. Перевести runtime and repo-facing naming на ai-models/backend

### Цель

Убрать `mlflow` из runtime/object naming и operational entrypoints там, где это
не является обязательной internal implementation detail.

### Изменяемые области

- `templates/`
- `fixtures/module-values.yaml`
- `Makefile`
- `Taskfile.yaml`
- `.agents/` / `.codex/`, если там ещё остались project-facing упоминания

### Проверки

- `make helm-template`
- `make verify`

### Артефакт

Runtime naming и developer entrypoints выровнены под `ai-models`.

## Rollback point

После Slice 1. Scope cleanup уже ясен, но рабочее дерево ещё не менялось.

## Final validation

- `make verify`
