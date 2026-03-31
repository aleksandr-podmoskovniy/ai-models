# PLAN

## Current phase
Этап 1. Внутренний managed backend внутри модуля.

## Режим orchestration
`solo`

## Slice 1. Определить минимальный contract для S3 CA trust
- цель: выбрать семейный и нераздутый values contract для outbound S3 TLS trust;
- области:
  - `openapi/config-values.yaml`
  - `openapi/values.yaml`
  - `templates/_helpers.tpl`
  - `docs/CONFIGURATION*.md`
- проверки:
  - render/validation checks
- артефакт:
  - короткий user-facing contract для CA bundle без размножения storage paths,
    с default reuse platform CA от Dex/global HTTPS path.

## Slice 2. Пробросить S3 CA в backend runtime и import Jobs
- цель: сделать один source of truth для backend и one-shot HF import Jobs;
- области:
  - `templates/module/*`
  - `templates/backend/*`
  - `images/backend/scripts/ai_models_backend_runtime.py`
  - `tools/run_hf_import_job.sh`
- проверки:
  - узкие py/shell checks
  - `make verify`
- артефакт:
  - mounted CA bundle и exported env vars для boto3/urllib3/MLflow S3 path.

## Slice 3. Закрепить docs и guards
- цель: не дать insecure runtime вернуться незаметно;
- области:
  - `tools/helm-tests/validate-renders.py`
  - `docs/CONFIGURATION*.md`
- проверки:
  - `make verify`
- артефакт:
  - docs и verify loop согласованы с новым trust contract.

## Rollback point
До изменений в `openapi/*`, `templates/*`, `images/backend/scripts/*`, `tools/*`.

## Final validation
- `python3 -m py_compile images/backend/scripts/ai_models_backend_runtime.py`
- `bash -n tools/run_hf_import_job.sh`
- `make verify`
