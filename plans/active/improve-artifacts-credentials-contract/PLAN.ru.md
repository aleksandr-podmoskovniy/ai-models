# План работ: improve artifacts credentials contract

> Superseded by `plans/active/simplify-artifacts-contract-like-virtualization/`.

## Current phase

Этап 1: внутренний managed backend внутри модуля `ai-models`.

## Slice 1. Зафиксировать новый bootstrap-friendly contract

### Цель

Определить user-facing semantics для artifacts credentials, совместимую с DKP
module patterns и не требующую ручного создания secret в `d8-ai-models` для
простого старта.

### Изменяемые области

- `plans/active/improve-artifacts-credentials-contract/`

### Проверки

- read-only inspection текущих `openapi/`, `templates/` и reference modules

### Артефакт

Понятна целевая схема: `existingSecret` или inline `accessKey` / `secretKey`.

## Slice 2. Реализовать dual-mode credentials flow

### Цель

Добавить inline credentials в user-facing schema и template wiring, не ломая
reuse внешнего секрета.

### Изменяемые области

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/module/artifacts-secret.yaml`
- `templates/backend/deployment.yaml`

### Проверки

- `make lint`
- `make helm-template`

### Артефакт

Модуль умеет работать либо с `existingSecret`, либо с inline credentials.

## Slice 3. Обновить docs и fixtures

### Цель

Сделать bootstrap flow понятным и согласованным между docs, examples и
templates.

### Изменяемые области

- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `fixtures/module-values.yaml`

### Проверки

- `make lint`
- `make helm-template`
- `make verify`

### Артефакт

Repo contract, docs и fixtures согласованы.

## Rollback point

После Slice 1. Новая схема зафиксирована только в bundle, runtime ещё не
изменён.

## Final validation

- `make lint`
- `make helm-template`
- `make verify`
