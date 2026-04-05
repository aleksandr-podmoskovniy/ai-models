# PLAN

## 1. Current phase

Это phase-2 architecture analysis slice вокруг artifact delivery и auth
patterns для `Model` / `ClusterModel`.

Orchestration mode: `light`.

Причина:

- вопрос затрагивает auth, storage, runtime boundary и public contract;
- в этом slice не будет кодовых изменений;
- достаточно одного read-only subagent по integration/auth/storage risk.

Read-only subagents:

- `integration_architect`
  - проверить recommended boundary между RBAC-style OCI access и S3 credential
    issuance.

## 2. Slices

### Slice 1. Collect references

Цель:

- собрать локальные референсы по virtualization и payload-registry;
- собрать внешние official references по accepted patterns.

Файлы:

- read-only local docs/code
- bundle notes if needed

Проверки:

- источники покрывают OCI и S3 стороны.

### Slice 2. Synthesize recommendation

Цель:

- сформулировать recommended access model для OCI и S3 delivery classes;
- отделить public contract от internal implementation detail.

Файлы:

- `plans/active/analyze-model-artifact-access-patterns/REVIEW.ru.md`

Проверки:

- вывод связан с ADR direction;
- вывод не тащит raw backend entities в public API.

## 3. Rollback point

Безопасная точка остановки: bundle создан, но рекомендации ещё не зафиксированы.

## 4. Final validation

- `git diff --check -- plans/active/analyze-model-artifact-access-patterns/*`
