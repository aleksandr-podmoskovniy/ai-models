# PLAN: убрать default KEK passphrase из runtime ai-models

## Current phase

Этап 1. Managed backend inside the module. Это runtime/security hardening slice
без расширения user-facing contract модуля.

Режим orchestration: `solo`.

## Slices

### Slice 1. Добавить stable generated crypto secret

Цель:
- сделать внутренний Secret с KEK passphrase, который стабилен между upgrade.

Файлы/каталоги:
- `templates/_helpers.tpl`
- `templates/module/*`

Проверки:
- `make helm-template`

Артефакт результата:
- module-owned Secret и helper для reuse existing value через `lookup`.

### Slice 2. Подключить passphrase в backend runtime

Цель:
- убрать использование upstream default passphrase в Deployment.

Файлы/каталоги:
- `templates/backend/deployment.yaml`

Проверки:
- `make helm-template`

Артефакт результата:
- backend env включает `MLFLOW_CRYPTO_KEK_PASSPHRASE` из internal Secret.

### Slice 3. Обновить docs и verify

Цель:
- зафиксировать новый security baseline и проверить repo.

Файлы/каталоги:
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `plans/active/harden-mlflow-crypto-passphrase/*`

Проверки:
- `make verify`

Артефакт результата:
- docs согласованы с runtime, repo checks зелёные.

## Rollback point

После Slice 1. Если wiring runtime окажется спорным, можно оставить только
analysis bundle без изменения Deployment contract.

## Final validation

- `make helm-template`
- `make verify`
