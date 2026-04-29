# TASK: Remove workload materialize fallback and cluster-wide Secret writes

## Контекст

В текущей delivery-схеме исторический `materialize-artifact` fallback требует
проецировать DMCR read credentials и runtime image pull secret в namespace
пользовательского workload. Из-за этого controller получает cluster-wide
`secrets` write, что плохо для RBAC и не соответствует целевой картине:
runtime delivery должен идти через module-owned node-cache/CSI path, а не через
per-workload initContainer с Secret'ами в пользовательских namespace.

## Scope

- Найти и удалить materialize fallback из workload delivery.
- Убрать необходимость создавать/обновлять/удалять Secret'ы в workload
  namespaces.
- Перевести workload delivery на fail-closed target path:
  - `SharedDirect`/node-cache CSI готов — workload мутируется;
  - node-cache/CSI не готов — workload получает понятный condition/backoff и не
    получает legacy materializer.
- Сузить controller RBAC: оставить Secret writes только в module namespace для
  module-owned state, auth and runtime internals.
- Обновить тесты, helm render checks, e2e runbook and RBAC evidence.

## Non-goals

- Не оставлять совместимый fallback через initContainer.
- Не раздавать DMCR credentials в пользовательские namespace.
- Не добавлять новый пользовательский knob для выбора legacy delivery.
- Не менять `Model` / `ClusterModel` public source API.
- Не делать FUSE/lazy chunked delivery в этом slice.

## Acceptance criteria

- В workload namespace не создаются controller-owned Secret'ы для delivery.
- Workload mutation больше не добавляет `materialize-artifact` initContainer,
  runtime pull secret или OCI env credentials.
- Controller ClusterRole не имеет cluster-wide `create/update/patch/delete` для
  core `secrets`.
- SharedDirect gating остаётся fail-closed и даёт стабильный reason без event
  spam.
- Старые projected Secret'ы и materializer initContainers чистятся при касании
  managed workload.
- Unit tests покрывают removal, refusal and stale cleanup paths.
- Helm/static checks подтверждают RBAC и render shape.

## RBAC coverage

- Human RBAC: без изменений в праве пользователей читать/изменять `Model` /
  `ClusterModel`; Secret, pod exec/attach/portforward, status/finalizers remain
  denied as before.
- Controller SA:
  - allowed: module namespace Secret writes for module-owned state;
  - denied by template: cluster-wide Secret writes into workload namespaces.
- Runtime SA:
  - node-cache runtime keeps only node/runtime permissions needed for CSI and
    cache operation.

## Orchestration

Mode: `full`.

Required read-only reviews before code implementation:

- `integration_architect`: delivery-auth, RBAC, storage/runtime ownership.
- `api_designer`: condition/status and Kubernetes API convention impact.

## Rollback point

Revert this bundle's production diff and restore previous materialize fallback
only if SharedDirect/CSI rollout is not available. This rollback is intentionally
not a supported runtime mode after the target slice lands.
