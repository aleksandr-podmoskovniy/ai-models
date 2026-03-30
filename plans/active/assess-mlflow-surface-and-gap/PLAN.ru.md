# PLAN: оценить реальный MLflow surface и gap phase-1 модуля

## Current phase

Этап 1. Внутренний managed backend inside the module.

Задача analysis-only: нужно подтвердить текущий рабочий baseline, отделить
поддержанный phase-1 contract от видимого upstream surface и не смешать это с
phase-2 API каталога моделей.

Режим orchestration: `solo`.

## Slices

### Slice 1. Проверить cluster-side SSO и runtime surface

Цель:
- понять, что реально происходит при входе в `ai-models` и какие backend
  endpoints уже живы в кластере.

Файлы/каталоги:
- `plans/active/assess-mlflow-surface-and-gap/*`

Проверки:
- `kubectl get/describe/logs` по `d8-ai-models`
- `curl -I` по public URL

Артефакт результата:
- список подтверждённых фактов по Dex redirect, backend readiness и реально
  используемым runtime endpoints.

### Slice 2. Сверить repo wiring и phase-1 contract

Цель:
- проверить, что модуль на самом деле wiring'ит как supported surface.

Файлы/каталоги:
- `templates/auth/`
- `templates/backend/`
- `docs/CONFIGURATION*.md`

Проверки:
- read-only inspection repo files

Артефакт результата:
- описание поддержанного phase-1 contract для auth, storage, registry usage и
  observability.

### Slice 3. Сверить upstream MLflow capabilities и gap'ы

Цель:
- отделить доступные upstream capabilities от поддержанного module contract.

Файлы/каталоги:
- `.cache/backend-upstream/mlflow/*`
- при необходимости локальные reference repos

Проверки:
- read-only inspection upstream files

Артефакт результата:
- capability matrix: supported now / visible but not supported / future options.

### Slice 4. Зафиксировать выводы и рекомендации

Цель:
- собрать короткий, предметный ответ пользователю без архитектурного дрейфа.

Файлы/каталоги:
- `plans/active/assess-mlflow-surface-and-gap/*`

Проверки:
- self-review по `AGENTS.md`

Артефакт результата:
- итоговая gap-analysis по SSO, registry flow, Hugging Face artifacts и
  additional MLflow surfaces.

## Rollback point

После завершения анализа без runtime/code changes. Если выводы окажутся
неполными или спорными, bundle можно оставить как analysis-only артефакт без
затрагивания модульного contract.

## Final validation

- Проверить факты по кластеру командами `kubectl` и `curl`.
- Сверить ключевые выводы с текущими templates и docs.
- При отсутствии code/doc changes repo-level `make verify` не обязателен.
