# План работ: pin backend to stable release

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Подтвердить текущий pinning и выбрать stable release

### Цель

Понять, какой ref/version сейчас используются, и подтвердить последний
стабильный upstream release по официальным источникам.

### Изменяемые области

- `plans/active/pin-backend-to-stable-release/`

### Проверки

- чтение `images/backend/upstream.lock`
- проверка official upstream release metadata

### Артефакт

Зафиксированы stable release version и ref для production pinning.

## Slice 2. Обновить upstream metadata и docs

### Цель

Перевести build metadata на stable release tag/version и убрать dev snapshot
semantics из repo-facing описаний.

### Изменяемые области

- `images/backend/upstream.lock`
- при необходимости `DEVELOPMENT.md`
- при необходимости `images/backend/patches/README.md`

### Проверки

- чтение итогового diff

### Артефакт

Pinned upstream metadata явно описывает stable release baseline.

## Slice 3. Подтвердить fetch loop на стабильном release

### Цель

Убедиться, что scripts корректно тянут stable upstream source и распознают
locked version.

### Изменяемые области

- без новых файлов, если fetch проходит

### Проверки

- `make backend-fetch-source`
- `make backend-shell-check`

### Артефакт

Stable release pin подтверждён рабочим local fetch loop.

## Rollback point

После Slice 1. Stable release выбран, но metadata ещё не изменена.

## Final validation

- `make backend-fetch-source`
- `make backend-shell-check`
