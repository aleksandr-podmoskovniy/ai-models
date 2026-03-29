# План работ: cleanup production placeholders

## Current phase

Этап 1: managed MLflow inside the DKP module.

## Slice 1. Найти реальные sample/placeholder точки

### Цель

Отделить реальные demo-следы от нормальных runtime defaults.

### Изменяемые области

- `plans/cleanup-production-placeholders/`

### Проверки

- ручная сверка `fixtures/module-values.yaml`
- ручная сверка `openapi/values.yaml`
- поиск по repo на `example`, `fake`, `placeholder`

### Артефакт

Есть короткий список файлов, которые нужно чистить.

## Slice 2. Вычистить fixture и wording

### Цель

Заменить demo/sample значения на нейтральные production-safe значения и убрать явные placeholder формулировки.

### Изменяемые области

- `fixtures/module-values.yaml`
- `openapi/values.yaml`
- `Taskfile.yaml`

### Проверки

- `make lint`
- `make helm-template`

### Артефакт

Репозиторий не содержит явных sample-заглушек в затронутых местах и сохраняет рабочий render loop.

## Rollback point

После Slice 1. Список проблем уже понятен, но рабочее дерево ещё не менялось.

## Final validation

- `make verify`
