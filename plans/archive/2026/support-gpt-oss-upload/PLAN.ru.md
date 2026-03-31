# PLAN

## Current phase
Этап 1. Внутренний managed backend inside the module.

## Режим orchestration
`solo`

## Slice 1. Перевести import semantics на local checkpoint
- цель: уйти от `pipeline(...)` и загрузки больших HF-моделей в RAM;
- файлы:
  - `images/backend/scripts/ai-models-backend-hf-import.py`
  - `tools/upload_hf_model.py`
- проверки:
  - `python3 -m py_compile images/backend/scripts/ai-models-backend-hf-import.py`
  - `python3 -m py_compile tools/upload_hf_model.py`
- артефакт: единый import flow через `snapshot_download -> log_model(local checkpoint) -> register_model`.
  Дополнение по эксплуатации:
  - для local checkpoint path helper должен передавать explicit `pip_requirements`,
    чтобы MLflow не падал на auto-inference отсутствующего `tensorflow` в
    lightweight import image.

## Slice 2. Встроить import runtime в backend image
- цель: сделать import runtime частью module-owned executable layer, а не внешним ad-hoc env;
- файлы:
  - `images/backend/werf.inc.yaml`
  - `images/backend/Dockerfile.local`
  - `images/backend/scripts/smoke-runtime.sh`
- проверки:
  - `make verify`
- артефакт: backend image содержит runtime script и lightweight HF deps.

## Slice 3. Дать phase-1 in-cluster Job helper
- цель: запускать large-model import внутри кластера, без прокачки весов через ноутбук;
- файлы:
  - `tools/run_hf_import_job.sh`
  - `README.md`
  - `README.ru.md`
- проверки:
  - `bash -n tools/run_hf_import_job.sh`
  - `make verify`
- артефакт: operator helper для одноразового HF import Job, который затем сможет
  быть переиспользован будущим controller-owned Job flow.
  Дополнение по эксплуатации:
  - helper должен оставаться совместимым со штатным macOS `/bin/bash` 3.2, а не
    только с GNU bash 4+.
  - helper должен рендерить Kubernetes `env.value` как строки, а не как YAML
    bool/null scalar values.

## Rollback point
- откатить изменения только в `images/backend/*`, `tools/*`, `README*.md` и
  bundle этой задачи.

## Final validation
- `python3 -m py_compile images/backend/scripts/ai-models-backend-hf-import.py`
- `python3 -m py_compile tools/upload_hf_model.py`
- `bash -n tools/run_hf_import_job.sh`
- `make verify`
