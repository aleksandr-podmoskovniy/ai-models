# PLAN

## 1. Current phase

Задача относится к phase 2: развитие public DKP API каталога моделей поверх
existing phase-1 backend.

Orchestration mode: `light`.

Причина:
- меняется public API contract;
- задача затрагивает validation/defaulting/immutability для `Model` /
  `ClusterModel`;
- нужен read-only review по API semantics до изменения schema markers.

Read-only subagent:
- `api_designer`
  - проверить:
    - какие defaults безопасно закрепить уже сейчас;
    - где нужны `XValidation` для one-of и immutability;
    - нужно ли делать stricter access validation для `ClusterModel` на
      schema level.

## 2. Slices

### Slice 1. Зафиксировать validation/defaulting scope

Цель:
- перевести следующий phase-2 шаг в проверяемый implementation bundle.

Файлы:
- `plans/active/implement-model-catalog-api-validation/TASK.ru.md`
- `plans/active/implement-model-catalog-api-validation/PLAN.ru.md`

Проверки:
- согласованность с `AGENTS.md`
- согласованность с design bundle

Артефакт:
- bundle для schema-level API refinement.

### Slice 2. Добавить validation/defaulting/immutability markers

Цель:
- зафиксировать initial API semantics прямо в schema markers.

Файлы:
- `api/core/v1alpha1/*`

Проверки:
- `go test ./...` из `api/`
- локальная CRD schema generation check

Артефакт:
- public API types с one-of validation, requiredness, defaults и
  artifact-producing immutability.

### Slice 3. Добавить verify path и закрыть review gate

Цель:
- сделать schema checks воспроизводимыми и зафиксировать residual risks.

Файлы:
- `api/scripts/*`
- `api/README.md`
- `plans/active/implement-model-catalog-api-validation/REVIEW.ru.md`

Проверки:
- `make fmt`
- `go generate ./...` в `api`
- `go test ./...` в `api`
- repo-local CRD verification command
- `git diff --check`

Артефакт:
- review bundle и reproducible verification workflow.

## 3. Rollback point

Безопасная точка остановки: после marker-only изменений и verify script в
`api/`, без подключения runtime/controller/templates. В худшем случае этот
slice можно удалить без влияния на phase-1 runtime.

## 4. Final validation

- `make fmt`
- `go generate ./...` в `api`
- `go test ./...` в `api`
- repo-local CRD verification command
- `git diff --check`
- согласованность с:
  - `docs/development/PHASES.ru.md`
  - `docs/development/REVIEW_CHECKLIST.ru.md`
  - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md`
