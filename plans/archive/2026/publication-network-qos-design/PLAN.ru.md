## 1. Current phase

Этап 1. Publication/runtime baseline.

Задача остаётся внутри internal runtime wiring и operator-facing controls.
Public API моделей не меняется.

## 2. Orchestration

Режим: `light`.

Нужны read-only проверки до формализации рекомендации:

- `integration_architect` по platform/CNI границам и reuse паттернов
  `deckhouse`/`virtualization`;
- `backend_integrator` по точкам встраивания в publication runtime без
  ненужной связности.

## 3. Slices

### Slice 1. Собрать локальные evidence

Цель:

- подтвердить реальные механизмы в `deckhouse` и `virtualization`.

Файлы и каталоги:

- `deckhouse/modules/021-cni-cilium/*`
- `virtualization/templates/kubevirt/*`
- `virtualization/images/hooks/pkg/hooks/migration-config/*`
- `virtualization/images/virtualization-artifact/pkg/common/network_policy/*`
- `plans/active/publication-network-qos-design/*`

Проверки:

- targeted `rg`/`sed` по найденным путям;
- фиксация выводов в bundle.

Артефакт:

- локальные выводы по:
  - bandwidth manager;
  - pod annotations;
  - роли `NetworkPolicy`;
  - migration bandwidth pattern в `virtualization`.

### Slice 2. Подтвердить внешними primary refs

Цель:

- проверить, что локальные выводы совпадают с официальной документацией
  Kubernetes и Cilium.

Файлы и каталоги:

- `plans/active/publication-network-qos-design/*`

Проверки:

- официальные docs `kubernetes.io`;
- официальные docs `docs.cilium.io`.

Артефакт:

- source-backed вывод по возможностям и ограничениям:
  - `NetworkPolicy`;
  - pod bandwidth annotations;
  - Cilium bandwidth manager.

### Slice 3. Зафиксировать target design

Цель:

- выбрать defendable recommendation для `ai-models`.

Файлы и каталоги:

- `plans/active/publication-network-qos-design/*`

Проверки:

- manual consistency review against current `ai-models` runtime boundaries.

Артефакт:

- staged recommendation:
  - что можно сделать сразу;
  - что требует отдельного implementation slice;
  - что отклоняем.

## 4. Rollback point

Безопасная точка остановки: после design-only bundle без кода.

## 5. Final validation

- `git diff --check`
