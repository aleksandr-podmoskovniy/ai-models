# PLAN

## 1. Current phase

Это phase-2 architecture/design slice для `Model` / `ClusterModel`.

Orchestration mode: `full`.

Причина:

- задача меняет public status shape и controller/runtime boundaries;
- задача затрагивает auth, storage и runtime delivery semantics;
- нужен read-only cross-check по API и integration границам до любых будущих
  code changes.

Read-only subagents:

- `api_designer`
  - проверить конкретный `status` shape и condition/lifecycle boundary;
- `integration_architect`
  - проверить auth/storage/runtime delivery boundary для OCI vs S3.

## 2. Slices

### Slice 1. Capture current mismatch

Цель:

- собрать текущее состояние `api/`, CRD и controller contracts;
- зафиксировать, где shape уже расходится с ADR direction.

Файлы:

- read-only `api/core/v1alpha1/*`
- read-only `crds/*`
- read-only `images/controller/internal/*`

Проверки:

- различия между public status и internal contracts понятны.

### Slice 2. Define target shape

Цель:

- оформить concrete target shape для public status и internal delivery
  contracts.

Файлы:

- `plans/active/define-model-status-and-runtime-delivery-target-shape/DECISIONS.ru.md`

Проверки:

- shape годится как source of truth для следующего implementation slice;
- public vs internal boundary прозрачна;
- OCI vs S3 auth path различены.

### Slice 3. Review gate

Цель:

- зафиксировать residual risks и implementation follow-ups.

Файлы:

- `plans/active/define-model-status-and-runtime-delivery-target-shape/REVIEW.ru.md`

Проверки:

- `git diff --check -- plans/active/define-model-status-and-runtime-delivery-target-shape/*`

## 3. Rollback point

Безопасная точка остановки: bundle создан, но target shape ещё не зафиксирован.

## 4. Final validation

- `git diff --check -- plans/active/define-model-status-and-runtime-delivery-target-shape/*`
