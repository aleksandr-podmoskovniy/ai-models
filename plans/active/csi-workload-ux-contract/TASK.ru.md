# CSI workload UX and SharedPVC delivery contract

## Задача

Упростить user-facing delivery contract для моделей в workload и выровнять
runtime delivery modes:

- `nodeCache.enabled=true` -> только `SharedDirect` через ai-models CSI поверх
  managed SDS local node-cache;
- `nodeCache.enabled=false` -> целевой `SharedPVC` через RWX StorageClass;
- если ни один режим недоступен -> `Disabled/Blocked` с понятной condition,
  без hidden fallback;
- убрать legacy reference/rename annotation из code, templates, docs и tests.

Пользователь объявляет только имена моделей:

```yaml
metadata:
  annotations:
    ai.deckhouse.io/clustermodel: qwen3-14b,bge-m3
```

Модели всегда доступны по стабильному пути:

```text
/data/modelcache/models/<model-name>
```

## Scope

- Перевести parser annotations на comma-separated списки в
  `ai.deckhouse.io/model` и `ai.deckhouse.io/clustermodel`.
- Удалить legacy reference/rename UX из code/docs/tests.
- Сохранить controller-owned CSI stamping для `SharedDirect`.
- Зафиксировать целевой `SharedPVC` contract: controller-owned RWX PVC в
  namespace workload'а per owner/model-set, no RWO delivery, no workload
  namespace registry credentials.
- Обновить пользовательские docs так, чтобы они описывали конечную реализацию,
  а не историю разработки.

## Non-goals

- Не возвращать materialize/PVC fallback через namespace Secret.
- Не добавлять RWO как user-facing delivery mode.
- Не делать быстрый небезопасный SharedPVC через копирование DMCR read secret в
  workload namespace.
- Не добавлять Ray/KubeRay-specific branches.
- Не проектировать полный ai-inference scheduler в этом slice.

## Acceptance criteria

- `ai.deckhouse.io/model` принимает `name-a,name-b` и создаёт пути
  `/data/modelcache/models/name-a`, `/data/modelcache/models/name-b`.
- `ai.deckhouse.io/clustermodel` работает аналогично.
- Одновременное использование `model` и `clustermodel` допустимо, если имена не
  конфликтуют по target path; конфликт fail-closed.
- legacy reference/rename annotation не участвует в webhook, parser или tests.
- В single-model и multi-model случаях нет `/data/modelcache/model` как
  special-case path.
- `SharedDirect` остаётся единственным реализованным runtime path при
  `nodeCache.enabled=true`.
- `SharedPVC` имеет явный public admin knob `sharedPVC.storageClassName`,
  controller-owned RWX PVC foundation и fail-closed gate до digest-scoped
  materializer readiness; запрещён unsafe shortcut через общие registry secrets
  в workload namespace.
- Docs/FAQ/Examples/User guide не содержат вопросов и объяснений, завязанных на
  ход разработки вместо конечного UX.

## Validation

- `cd images/controller && go test ./internal/nodecache ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/workloaddelivery`
- `git diff --check`
- `make verify` если focused checks зелёные и окружение позволяет.

## Rollback point

Вернуть предыдущий rename parser и `/data/modelcache/model` primary path без
возврата PVC/materialize fallback и без Ray-specific reconciliation.
