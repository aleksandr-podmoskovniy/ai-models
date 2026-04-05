# Implement Backend-Specific Artifact Location And Delete Cleanup

## 1. Контекст

В репозитории уже есть:

- phase-2 public API `Model` / `ClusterModel`;
- controller-side contracts для:
  - upload session lifecycle;
  - publication snapshot;
  - managed backend selection;
  - runtime delivery planning;
- phase-1 managed backend baseline вокруг MLflow under `images/backend/*`.

Текущий design bundle исходил из того, что canonical published artifact всегда
идёт через OCI registry. Пользователь меняет направление:

- runtime delivery должен целиться в always-local materialization path;
- удаление `Model` / `ClusterModel` должно инициировать cleanup модели;
- artifact location теперь зависит от backend/distribution choice:
  - `mlflow` path -> `s3://...`;
  - internal backend / `Harbor` path -> OCI reference.

Нужно скорректировать цели и реализацию так, чтобы получить уже не только
planning layer, но и первый working cleanup-oriented slice.

Read-only delegation уже подсветила важные границы:

- public status не должен тащить `backendKind`, `workspace`, `runID` или другие
  raw backend entities;
- generalized artifact location должен быть public neutral shape:
  `kind + uri + optional digest/mediaType/sizeBytes`;
- actual delete path не может опираться только на public artifact location:
  для MLflow cleanup нужен internal cleanup handle с backend-specific данными;
- чтобы deletion реально работал на CR delete, нужен минимальный live
  finalizer/controller path, а не ещё один pure-library slice.

## 2. Постановка задачи

Сделать bounded implementation slice, который:

- переводит artifact contract с OCI-only на backend-specific location contract;
- сохраняет public API достаточно платформенным, без raw MLflow entity leaks;
- добавляет controller-side deletion cleanup contract для `Model` /
  `ClusterModel`;
- реализует first working cleanup path для backend-dependent artifact location,
  начиная с MLflow/S3;
- оставляет always-local materialization как canonical runtime-delivery goal.

## 3. Scope

- `api/core/v1alpha1/*`
- `api/scripts/*`, если нужен codegen refresh
- `images/controller/internal/*`
- `images/controller/cmd/*`, если нужен bootstrap/config wiring
- `images/controller/README.md`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/*`

Внутри slice:

- public artifact status refactor from OCI-only to backend-specific location;
- controller-side artifact locator contract (`S3` | `OCI`);
- internal cleanup-handle contract for backend-specific delete execution;
- minimal delete-only controller/finalizer path for `Model` / `ClusterModel`;
- first executable cleanup implementation for MLflow/S3 artifact location через
  existing backend cleanup script boundary;
- tests for artifact projection, cleanup planning, and backend-specific behavior.

## 4. Non-goals

- Не переписывать phase-1 backend deployment templates.
- Не строить полный controller-runtime manager со всеми reconcile flows publish /
  sync / runtime-injection; live path здесь ограничивается delete/finalizer.
- Не реализовывать полный Harbor/internal registry delete path, если в этой
  итерации first-class live cleanup закрываем только для MLflow/S3.
- Не тащить raw `Run`, `Workspace`, `Logged Model` в public API.
- Не делать pod mutation/materializer runtime в этом же slice.

## 5. Затрагиваемые области

- `api/*`
- `images/controller/*`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/*`

Reference only:

- `images/backend/scripts/ai-models-backend-model-cleanup.py`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `plans/active/implement-managed-backend-contract-and-runtime-delivery/*`

## 6. Критерии приёмки

- Public status больше не жёстко завязан только на `ociRef`; artifact location
  умеет представлять хотя бы `S3` и `OCI` без raw backend entity leaks.
- Controller-side publication/runtime delivery contracts используют generalized
  artifact locator.
- Есть controller-owned internal cleanup handle для backend-specific delete
  execution без вытаскивания этих данных в public status.
- Есть tested minimal delete-only controller path с finalizer semantics для
  `Model` / `ClusterModel`.
- Есть first working cleanup executor/job path для MLflow/S3 baseline.
- Bundle и docs явно фиксируют новый target: always-local materialization как
  runtime goal и backend-specific artifact location как distribution reality.

## 7. Риски

- Если прямо протащить backend-specific URI в public API без общей модели,
  platform contract быстро станет storage-leaky.
- Если сделать cleanup только по public `S3` URI, MLflow metadata останется
  orphaned; нужен internal cleanup handle.
- Если попытаться одновременно закрыть S3 cleanup, OCI cleanup, finalizer
  wiring, runtime materialization и full reconciler manager, slice снова
  расползётся.
