# PLAN

## 1. Current phase

Это phase-2 architectural docs slice. Код не меняется; меняется source of truth
для будущего `Model` / `ClusterModel` contract.

Orchestration mode: `light`.

Причина:

- задача архитектурная и затрагивает public contract;
- но реализация в этом slice docs-only;
- достаточно двух read-only subagents по API shape и backend/publication
  boundary.

Read-only subagents:

- `api_designer`
  - проверить, как минимально добавить multi-source ingestion без потери ADR
    ideology;
- `backend_integrator`
  - проверить, как описывать stable artifact reference, не вытаскивая raw
    backend наружу.

## 2. Slices

### Slice 1. Capture refinement boundaries

Цель:

- зафиксировать, что именно меняем в ADR и чего сознательно не делаем.

Файлы:

- `plans/active/refine-restored-adr-for-multi-source-publication/*`

Проверки:

- scope согласован;
- rollback point понятен.

### Slice 2. Refine ADR

Цель:

- аккуратно обновить ADR по multi-source ingestion, published artifact ref,
  metadata enrichment, runtime local-materialization flow и cleanup.

Файлы:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

Проверки:

- смысл документа остаётся связным;
- old ideology не потеряна;
- source types и lifecycle совпадают с уточнённым user flow;
- diff не превращает ADR в source-oriented rewrite.

### Slice 3. Review gate

Цель:

- зафиксировать итоговые решения и residual risks.

Файлы:

- `plans/active/refine-restored-adr-for-multi-source-publication/REVIEW.ru.md`

Проверки:

- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check`

## 3. Rollback point

Безопасная точка остановки: bundle создан, но ADR ещё не изменён.
