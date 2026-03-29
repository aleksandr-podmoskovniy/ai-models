# План работ: user-facing позиционирование модуля ai-models

## Current phase

Этап 1: внутренний managed `MLflow` inside the DKP module.

## Slice 1. Переписать module metadata и repository entrypoints

### Цель

Убрать из module metadata и top-level README временную лексику и задать нормальное product-level описание модуля.

### Изменяемые области

- `module.yaml`
- `README.md`
- `README.ru.md`
- `oss.yaml`

### Проверки

- wording не обещает уже реализованный phase-2 API;
- описание модуля одинаково читается в metadata и README.

### Артефакт

`ai-models` позиционируется как DKP-модуль для AI/ML model registry и catalog services, а managed `MLflow` описан как текущая runtime-реализация.

## Slice 2. Переписать docs overview/configuration и OpenAPI intro

### Цель

Выровнять docs и user-facing schema description под тот же narrative, без `shell` и `foundation`.

### Изменяемые области

- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `openapi/config-values.yaml`

### Проверки

- `make lint`
- `make verify`

### Артефакт

User-facing docs описывают текущий контракт модуля как конфигурацию runtime `ai-models` на базе managed `MLflow`, а не как bootstrap для будущего продукта.

## Rollback point

После Slice 1. На этом шаге module metadata и top-level README уже согласованы, но docs и OpenAPI intro ещё не переписаны.

## Final validation

- `make lint`
- `make verify`
