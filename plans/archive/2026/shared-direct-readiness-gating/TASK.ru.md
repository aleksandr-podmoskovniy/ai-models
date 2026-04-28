# SharedDirect readiness gating

## 1. Заголовок

Исправить readiness gating для managed SharedDirect delivery.

## 2. Контекст

Managed SharedDirect delivery уже умеет добавлять inline CSI volume,
workload-facing env contract и node-cache node selector. Перед тяжёлым e2e
тестом node-cache/shared-direct нельзя отпускать workload в scheduling только
по факту наличия любой ready node-cache ноды в кластере.

Текущий риск: workload может иметь собственный `nodeSelector`, node affinity
или tolerations/taints surface, несовместимый с ready node-cache нодами. В таком
случае контроллер должен оставить scheduling gate до появления реально
подходящей готовой ноды, иначе kubelet/CSI получат поздний mount failure вместо
понятного controller-owned ожидания.

## 3. Постановка задачи

Сделать readiness check topology-aware и workload-aware: managed SharedDirect
может снять `ai.deckhouse.io/model-delivery` scheduling gate только если есть
хотя бы одна ready node-cache нода, которая удовлетворяет scheduling
constraints итогового Pod template.

## 4. Scope

- Проверять ready node-cache ноды после применения managed cache template
  state.
- Учитывать `PodSpec.NodeSelector`.
- Учитывать required node affinity.
- Учитывать untolerated `NoSchedule` / `NoExecute` taints.
- Добавить regression tests на mismatch по selector/affinity/taint.
- Оставить controller/adapters границы без новых публичных API.

## 5. Non-goals

- Не менять upload ingress/controller identity split.
- Не менять node-cache desired-artifacts contract.
- Не менять public `Model` / `ClusterModel` API, CRD или RBAC.
- Не запускать live e2e в рамках этого slice.
- Не проектировать новые LM Studio-like capability профили.

## 6. Затрагиваемые области

- `images/controller/internal/adapters/k8s/modeldelivery/`
- `images/controller/internal/controllers/workloaddelivery/` только tests, если
  нужен owner-level regression.
- `plans/active/shared-direct-readiness-gating/`

## 7. Критерии приёмки

- SharedDirect delivery оставляет scheduling gate, если нет подходящей ready
  node-cache ноды.
- SharedDirect delivery снимает gate только при наличии ready ноды,
  совместимой с итоговым Pod template.
- Тесты покрывают конфликт workload selector, required affinity и untolerated
  taint.
- Изменение не добавляет публичный API/RBAC/exposure/runtime entrypoint.
- Файлы остаются в LOC discipline: без монолитного scheduler clone и без
  смешивания K8s shaping с domain policy.

## 8. Риски

- Неполное копирование Kubernetes scheduler semantics может дать false-positive
  ready result. Поэтому slice ограничен строгими базовыми constraints:
  nodeSelector, required node affinity и hard taints.
- Слишком широкая логика в `modeldelivery` может превратить adapter в
  scheduling subsystem. Поэтому helper должен оставаться узким guardrail для
  module-managed SharedDirect.
