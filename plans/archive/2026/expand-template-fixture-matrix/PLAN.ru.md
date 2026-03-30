# План работ: expand template fixture matrix

## Current phase

Этап 1: внутренний managed backend внутри модуля `ai-models`.

## Slice 1. Спроектировать fixture coverage matrix

### Цель

Определить минимальный, но полезный набор render scenarios, который покрывает
главные ветки phase-1 templates.

### Изменяемые области

- `plans/active/expand-template-fixture-matrix/`

### Проверки

- read-only inspection `tools/helm-tests/helm-template.sh`
- read-only inspection current fixtures and helpers

### Артефакт

Понятен список scenario files и expected render names.

## Slice 2. Добавить fixtures и обновить render loop

### Цель

Сделать `make helm-template` многосценарным без усложнения `kubeconform`
pipeline.

### Изменяемые области

- `fixtures/module-values.yaml`
- `fixtures/render/*.yaml`
- `tools/helm-tests/helm-template.sh`

### Проверки

- `make helm-template`
- `make kubeconform`

### Артефакт

Несколько `helm-template-*.yaml` рендерятся автоматически.

## Slice 3. Обновить workflow docs и прогнать repo-level checks

### Цель

Зафиксировать новый verify behavior и убедиться, что repo-level checks зелёные.

### Изменяемые области

- при необходимости `DEVELOPMENT.md`

### Проверки

- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify`

### Артефакт

Fixture matrix закреплена в tooling и понятна разработчикам.

## Rollback point

После Slice 1. Матрица согласована в bundle, но render loop ещё не менялся.

## Final validation

- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify`
