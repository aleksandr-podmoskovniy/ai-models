# Harden HF Import Metadata For Prod UI

## 1. Контекст

Внутренний backend уже умеет импортировать модели из Hugging Face в MLflow и S3 через in-cluster Job. На живом кластере импорт `google/gemma-3-4b-it` завершился успешно, но карточка модели в MLflow UI остаётся почти пустой: нет description, tags, полезных run params и понятных артефактов с metadata.

При этом апстримный `mlflow.transformers` уже умеет сохранять model card, license и часть flavor metadata. Текущий importer использует local checkpoint path ради memory-safe large-model flow, но почти не переносит HF metadata в UI-visible поверхности MLflow.

## 2. Постановка задачи

Сделать HF import production-ready с точки зрения апстримно-совместимой metadata: importer должен после импорта оставлять в MLflow понятные description/tags/params/artifacts, чтобы пользователь видел не только факт регистрации модели, но и основную информацию о модели и snapshot без отдельного S3 browser.

## 3. Scope

- доработать runtime importer для HF -> MLflow import;
- использовать апстримные возможности `mlflow.transformers` и `huggingface_hub`, а не самодельный UI;
- обогатить run/model/model-version metadata и артефакты;
- обновить docs и проверки.

## 4. Non-goals

- не делать отдельный custom UI поверх MLflow;
- не вводить phase-2 `Model` / `ClusterModel`;
- не менять storage path, import job orchestration или SSO/workspace архитектуру;
- не обещать универсальную rich schema для всех multimodal tasks, если апстримный flavor её не даёт без загрузки модели в RAM.

## 5. Затрагиваемые области

- `images/backend/scripts/ai-models-backend-hf-import.py`
- `tools/run_hf_import_job.sh`
- `tools/helm-tests/validate-renders.py`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- при необходимости related tests / smoke helpers

## 6. Критерии приёмки

- после HF import в MLflow остаются осмысленные model / model-version tags и description;
- run содержит полезные HF-related params/tags вместо почти пустой metadata;
- importer логирует отдельные artifacts с HF metadata / snapshot manifest;
- решение опирается на апстримный `mlflow.transformers` contract, а не на custom registry UI;
- `make verify` проходит.

## 7. Риски

- local checkpoint path в `mlflow.transformers` имеет ограничения по automatic signature/model-card inference;
- multimodal tasks могут не давать полноценную schema без загрузки модели;
- легко перегрузить UI слишком шумными tags/params, если логировать весь snapshot без фильтрации.
