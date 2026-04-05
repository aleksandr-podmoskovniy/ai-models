# PLAN

## 1. Current phase

Задача относится к phase 2: public catalog API и controller semantics вокруг
`Model` / `ClusterModel`.

Orchestration mode: `light`.

Причина:

- сравнивается архитектурный ADR с текущим public API contract;
- нужно не только прочитать документы, но и проверить фактические controller
  boundaries;
- задача read-only, поэтому достаточно одного независимого read-only pass.

Read-only subagent:

- `api_designer`
  - проверить, не разошлись ли public API shape и lifecycle semantics с ADR.

## 2. Slices

### Slice 1. Собрать материалы для сверки

Файлы:

- `plans/active/review-crd-against-internal-adr/TASK.ru.md`
- `plans/active/review-crd-against-internal-adr/PLAN.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

Проверки:

- найден ровно релевантный ADR
- scope текущего review понятен

### Slice 2. Сверить ADR с CRD и controller semantics

Файлы:

- `api/core/v1alpha1/*`
- `images/controller/internal/{publication,runtimedelivery,cleanuphandle,cleanupjob,modelcleanup,managedbackend}/*`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/*`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`

Проверки:

- фактические выводы подтверждаются кодом или bundle docs
- не смешаны implemented и planned части

### Slice 3. Зафиксировать findings

Файлы:

- `plans/active/review-crd-against-internal-adr/REVIEW.ru.md`

Проверки:

- findings конкретные
- residual risks и next step отделены от findings

## 3. Rollback point

Безопасная точка остановки: task bundle создан, ADR прочитан, но review findings
ещё не зафиксированы.

## 4. Final validation

- read-only сверка между ADR и текущим code/bundle state
- наличие `REVIEW.ru.md` с конкретными findings
