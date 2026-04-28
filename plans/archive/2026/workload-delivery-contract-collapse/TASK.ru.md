# Workload delivery contract collapse

## Контекст

После SharedDirect/node-cache и upload gateway slices часть стабильного
workload-delivery contract всё ещё живёт в concrete K8s adapter
`adapters/k8s/modeldelivery`. Из-за этого `monitoring/runtimehealth` импортирует
adapter только ради annotation keys, delivery mode/reason values и имени
managed materializer init container.

Это нарушает hexagonal split: monitoring должен читать публичные/стабильные
signals с workload templates/pods, но не зависеть от mutating adapter.

## Scope

- Вынести стабильные workload delivery annotations, delivery mode/reason values
  и managed materializer init container name в shared internal contract package.
- Оставить concrete Pod mutation/rendering в `adapters/k8s/modeldelivery`.
- Перевести `internal/nodecache` на shared annotation/mode/reason constants,
  чтобы node-cache desired-artifacts contract не дублировал workload metadata.
- Перевести `monitoring/runtimehealth` на shared contract вместо adapter import.
- Сохранить публичные annotation names и metric label values без изменений.

## Non-goals

- Не менять public annotations.
- Не менять workload delivery behavior, CSI volume rendering или scheduling gate.
- Не менять RBAC/templates.
- Не проектировать новый public API.

## Acceptance criteria

- `monitoring/runtimehealth` больше не импортирует `adapters/k8s/modeldelivery`.
- `nodecache` не владеет generic workload delivery annotation names, только
  использует shared contract.
- `modeldelivery` остаётся adapter boundary и алиасит shared constants для
  обратной совместимости с текущими call sites.
- Targeted tests for runtimehealth, nodecache and modeldelivery pass.
- `git diff --check` passes.

## Rollback

Откатить новый shared package и вернуть constants в `modeldelivery` / `nodecache`.
