# PLAN

## 1. Current phase

Задача относится к phase 2: controller-side реализация public catalog platform
logic поверх уже существующего phase-1 backend.

Orchestration mode: `full`.

Причина:
- меняется layout `images/controller/` и появляется новый executable module;
- задача затрагивает storage/authz boundary через registry path/access logic;
- нужно отдельно проверить repo layout и integration boundaries до первых
  изменений production code;
- после реализации нужен финальный reviewer pass.

Read-only subagents:
- `repo_architect`
  - сверить layout `images/controller/` с паттернами `virtualization` и
    `gpu-control-plane`;
- `integration_architect`
  - проверить boundaries для registry path conventions и access manager без
    premature reconcile/runtime sprawl.

Выводы delegation, которые фиксируем до реализации:
- единый `go.mod` лежит прямо в `images/controller/`;
- `cmd/*` остаётся thin bootstrap shell, а не местом для domain logic;
- `pkg/*` в этом slice не нужен, весь новый код остаётся под `internal/*`;
- минимальная domain boundary для slice:
  - `internal/registrypath` -> deterministic `RepositoryTarget`;
  - `internal/accessplan` -> semantic access expansion for `Model` /
    `ClusterModel`;
  - `internal/payloadregistry` -> payload-registry-specific rendered intents;
- live Role/RoleBinding creation, `PayloadRepositoryTag`, upload lifecycle,
  reconcile/watch wiring и status mutations остаются вне scope.

## 2. Slices

### Slice 1. Зафиксировать implementation bundle и target layout

Цель:
- оформить следующий controller-side step как исполнимый bundle.

Файлы:
- `plans/active/implement-model-catalog-registry-access/TASK.ru.md`
- `plans/active/implement-model-catalog-registry-access/PLAN.ru.md`

Проверки:
- согласованность с `AGENTS.md`
- согласованность с `docs/development/REPO_LAYOUT.ru.md`

Артефакт:
- bundle для `images/controller/` slice.

### Slice 2. Завести controller module и pure libraries

Цель:
- создать bounded controller-side codebase under `images/controller/`.

Файлы:
- `images/controller/go.mod`
- `images/controller/cmd/*`
- `images/controller/internal/*`

Проверки:
- `go test ./...` из `images/controller/`

Артефакт:
- module-local controller scaffold с tested path/access logic.

### Slice 3. Закрыть review gate

Цель:
- проверить, что slice не ушёл в full controller implementation и не смешал
  concerns.

Файлы:
- `plans/active/implement-model-catalog-registry-access/REVIEW.ru.md`

Проверки:
- `make fmt`
- `make test`
- `git diff --check`

Артефакт:
- review bundle с residual risks и следующими шагами.

## 3. Rollback point

Безопасная точка остановки: после появления isolated `images/controller/`
module и unit-tested path/access libraries, но до templates/runtime deployment
integration. В худшем случае новый module можно убрать без влияния на phase-1
runtime.

## 4. Final validation

- `make fmt`
- `go test ./...` в `images/controller/`
- `make test`
- `git diff --check`
- согласованность с:
  - `docs/development/REPO_LAYOUT.ru.md`
  - `plans/active/design-model-catalog-controller-and-publish-architecture/TARGET_ARCHITECTURE.ru.md`
  - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md`
