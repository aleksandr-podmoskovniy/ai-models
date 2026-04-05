# PLAN

## 1. Current phase

Задача относится к phase 2: уточнение и синхронизация public catalog API
architecture вокруг `Model` / `ClusterModel`.

Orchestration mode: `light`.

Причина:

- меняется архитектурный ADR, который описывает public contract;
- нужно сверить его с текущим design bundle и аккуратно обновить narrative;
- задача docs-only, поэтому достаточно одного read-only subagent по структуре
  решения.

Read-only subagent:

- `api_designer`
  - проверить target shape обновлённого ADR и убедиться, что он не возвращает
    нас к старому OCI-only / inference-centric contract.

## 2. Slices

### Slice 1. Собрать текущие источники истины

Файлы:

- `plans/active/update-internal-adr-for-current-model-catalog-design/TASK.ru.md`
- `plans/active/update-internal-adr-for-current-model-catalog-design/PLAN.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/*`

Проверки:

- target ADR drift понятен
- boundaries rewrite сформулированы

### Slice 2. Обновить ADR

Файлы:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

Проверки:

- ADR отражает current design bundle
- ADR не течёт в virtualization-specific details
- narrative остаётся архитектурным, а не implementation dump

### Slice 3. Зафиксировать review

Файлы:

- `plans/active/update-internal-adr-for-current-model-catalog-design/REVIEW.ru.md`

Проверки:

- перечислены major shifts относительно старого ADR
- отмечены residual risks и follow-up

## 3. Rollback point

Безопасная точка остановки: новый target structure ADR согласован, но сам внешний
документ ещё не переписан.

## 4. Final validation

- read-through updated ADR
- review against current design bundle
- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check`
