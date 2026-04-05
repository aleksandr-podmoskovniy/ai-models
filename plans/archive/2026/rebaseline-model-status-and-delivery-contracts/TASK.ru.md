# Rebaseline Model Status And Delivery Contracts

## 1. Контекст

Для `Model` / `ClusterModel` уже зафиксирован target shape в bundle
`define-model-status-and-runtime-delivery-target-shape`.

Следующий implementation slice должен перевести это решение в код:

- public `status` в `api/core/v1alpha1`;
- generated CRD;
- internal contracts в `publication` и `runtimedelivery`.

При этом scope сознательно ограничен status/delivery boundary и не включает
полный rebaseline `spec.source`.

## 2. Постановка задачи

Реализовать в коде:

- `status.resolved` вместо current `status.metadata`;
- public artifact class `OCI | ObjectStorage` вместо `OCI | S3`;
- public phase/condition cleanup без internal-only orchestration markers;
- internal publication contract вокруг `ResolvedProfile`;
- internal runtime delivery contract вокруг explicit `AccessPlan` и
  `VerificationPlan`.

## 3. Scope

- `api/core/v1alpha1/types.go`
- `api/core/v1alpha1/conditions.go`
- `api/core/v1alpha1/*_test.go` if needed
- generated `crds/ai-models.deckhouse.io_models.yaml`
- generated `crds/ai-models.deckhouse.io_clustermodels.yaml`
- `images/controller/internal/publication/*`
- `images/controller/internal/runtimedelivery/*`
- affected tests under `images/controller/internal/*`
- bundle under `plans/active/rebaseline-model-status-and-delivery-contracts/*`

## 4. Non-goals

- Не трогать сейчас полный shape `spec.source`.
- Не materialize'ить live runtime agent/pod mutation.
- Не реализовывать live registry RBAC materialization или S3 STS issuance.
- Не менять docs/ADR кроме случаев, если code comments требуют минимальной sync.

## 5. Затрагиваемые области

- `api/`
- `crds/`
- `images/controller/internal/publication/`
- `images/controller/internal/runtimedelivery/`
- при необходимости controller tests in adjacent packages

## 6. Критерии приёмки

- `ModelStatus` и generated CRD соответствуют target shape:
  - `resolved` вместо `metadata`;
  - `ObjectStorage` вместо `S3`;
  - без `Syncing`, `ArtifactStaged`, `AccessConfigured`,
    `BackendSynchronized`.
- `publication` internal contract использует `ResolvedProfile`.
- `runtimedelivery.Plan` различает artifact class, access mode и verification
  plan.
- Тесты на изменённые пакеты проходят.

## 7. Риски

- Неполный rebaseline сломает существующие tests вокруг cleanup/uploadsession.
- Generated CRD может разъехаться с `types.go`, если пропустить codegen.
- Можно случайно протащить internal auth details в public API.
