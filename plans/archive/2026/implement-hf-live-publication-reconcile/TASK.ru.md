# Implement HF Live Publication Reconcile

## 1. Контекст

В репозитории уже есть:

- public API `Model` / `ClusterModel` и rebased `status`;
- controller runtime shell с HA/metrics/leader election;
- cleanup controller и cleanup Jobs;
- phase-1 backend runtime entrypoint `ai-models-backend-hf-import`;
- internal planning contracts `publication`, `managedbackend`, `runtimedelivery`.

Следующий практический шаг phase-2 — первый рабочий publication path, который
не ограничивается planning-only кодом.

## 2. Постановка задачи

Реализовать первый live reconcile path для `Model` / `ClusterModel` c
`spec.source.type=HuggingFace`:

- controller создаёт и отслеживает controller-owned import Job;
- Job запускает backend runtime import в managed backend `mlflow`;
- controller получает publication result из Job, пишет `status.source`,
  `status.artifact`, `status.resolved`, `phase`, `conditions`;
- controller сохраняет cleanup handle для delete lifecycle;
- при включённом модуле controller deployment уже содержит всё нужное для этого
  live path.

## 3. Scope

- `images/backend/scripts/ai-models-backend-hf-import.py`
- new internal package for HF import Job builder/result contract
- new internal package for live publication reconciler
- `images/controller/internal/app/*`
- `images/controller/cmd/ai-models-controller/run.go`
- `templates/controller/*` if runtime/RBAC wiring needs extension
- affected tests under `images/controller/internal/*`
- minimal docs sync if controller runtime contract changes
- bundle under `plans/active/implement-hf-live-publication-reconcile/*`

## 4. Non-goals

- Не реализовывать сейчас live paths для `Upload`, `HTTP` или `OCIArtifact`.
- Не реализовывать OCI packaging/push в payload-registry.
- Не реализовывать runtime-side materialization agent/PVC mutation.
- Не реализовывать gated/private HF auth through `authSecretRef` в этом slice.
- Не менять сейчас полный public shape `spec.source` beyond what live HF path
  strictly requires.

## 5. Затрагиваемые области

- `images/backend/scripts/`
- `images/controller/internal/`
- `images/controller/cmd/`
- `templates/controller/`
- при необходимости `docs/`

## 6. Критерии приёмки

- Для `Model` и `ClusterModel` с `spec.source.type=HuggingFace` controller
  создаёт import Job и отслеживает его completion/failure.
- После успешного import'а controller пишет:
  - `status.observedGeneration`;
  - `status.phase=Ready`;
  - `status.source.resolvedType/resolvedRevision`;
  - `status.artifact` с сохранённым artifact locator;
  - `status.resolved` с базовым technical profile;
  - public lifecycle conditions без утечки internal backend entities.
- Object получает cleanup handle, который existing delete controller может
  использовать без ручных действий.
- При job failure object переходит в `Failed` с объяснимыми conditions/reasons.
- Controller deployment/RBAC содержит всё нужное для этого flow в кластере.
- Узкие tests на изменённые пакеты проходят.

## 7. Риски

- Слишком широкий slice может смешать live HF path с будущими `Upload`/OCI
  сценариями.
- Непродуманный result handoff из Job в controller даст хрупкий runtime
  контракт.
- Легко протащить MLflow-specific детали в public status вместо artifact-facing
  контракта.
- Нельзя ломать existing cleanup controller, который уже опирается на cleanup
  handle annotation.
