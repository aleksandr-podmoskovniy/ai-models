# План работ: упростить config-values и перейти на global defaults

## Current phase

Этап 1: внутренний managed `MLflow` inside the DKP module.

## Slice 1. Сделать короткий stable user-facing schema

### Цель

Убрать из `config-values` преждевременные локальные override и оставить короткий stable user-facing contract, в который входят только действительно нужные настройки `PostgreSQL` и `S3-compatible artifacts`.

### Изменяемые области

- `openapi/config-values.yaml`
- `openapi/values.yaml`

### Проверки

- schema остаётся валидной;
- в schema нет локальных полей для `https`, `ingress`, `ha`, cert selection и `auth`;
- в schema есть понятные и ограниченные user-facing поля для `PostgreSQL` и `S3-compatible artifacts`;
- в `values` появляются явные defaults для runtime-only sections.

### Артефакт

`ai-models` выставляет короткий stable user-facing contract с `PostgreSQL` и `S3-compatible artifacts`, а runtime defaults живут в `values`; общая platform wiring берётся из global/platform defaults.

## Slice 2. Выровнять README и docs

### Цель

Сделать так, чтобы README и docs описывали минимальный contract модуля и не обещали прежние override-настройки.

### Изменяемые области

- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

### Проверки

- `make lint`
- `make verify`

### Артефакт

README и docs объясняют, что текущий runtime `ai-models` использует platform/global defaults для общей обвязки, а module-specific user-facing настройки остаются минимальными.

## Rollback point

После Slice 1. На этом шаге schema уже минимализирована, но docs и README ещё не выровнены.

## Final validation

- `make lint`
- `make verify`
