# Implement Managed Backend Contract And Local Runtime Delivery Model

## 1. Контекст

В репозитории уже есть:

- phase-1 managed backend baseline вокруг MLflow under `images/backend/*`;
- phase-2 public API `Model` / `ClusterModel`;
- controller-side pure libraries для publish plane:
  - registry path conventions;
  - upload session lifecycle;
  - published access planning;
- design bundle, который фиксирует:
  - internal backend как metadata/provenance/evaluation sink, а не canonical
    serving registry;
  - published artifact через OCI registry;
  - `KServe` first-class consumer и `KubeRay` adapter path.

Пользователь хочет ввести более явную contract model:

- managed backend внутри контроллера должен быть switchable:
  - initial implementation: `mlflow`;
  - later implementation: module-owned internal backend;
- end-to-end scenario должен покрывать:
  - source download/import;
  - publication в registry;
  - metadata generation;
  - backend mirror;
  - unified consumer delivery path, где runtime получает локальную модель на
    PVC/shared volume, а не raw S3/MLflow serving semantics.

## 2. Постановка задачи

Реализовать первый bounded slice новой модели:

- ввести internal managed-backend abstraction для controller-side orchestration;
- добавить first implementation selection для `mlflow`;
- ввести unified runtime-delivery contract, где published artifact
  materialize'ится в local path on shared volume/PVC через общий pull/mount
  pattern;
- зафиксировать contract types и tests так, чтобы следующий runtime slice мог
  поверх этого добавить live worker execution, backend sync и pod injection.

## 3. Scope

- `images/controller/internal/*`
- `images/controller/cmd/*`, если нужен config plumbing
- `images/controller/README.md`
- `plans/active/implement-managed-backend-contract-and-runtime-delivery/*`

Внутри slice:

- managed backend kind/config contract (`mlflow` | `internal`);
- controller-side publication/backend sync request model;
- first MLflow adapter as internal implementation of that contract;
- unified runtime-delivery contract for local-PVC/shared-volume model
  materialization for `vllm`, `KubeRay`, `KServe` and generic runtimes;
- tests на backend selection, MLflow mapping, and runtime delivery plan.

## 4. Non-goals

- Не менять phase-1 backend deployment templates.
- Не делать live backend API calls или database migrations из controller.
- Не реализовывать actual HF publish worker и actual promote worker в этом
  slice.
- Не materialize'ить pod mutations, sidecars, init-containers или PVC objects в
  кластере.
- Не тащить backend kind или runtime credentials в public `Model` /
  `ClusterModel` status.
- Не заменять canonical publish plane: published artifact остаётся OCI registry,
  а backend остаётся internal mirror.

## 5. Затрагиваемые области

- `images/controller/*`
- `plans/active/implement-managed-backend-contract-and-runtime-delivery/*`

Reference only:

- `images/backend/*`
- `plans/active/design-backend-isolation-and-storage-strategy/*`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `plans/active/design-kuberay-rgw-s3-consumption/*`

## 6. Критерии приёмки

- В controller code есть явный managed-backend contract, который не завязан на
  raw MLflow entities как на public platform API.
- Backend selection поддерживает как минимум:
  - `mlflow`
  - placeholder/future `internal`
- Есть tested MLflow-first adapter, который маппит publication metadata и
  artifact refs в internal backend sync request/plan.
- Есть tested runtime-delivery contract, который описывает единый
  local-materialization path для runtime pods и не заставляет consumer contract
  зависеть от raw S3 path.
- Slice остаётся в pure planning / adapter boundary без live runtime side
  effects.

## 7. Риски

- Если backend abstraction сделать вокруг raw MLflow terms (`run`, `workspace`,
  `logged model`), later backend swap станет фикцией.
- Если единый runtime-delivery path смешать с public API status, туда быстро
  протекут credentials и runtime-specific plumbing.
- Если попытаться сразу materialize'ить workers, backend sync и pod mutation,
  задача расползётся в несколько phase-2 slices и потеряет reviewability.
