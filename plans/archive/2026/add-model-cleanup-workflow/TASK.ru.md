# Add Model Cleanup Workflow

## Контекст

В phase-1 модуль уже умеет импортировать модели из Hugging Face во внутренний
MLflow backend и S3-compatible storage через in-cluster Job. При этом удаление
через UI/API `Model Version` не удаляет связанные `Logged Model`, `Run` и
artifact prefixes в S3. Это соответствует семантике upstream MLflow, но
оставляет module-side эксплуатационный gap: оператору приходится вручную
дочищать `logged model` и S3.

## Постановка задачи

Добавить штатный phase-1 cleanup workflow, который позволяет удалить
зарегистрированную версию модели и связанные internal backend entities и
artifacts без ручной работы в S3.

## Scope

- image-owned cleanup runtime entrypoint под `images/backend/scripts`;
- in-cluster operator helper под `tools/` для запуска cleanup Job текущим backend
  image;
- direct cleanup связанного `logged model`, `run` и S3 artifact prefixes через
  machine-only path;
- документация и проверки для нового cleanup workflow.

## Non-goals

- не делать phase-2 CRD/API UX для удаления моделей;
- не менять upstream MLflow delete semantics;
- не включать auto-delete старых версий при каждом новом import;
- не поддерживать storage backends кроме S3-compatible path, который уже является
  phase-1 baseline модуля.

## Затрагиваемые области

- `images/backend/scripts/*`
- `images/backend/werf.inc.yaml`
- `tools/*`
- `docs/*`
- `plans/active/add-model-cleanup-workflow/*`

## Критерии приёмки

- есть image-owned cleanup entrypoint, который по имени модели и версии умеет
  удалить `Model Version`, связанный `Logged Model`, связанный `Run` и их S3
  artifacts;
- есть operator helper для запуска cleanup как in-cluster Job через текущий
  backend image;
- cleanup использует те же machine creds и S3 trust/env bridge, что и import;
- cleanup не удаляет предыдущие версии автоматически во время import;
- релевантные локальные проверки и `make verify` проходят.

## Риски

- можно удалить не тот S3 prefix, если неверно резолвить `run_id` / `model_id`;
- можно сломать cleanup на кластерах с кастомным S3 endpoint/TLS, если не reuse
  текущий S3 env/CA bridge;
- можно случайно зашить destructive default behavior вместо явного operator
  action.
