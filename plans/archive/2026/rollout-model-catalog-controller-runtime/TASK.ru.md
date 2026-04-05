# Rollout Model Catalog Controller Runtime

## 1. Контекст

В репозитории уже есть:

- phase-1 managed backend как полноценный DKP module component;
- phase-2 public API types `Model` / `ClusterModel`;
- controller code under `images/controller/*` для:
  - upload session planning;
  - registry path planning;
  - access planning;
  - managed backend planning;
  - runtime delivery planning;
  - delete-only cleanup runtime.

Но сейчас отсутствует обязательный operational baseline для phase 2:

- controller не развёртывается модулем в кластере;
- для него нет module manifests, image wiring, RBAC, Service/ServiceMonitor;
- leader election и metrics/health runtime shape не доведены до module-ready
  состояния;
- `Model` / `ClusterModel` CRD schema генерируется локально, но ещё не
  устанавливается модулем в кластер.

Пока этот слой не доведён, любой следующий HF/import/publish slice останется
локальной библиотекой, а не cluster feature.

## 2. Постановка задачи

Сделать первый operational phase-2 slice, который:

- добавляет controller image в module build;
- разворачивает controller как полноценный module runtime component;
- включает HA-ready controller-manager shape:
  - leader election;
  - health probes;
  - metrics endpoint;
  - ServiceMonitor;
- добавляет CRD rollout для `Model` / `ClusterModel` в cluster lifecycle
  модуля;
- оформляет values/helpers/docs так, чтобы следующий reconcile slice мог уже
  работать в реальном кластере.

## 3. Scope

- `images/controller/*`
- `images/hooks/*`
- `templates/*`
- `openapi/*`
- `werf.yaml`
- `.werf/stages/*`, если нужно image wiring
- `api/scripts/*`, если нужен reproducible CRD export path
- `docs/CONFIGURATION*.md`
- `plans/active/rollout-model-catalog-controller-runtime/*`

## 4. Non-goals

- Не реализовывать в этом slice HF import reconcile loop.
- Не делать live publish/promote flow в payload-registry.
- Не делать live backend sync executor.
- Не реализовывать KubeRay/vLLM materializer agent.
- Не пытаться одновременно закрыть все phase-2 workflows одним изменением.

## 5. Затрагиваемые области

- module shell:
  - `templates/`
  - `openapi/`
  - `images/hooks/`
  - werf image wiring
- controller runtime:
  - `images/controller/`
- docs/review:
  - `docs/CONFIGURATION*.md`
  - `plans/active/rollout-model-catalog-controller-runtime/*`

## 6. Критерии приёмки

- В module build появляется отдельный controller image.
- Модуль рендерит controller Deployment/ServiceAccount/RBAC/Service/ServiceMonitor/PDB.
- Controller manager запускается с leader election и опубликованным metrics
  endpoint.
- `Model` / `ClusterModel` CRD schema устанавливается в кластер module-owned
  способом, а не остаётся только локально проверяемым генератором.
- Релевантные runtime-only module values/helpers присутствуют и согласованы.
- Docs больше не создают впечатление, что phase-2 controller ещё не существует
  как module runtime component.

## 7. Риски

- Если rollout controller сделать без CRD install, module даст process без
  usable API.
- Если CRD install сделать шаблонами в `templates/`, layout быстро станет
  лоскутным; лучше reuse module hook pattern для ensure CRDs.
- Если включить controller без leader election и metrics/health, потом придётся
  ломать deployment shape уже в live phase-2 rollout.
- Если в этот же slice протащить publish workers и HF logic, потеряется
  операционная чёткость и rollback point.
