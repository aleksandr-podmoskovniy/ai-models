# Node-cache desired-artifacts contract

## 1. Заголовок

Вынести desired-artifacts contract из adapter-to-adapter coupling.

## 2. Контекст

`k8s/nodecacheruntime` сейчас импортирует `k8s/modeldelivery`, чтобы прочитать
workload delivery annotations и понять, какие OCI artifacts нужны на конкретной
ноде. Это связывает два concrete K8s adapter package напрямую и делает
node-cache runtime зависимым от writer implementation вместо shared runtime
contract.

## 3. Постановка задачи

Сделать `internal/nodecache` единственным shared contract для desired artifacts:
modeldelivery пишет стабильные annotation keys/value через этот contract, а
nodecacheruntime читает его без импорта modeldelivery.

## 4. Scope

- Добавить в `internal/nodecache` pure parser для desired artifacts из workload
  annotations.
- Сделать modeldelivery annotation constants aliases на nodecache contract.
- Убрать import `modeldelivery` из `k8s/nodecacheruntime`.
- Перевести tests на shared contract constants.

## 5. Non-goals

- Не менять annotation names и JSON shape.
- Не менять CSI publish protocol.
- Не менять controller watches/RBAC/templates.
- Не запускать live e2e.
- Не расширять LM Studio-like capability metadata в этом slice.

## 6. Затрагиваемые области

- `images/controller/internal/nodecache/`
- `images/controller/internal/adapters/k8s/modeldelivery/options.go`
- `images/controller/internal/adapters/k8s/nodecacheruntime/`

## 7. Критерии приёмки

- `k8s/nodecacheruntime` не импортирует `k8s/modeldelivery`.
- Desired artifacts parser живёт в shared node-cache contract и не импортирует
  Kubernetes packages.
- Существующие annotation names/value остаются совместимыми.
- Tests покрывают single-model и multi-model annotations через nodecache
  contract.

## 8. Риски

- Смена names/value в annotations сломает совместимость live workload'ов.
  Поэтому modeldelivery получает только alias на прежние строки.
- Если parser станет знать Kubernetes Pod API, shared contract снова смешает
  K8s adapter с runtime contract. Parser должен принимать только map
  annotations.
