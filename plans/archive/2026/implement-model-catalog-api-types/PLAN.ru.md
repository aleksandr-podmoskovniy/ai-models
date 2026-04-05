# PLAN

## 1. Current phase

Задача относится к phase 2: реализация public DKP API каталога моделей поверх
уже существующего phase-1 backend.

Orchestration mode: `light`.

Причина:
- меняется public API contract;
- задача затрагивает `Model` / `ClusterModel`, status/conditions и group/version
  layout;
- нужен read-only review по DKP API boundary до первого изменения production
  кода.

Read-only subagent:
- `api_designer`
  - проверить initial API shape, scope split `Model` / `ClusterModel`,
    spec/status boundary и стартовый versioning choice.

Read-only review findings:
- стартовая group/version: `ai-models.deckhouse.io/v1alpha1`;
- общий `Spec` / `Status` для `Model` и `ClusterModel` допустим, если различия
  остаются в scope и access semantics, а не в дублирующих shape;
- в initial public `status` безопасно включать `observedGeneration`, `phase`,
  `upload`, `artifact`, `metadata`, `conditions`;
- backend mirror details и opaque access-internal hashes лучше не выводить в
  первом implementation slice;
- `spec.access.serviceAccounts` лучше сразу делать structured ref с
  `namespace` и `name`, чтобы `ClusterModel` не получил неявный namespaced-only
  contract.

## 2. Slices

### Slice 1. Подготовить implementation bundle и API baseline

Цель:
- выбрать minimal reproducible layout для `api/` без operator-repo drift.

Файлы:
- `plans/active/implement-model-catalog-api-types/TASK.ru.md`
- `plans/active/implement-model-catalog-api-types/PLAN.ru.md`
- `api/README.md`

Проверки:
- согласованность с `AGENTS.md`
- согласованность с `docs/development/REPO_LAYOUT.ru.md`

Артефакт:
- ясный scoped plan для первого implementation slice.

### Slice 2. Завести versioned public API types

Цель:
- реализовать `Model` / `ClusterModel` и shared public structs под `api/`.

Файлы:
- `api/go.mod`
- `api/core/register.go`
- `api/core/v1alpha1/*`

Проверки:
- `go test ./...` из `api/`
- `go generate ./...` или equivalent repo-local generation command

Артефакт:
- компилируемый public API package с reproducible generated artifacts.

### Slice 3. Закрыть slice review gate

Цель:
- проверить, что initial API scaffold не протекает controller/backend деталями и
  остаётся совместимым с design bundle.

Файлы:
- `plans/active/implement-model-catalog-api-types/REVIEW.ru.md`

Проверки:
- `git diff --check`
- `make fmt`
- `make test`

Артефакт:
- короткий review с residual risks и следующими шагами.

## 3. Rollback point

Безопасная точка остановки: после добавления `api/` module и type definitions,
но до подключения CRD/templates/controller runtime. В худшем случае изменения
можно убрать, не влияя на phase-1 runtime manifests.

## 4. Final validation

- `git diff --check`
- `make fmt`
- `make test`
- согласованность с:
  - `docs/development/TZ.ru.md`
  - `docs/development/PHASES.ru.md`
  - `docs/development/REPO_LAYOUT.ru.md`
  - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md`
