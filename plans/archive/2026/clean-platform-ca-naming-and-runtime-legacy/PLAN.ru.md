# PLAN

## Current phase
Этап 1. Внутренний managed backend внутри модуля.

## Режим orchestration
`solo`

## Slice 1. Выровнять naming общего trust CA
- цель: убрать OIDC-only naming там, где path уже обслуживает и S3;
- области:
  - `templates/_helpers.tpl`
  - `templates/module/*`
  - `templates/backend/*`
  - `tools/helm-tests/validate-renders.py`
  - `docs/CONFIGURATION*.md`
- проверки:
  - render checks
  - `make verify`
- артефакт:
  - единый platform trust naming без collision с OIDC-only semantics.

## Slice 2. Убрать runtime мусор и проверить legacy shims
- цель: удалить явный dead/runtime мусор и не сломать нужную compatibility;
- области:
  - `images/backend/scripts/*`
  - `.gitignore` при необходимости
  - `templates/_helpers.tpl`
  - `plans/active/clean-platform-ca-naming-and-runtime-legacy/REVIEW.ru.md`
- проверки:
  - узкие shell/py checks
  - `make verify`
- артефакт:
  - repo чище, compatibility shim либо подтверждён как живой, либо удалён.

## Rollback point
До изменений в `templates/*`, `tools/*`, `images/backend/scripts/*`, `docs/*`.

## Final validation
- `python3 -m py_compile images/backend/scripts/ai_models_backend_runtime.py images/backend/scripts/ai-models-backend-hf-import.py`
- `bash -n tools/run_hf_import_job.sh`
- `make verify`
