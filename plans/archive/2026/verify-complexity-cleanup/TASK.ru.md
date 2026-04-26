## 1. Заголовок

Починить `make verify` после complexity gate failure

## 2. Контекст

После последних изменений `make verify` падает не на тестах и не на шаблонах,
а на `lint-controller-complexity`. Блокируются три функции:

- `oci.pushRawLayerDirectToBackingStorage`
- `oci.pushDescribedLayerDirectToBackingStorage`
- `sourceworker.(*Service).GetOrCreate`

Это corrective cleanup, а не новая функциональность. Нужно пройти quality gate
без дрейфа поведения.

## 3. Постановка задачи

Нужно уменьшить цикломатическую сложность трёх функций до прохождения текущего
репозиторного лимита и сохранить существующее поведение:

- direct-upload resume/recovery/checkpoint logic не ломается;
- sourceworker lifecycle не меняет семантику очереди, recreate failed pod,
  projected secrets и handle shaping;
- после правки `make verify` проходит целиком.

## 4. Scope

- разрезать `direct_upload_transport_raw.go` на более узкие helpers;
- разрезать `direct_upload_transport.go` на более узкие helpers;
- разрезать `sourceworker/service.go` на более узкие helpers;
- при необходимости скорректировать узкие тесты;
- прогнать проверки до зелёного `make verify`.

## 5. Non-goals

- не менять published contract;
- не менять DMCR direct-upload protocol;
- не менять sourceworker UX, statuses или scheduling semantics;
- не трогать unrelated legacy cleanup.

## 6. Затрагиваемые области

- `plans/active/verify-complexity-cleanup/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`

## 7. Критерии приёмки

- `make verify` проходит из корня репозитория;
- три перечисленные функции больше не валят `lint-controller-complexity`;
- поведение direct-upload resume/recovery и sourceworker `GetOrCreate` покрыто
  существующими или скорректированными узкими тестами;
- дифф не добавляет новый слой абстракций без причины и не размазывает
  ответственность между пакетами.

## 8. Риски

- легко “починить” complexity через искусственные helper-обёртки и сделать код
  хуже;
- легко повредить resume/recovery path в direct-upload;
- легко внести регрессию в очередь publication workers или recreate failed pod.
