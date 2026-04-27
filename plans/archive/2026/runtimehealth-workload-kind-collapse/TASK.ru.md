# Runtimehealth workload kind collapse

## 1. Заголовок

Сократить повторяющиеся workload-kind collectors в runtimehealth без изменения
Prometheus metric contract.

## 2. Контекст

`internal/monitoring/runtimehealth` остаётся observability boundary для
runtime-plane health. В workload-delivery части четыре почти одинаковых list
paths для `Deployment`, `StatefulSet`, `DaemonSet` и `CronJob` повторяют
одинаковую схему: list typed workload, extract pod template, count delivery
annotations.

Это package-local cleanup. Метрики, labels, workload delivery behavior, node
cache byte path, DMCR GC semantics и public API не меняются.

## 3. Постановка задачи

- Заменить четыре повторяющихся workload lister functions одним
  package-local kind table / scanner helper.
- Сохранить metric names, labels and cardinality.
- Не менять scrape-time source of truth в этом slice.
- Зафиксировать отдельно найденные крупные runtime risks: `SharedDirect`
  readiness gating and public upload ingress/controller identity split.

## 4. Scope

- `images/controller/internal/monitoring/runtimehealth/workload_delivery.go`
- adjacent runtimehealth tests if needed.
- `plans/active/runtimehealth-workload-kind-collapse/`

## 5. Non-goals

- Не удалять и не переименовывать Prometheus metrics.
- Не менять modeldelivery annotations или controller runtime behavior.
- Не менять API/RBAC/templates/runtime entrypoints.
- Не решать здесь `SharedDirect` readiness gating.
- Не решать здесь upload ingress/controller ServiceAccount split.
- Не создавать cross-package monitoring abstraction.

## 6. Критерии приёмки

- Workload delivery workload-kind counting uses one local scanner path.
- Existing workload delivery metrics stay unchanged.
- Runtimehealth package tests pass.
- Repeated/race tests pass for runtimehealth.
- Files remain LOC<350.
- `git diff --check` passes.
- `make verify` passes before handoff if feasible.

## 7. Риски

- Ошибка kind table может silently перестать считать один workload kind.
- Случайное изменение metric labels может сломать dashboards.
- Слишком общий scanner может создать скрытую observability abstraction вместо
  простой package-local cleanup.
