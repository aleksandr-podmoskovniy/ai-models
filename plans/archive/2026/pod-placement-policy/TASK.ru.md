# Pod placement policy alignment

## 1. Заголовок

Выровнять placement module-owned Pods с Deckhouse/DVCR-style system placement:
не прибивать controller и upload-gateway к masters и не toleration'ить
control-plane без явной необходимости.

## 2. Контекст

Сейчас `controller` и `upload-gateway` используют `helm_lib_node_selector`
со стратегией `master` и `any-node` tolerations, поэтому остаются на
control-plane. В live `k8s.apiac.ru` system-нод нет, но `dmcr` уже корректно
уходит на workers через `helm_lib_node_selector "system"` и
`helm_lib_tolerations "system"`.

Для `ai-models` master fallback не решает проблему: при отсутствии выделенных
system-нод он снова держит управляющие Pods на control-plane. Более подходящий
pattern для этого модуля — тот же, что у DMCR/DVCR-style registry service:
system placement без master/control-plane fallback.

## 3. Scope

- Перевести `controller` и `upload-gateway` с fixed `master`/`any-node` на
  `helm_lib_node_selector "system"` и `helm_lib_tolerations "system"`.
- Оставить `dmcr` на `system`, как DVCR-style registry/backend service.
- Сохранить anti-affinity, priorityClass и VPA labels без
  расширения публичного API.
- Добавить helm render guardrail на отсутствие master/control-plane placement
  для controller и upload-gateway.

## 4. Non-goals

- Не добавлять user-facing nodeSelector/tolerations knobs.
- Не менять node-cache runtime/substrate placement: они завязаны на explicit
  `aiModels.nodeCache.nodeSelector`.
- Не менять workload delivery placement для пользовательских workloads.
- Не менять RBAC.
- Не запускать live cluster migration в этом slice.

## 5. Acceptance Criteria

- При наличии `global.discovery.d8SpecificNodeCountByRole.system > 0`
  rendered controller и upload-gateway получают `node-role.deckhouse.io/system`.
- При отсутствии system nodes rendered controller и upload-gateway не получают
  `node-role.kubernetes.io/control-plane` / `node-role.kubernetes.io/master`
  selector или tolerations.
- `dmcr` продолжает использовать `helm_lib_node_selector "system"` и system
  tolerations.
- В render fixtures есть отдельный `managed-system-role` сценарий.
- `make helm-template` и `make kubeconform` проходят.
- `git diff --check` проходит.

## 6. Риски

- На кластерах без system nodes scheduler будет выбирать подходящие
  non-control-plane nodes. Если все non-control-plane nodes закрыты
  несовместимыми taints, Pods останутся Pending вместо тихого переезда на
  masters. Это intentional fail-closed поведение.
