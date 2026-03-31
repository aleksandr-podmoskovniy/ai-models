# PLAN

## Current phase

Этап 1. Внутренний managed backend inside the module.

## Режим orchestration

`solo`

Причина:
- задача substantial и multi-area, но в текущем tool policy нельзя поднимать subagents без явного запроса пользователя;
- поэтому boundaries и решения фиксируются в bundle локально.

## Slice 1. Убрать dual auth contract

- перевести module contract на один канонический SSO path;
- удалить `Native` browser auth branch из values/helpers/templates;
- оставить внутренний machine account как internal runtime mechanism, а не user-facing mode.

Проверки:
- `make helm-template`

## Slice 2. Консолидировать runtime helpers

- убрать `render-auth-config` legacy;
- убрать отдельный `render-db-uri` script;
- ввести один backend runtime helper layer для мелкого glue;
- оставить отдельными только реальные операции: `db-upgrade`, `hf-import`, `bootstrap-oidc-auth`.

Проверки:
- `python3 -m py_compile images/backend/scripts/*.py`
- `make helm-template`

## Slice 3. Подтвердить import/storage baseline

- проверить, что backend и import job продолжают использовать direct-to-S3 path;
- выровнять fixtures / validate-renders под новый канонический contract.

Проверки:
- `make kubeconform`
- `make verify`

## Rollback point

- rollback point: текущее состояние до удаления dual auth branch и render-* scripts.

## Final validation

- `make verify`
