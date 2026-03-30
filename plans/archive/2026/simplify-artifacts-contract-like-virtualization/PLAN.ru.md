# План работ: simplify artifacts contract like virtualization

## Current phase

Этап 1: внутренний managed backend внутри модуля `ai-models`.

## Slice 1. Зафиксировать целевой contract

### Цель

Подтвердить, что для phase-1 ai-models достаточно более жёсткого S3 contract по
образцу `virtualization`: inline credentials и internal Secret без external
secret indirection.

### Изменяемые области

- `plans/active/simplify-artifacts-contract-like-virtualization/`

### Проверки

- read-only inspection current ai-models contract
- read-only inspection `virtualization`

### Артефакт

Понятна целевая схема и список user-facing полей, которые нужно убрать.

## Slice 2. Упростить schema и template wiring

### Цель

Убрать лишние user-facing knobs и сделать fixed internal secret wiring.

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

Модуль всегда рендерит internal Secret и не требует key mapping.

## Slice 3. Почистить docs и fixtures

### Цель

Согласовать docs и render matrix с новым коротким contract.

### Изменяемые области

- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `fixtures/module-values.yaml`
- `fixtures/render/*.yaml`

### Проверки

- `make helm-template`
- `make kubeconform`
- `make verify`

### Артефакт

Repo surface больше не содержит следов старого `existingSecret/keyKey` contract.

## Rollback point

После Slice 1. Новый contract зафиксирован только в bundle.

## Final validation

- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify`
