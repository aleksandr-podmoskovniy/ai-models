# Prod preflight: workload delivery projected-secret cleanup finalizer

## Контекст

`workloaddelivery` может создавать registry auth / CA / imagePull Secret в
namespace workload'а для `materialize-artifact` fallback. Kubernetes запрещает
cross-namespace ownerReference, поэтому эти Secret не могут быть надёжно
привязаны к `Model` / `ClusterModel`. Сейчас cleanup выполняется при снятии
аннотаций, но при удалении workload без предварительного снятия аннотаций
projected Secrets могут стать orphaned.

## Постановка задачи

Добавить controller finalizer на mutated workload и выполнять cleanup projected
Secrets до удаления workload:

- при успешной delivery mutation добавлять workload finalizer;
- при снятии annotations / managed state удалять projected Secrets и finalizer;
- при `deletionTimestamp` удалять projected Secrets и finalizer;
- не менять public `Model` / `ClusterModel` API и human RBAC.

## Scope

- `images/controller/internal/controllers/workloaddelivery/*`
- tests for apply, cleanup and delete-time finalization

## Non-goals

- Не решать весь residual cluster-wide Secret write вопрос: это отдельный
  delivery-auth redesign.
- Не менять delivery topology или annotations UX.
- Не менять RBAC в этом slice.

## Acceptance criteria

- Mutated workload gets `ai.deckhouse.io/model-delivery-cleanup` finalizer.
- Annotation removal removes managed state, projected Secrets and finalizer.
- Deleting a finalized workload removes projected Secrets and finalizer.
- Existing one-reconcile apply UX remains unchanged.
- Targeted controller tests and repo verify pass.

## RBAC coverage

Human RBAC не меняется. Controller уже имеет update/patch on supported workload
resources; finalizer stored in workload metadata uses the same patch path.

## Риски

- Finalizer на пользовательских workloads должен быть short-lived on delete:
  cleanup must ignore missing projected Secrets and must not block deletion
  when no managed delivery resources exist.
