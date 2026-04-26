## 1. Заголовок

Привести placement runtime-компонентов ai-models к Deckhouse/virtualization
паттерну

## 2. Контекст

Сейчас `ai-models` держит локальные helpers:

- `ai-models.controlPlaneNodeSelector`;
- `ai-models.systemNodeSelector`;
- `ai-models.controlPlaneTolerations`;
- `ai-models.systemTolerations`.

Из-за этого `DMCR` при отсутствии dedicated `system` nodes падает обратно на
`control-plane`, а в live-кластере все основные Pods модуля оказываются на
master/control-plane нодах. В соседнем модуле `virtualization` используется
общий Deckhouse pattern:

- control-plane/controller компоненты: `helm_lib_node_selector "master"` +
  `helm_lib_tolerations "any-node"`;
- internal registry/runtime компоненты: `helm_lib_node_selector "system"` +
  `helm_lib_tolerations "system"`.

Для `system` strategy общий helper не делает жёсткий fallback на master: если
system nodes нет, selector не рендерится, и Pod может запускаться на обычных
worker nodes.

## 3. Постановка задачи

Нужно заменить локальный ai-models placement на Deckhouse helm-lib placement:

1. Controller должен использовать master-style placement как
   `virtualization-controller`.
2. `DMCR` должен использовать system-style placement как `dvcr`.
3. Убрать лишние локальные helpers, чтобы не было второго placement contract.
4. Зафиксировать render evidence, что `DMCR` больше не получает
   control-plane selector при отсутствии system nodes.

## 4. Scope

- `templates/_helpers.tpl`;
- `templates/controller/deployment.yaml`;
- `templates/dmcr/deployment.yaml`;
- render fixtures/tests, если нужно для helm-lib discovery values;
- текущий task bundle.

## 5. Non-goals

- не вводить новый public `ModuleConfig` knob для placement;
- не менять node-cache substrate selectors;
- не менять publication/cleanup job placement в этом slice;
- не менять RBAC, API, storage или DMCR behaviour.

## 6. Затрагиваемые области

- DKP templates placement;
- module render fixtures/validation;
- operational scheduling behavior controller/DMCR.

## 7. Критерии приёмки

- В templates нет использования локальных
  `ai-models.*NodeSelector`/`ai-models.*Tolerations` helpers.
- Controller рендерится через `helm_lib_node_selector (tuple . "master")` и
  `helm_lib_tolerations (tuple . "any-node")`.
- `DMCR` рендерится через `helm_lib_node_selector (tuple . "system")` и
  `helm_lib_tolerations (tuple . "system")`.
- В baseline render без system nodes `Deployment/dmcr` не содержит
  `node-role.kubernetes.io/control-plane` selector.
- `make helm-template` проходит.
- `git diff --check` проходит.

## 8. Риски

- Если в конкретном кластере нет worker capacity для `DMCR`, после rollout Pod
  может переехать с master на worker и раскрыть скрытые resource constraints.
- Если fixture global discovery неполный, переход на helm-lib helpers может
  сломать локальный render до обновления fixtures.
- Controller intentionally остаётся master-style компонентом, потому что так
  сделано в `virtualization-controller`; требование "не всё на master" закрывает
  прежде всего `DMCR`/registry runtime path.
