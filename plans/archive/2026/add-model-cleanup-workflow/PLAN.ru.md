# Plan

## 1. Current phase

Этап 1: managed backend inside the module.

Задача относится к внутреннему backend engine, storage orchestration и
эксплуатации phase-1. Публичный DKP API каталога не меняется.

Режим orchestration: `solo`.

## 2. Slices

### Slice 1. Cleanup runtime entrypoint

- Цель: добавить image-owned Python entrypoint, который резолвит `Model Version`
  -> `Logged Model` / `Run` / S3 prefixes и выполняет controlled cleanup.
- Файлы:
  - `images/backend/scripts/ai-models-backend-model-cleanup.py`
  - `images/backend/scripts/ai_models_backend_runtime.py`
  - `images/backend/werf.inc.yaml`
- Проверки:
  - `python3 -m py_compile images/backend/scripts/ai_models_backend_runtime.py images/backend/scripts/ai-models-backend-model-cleanup.py`
- Результат:
  - новый runtime entrypoint доступен в backend image и reuse’ит текущий S3
    env/CA bridge.

### Slice 2. In-cluster operator helper

- Цель: добавить shell helper, который запускает cleanup как one-shot Job
  текущим backend image и reuse’ит machine auth/S3 env из live deployment.
- Файлы:
  - `tools/run_model_cleanup_job.sh`
  - при необходимости `tools/run_hf_import_job.sh`
- Проверки:
  - `bash -n tools/run_model_cleanup_job.sh`
  - `bash -n tools/run_hf_import_job.sh`
- Результат:
  - оператор может запускать cleanup через новый Job helper без ручного доступа в
    pod и без прямых `aws s3 rm`.

### Slice 3. Docs and repo validations

- Цель: зафиксировать cleanup workflow в docs и добавить минимальные guards.
- Файлы:
  - `README.md`
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `make verify`
- Результат:
  - cleanup workflow описан и проходит repo-level validation.

## 3. Rollback point

До merge можно безопасно откатиться к состоянию без cleanup workflow, удалив:

- `images/backend/scripts/ai-models-backend-model-cleanup.py`
- `tools/run_model_cleanup_job.sh`
- wiring нового entrypoint из `images/backend/werf.inc.yaml`

Это не ломает существующий import path и runtime contracts.

## 4. Final validation

- `python3 -m py_compile images/backend/scripts/ai_models_backend_runtime.py images/backend/scripts/ai-models-backend-model-cleanup.py`
- `bash -n tools/run_model_cleanup_job.sh`
- `bash -n tools/run_hf_import_job.sh`
- `make verify`
