# PLAN

## 1. Current phase

Этап 1: внутренний managed backend внутри модуля. Задача усиливает production readiness внутреннего backend import flow и не вытаскивает phase-2 catalog API наружу.

## 2. Orchestration

`solo`

Причина: основной риск локализован в одном slice вокруг HF importer metadata contract; read-only subagents здесь добавят мало сигнала.

## 3. Slices

### Slice 1. Upstream-aligned HF metadata enrichment

- Цель: доработать importer так, чтобы он использовал апстримные `mlflow.transformers` и `huggingface_hub` metadata surfaces и оставлял содержательную MLflow metadata после large-model import.
- Файлы:
  - `images/backend/scripts/ai-models-backend-hf-import.py`
  - при необходимости `tools/run_hf_import_job.sh`
- Проверки:
  - узкий smoke importer на синтаксис/CLI
  - локальная проверка helper logic
- Результат:
  - HF metadata, manifest и registry-facing tags/description логируются вместе с моделью.

### Slice 2. Guards and docs

- Цель: зафиксировать новый contract в docs и validation.
- Файлы:
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `make helm-template`
  - `make verify`
- Результат:
  - docs и guardrails согласованы с importer behavior.

## 4. Rollback point

До изменений в `ai-models-backend-hf-import.py`: если production-grade metadata enrichment оказывается слишком спорным, можно вернуться к current minimal importer без затрагивания storage/auth/runtime architecture.

## 5. Final validation

- `make verify`
