# PLAN

## 1. Current phase

Задача относится к phase 2: public catalog API и controller lifecycle поверх
already-running managed backend.

Orchestration mode: `full`.

Причина:

- меняется public artifact contract;
- меняется managed backend / distribution boundary;
- появляется deletion cleanup semantics for `Model` / `ClusterModel`;
- нужен read-only review по API и live cleanup boundary до production-code edits
  и финальный reviewer pass после реализации.

Read-only subagents:

- `api_designer`
  - проверить, как generalized artifact location и cleanup intent должны
    выглядеть в public API без raw backend leaks;
- `integration_architect`
  - проверить backend-specific artifact location, live cleanup feasibility и
    relation to current MLflow/S3 baseline.

Предварительные assumptions до delegation:

- public API всё ещё не должен нести raw MLflow entities;
- deletion cleanup semantics должны быть видны как object lifecycle concern, а
  не как side helper command;
- MLflow/S3 cleanup можно закрыть first-class раньше OCI delete path;
- always-local materialization остаётся runtime goal, но сам materializer не
  обязательно реализуется в этом slice.

Выводы delegation, которые фиксируем до реализации:

- public artifact status должен перейти на generalized locator:
  - `kind` (`OCI` | `S3`);
  - `uri`;
  - optional `digest`, `mediaType`, `sizeBytes`;
- public status не должен нести `backendKind`, `workspace`, `runID`,
  `loggedModelID`, `secretRef`, `localPath`;
- delete cleanup semantics должны жить через object lifecycle:
  - finalizer;
  - `phase=Deleting`;
  - stable conditions/reasons;
- live cleanup не может опираться только на public artifact locator:
  для MLflow нужен internal cleanup handle с backend-specific данными;
- cleanup executors лучше строить по artifact location kind (`S3`, `OCI`), а не
  по raw backend entities;
- narrowest working slice для user-requested functionality:
  - generalized artifact location;
  - internal cleanup handle;
  - minimal delete-only controller path with finalizer;
  - first live cleanup path for MLflow/S3;
  - OCI cleanup remains planned/not implemented in this slice.

## 2. Slices

### Slice 1. Зафиксировать bundle и boundary shift

Цель:

- оформить смену artifact/distribution цели как bounded task.

Файлы:

- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/TASK.ru.md`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/PLAN.ru.md`

Проверки:

- согласованность с `AGENTS.md`
- согласованность с current repo phase boundaries

Артефакт:

- bundle под новый cleanup + backend-specific artifact slice.

### Slice 2. Обновить API и controller contracts

Цель:

- перейти от OCI-only artifact model к generalized artifact location и
  подготовить delete cleanup contract.

Файлы:

- `api/core/v1alpha1/*`
- `api/scripts/*`, если нужен codegen
- `images/controller/internal/publication/*`
- `images/controller/internal/managedbackend/*`
- `images/controller/internal/runtimedelivery/*`
- `images/controller/internal/cleanup*`, если появятся

Проверки:

- `go generate ./...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `api`
- `go test ./...` в `images/controller`

Артефакт:

- generalized artifact location API/contract layer.

### Slice 3. Реализовать first working cleanup path

Цель:

- добавить executable cleanup-oriented implementation для backend-specific
  artifact locations, начиная с MLflow/S3.

Файлы:

- `images/controller/internal/*`
- `images/controller/cmd/*`, если потребуется runtime wiring
- `images/controller/README.md`
- возможно `images/controller/go.mod`

Проверки:

- `go test ./...` в `images/controller`

Артефакт:

- tested delete-only finalizer/controller path plus MLflow/S3 cleanup baseline.

### Slice 4. Закрыть review gate

Цель:

- убедиться, что artifact contract не развалил public API и cleanup semantics не
  размазаны по repo.

Файлы:

- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/REVIEW.ru.md`

Проверки:

- `make fmt`
- `make test`
- `make verify`
- `git diff --check`

Артефакт:

- финальный review bundle с findings, residual risks и next step.

## 3. Rollback point

Безопасная точка остановки: generalized artifact location и cleanup planning
contracts готовы, но live cleanup executor/finalizer path ещё не провязан.

## 4. Final validation

- `go generate ./...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `api`
- `go test ./...` в `images/controller`
- `make fmt`
- `make test`
- `make verify`
- `git diff --check`
