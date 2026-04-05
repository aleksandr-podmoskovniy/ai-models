# PLAN

## 1. Current phase

Задача относится к phase 2: controller orchestration и backend mirror поверх уже
работающего phase-1 managed backend.

Orchestration mode: `full`.

Причина:

- меняются controller boundaries around internal backend integration;
- появляется managed-backend abstraction с future backend swap;
- задача затрагивает runtime-delivery model для consumers;
- нужен read-only review по backend/runtime границам до production-code edits и
  финальный reviewer pass после реализации.

Read-only subagents:

- `integration_architect`
  - проверить managed backend abstraction, MLflow-first adapter boundary и
    phase-1/phase-2 split;
- `api_designer`
  - проверить runtime-delivery contract boundary, недопущение credential leaks в
    public API и relation to `Model` / `ClusterModel`.

Выводы delegation, которые уже фиксируем до реализации:

- public `Model` / `ClusterModel` API в этом slice не меняем;
- backend selection живёт только во внутреннем controller config/wiring;
- runtime-delivery model остаётся внутренним controller/runtime contract;
- credentials, secret refs, mount paths, volume names, sidecar/init-container
  details и backend-selection knobs не поднимаем в public status;
- managed backend contract строим вокруг:
  - `PublicationSnapshot`;
  - `BackendMirrorRequest`;
  а не вокруг raw `Run` / `Workspace` / `Logged Model` / `Model Version`;
- MLflow-first adapter должен маппить generic mirror request в internal sync
  plan, но не делать MLflow entities базовыми domain types;
- `local materialization` не объявляется новым canonical contract:
  - `KServe` сохраняет возможность native OCI path;
  - `KubeRay` / `vLLM` / generic runtimes могут использовать local-materialized
    adapter path;
- internal runtime split должен быть таким:
  - `PublishedArtifact` -> immutable published ref/digest/metadata snapshot;
  - `RuntimeDeliveryPlan` -> internal materialization strategy for a runtime pod.

## 2. Slices

### Slice 1. Зафиксировать bundle и target contract model

Цель:

- оформить новый architecture-heavy шаг как bounded task.

Файлы:

- `plans/active/implement-managed-backend-contract-and-runtime-delivery/TASK.ru.md`
- `plans/active/implement-managed-backend-contract-and-runtime-delivery/PLAN.ru.md`

Проверки:

- согласованность с `AGENTS.md`
- согласованность с current design bundle

Артефакт:

- bundle для managed backend contract slice.

### Slice 2. Реализовать controller-side contracts and adapters

Цель:

- добавить managed backend abstraction и unified runtime delivery model без live
  side effects.

Файлы:

- `images/controller/internal/*`
- `images/controller/cmd/*`, если понадобится config plumbing
- `images/controller/README.md`

Проверки:

- `go test ./...` в `images/controller/`

Артефакт:

- tested controller-side contract layer с MLflow-first adapter.

### Slice 3. Закрыть review gate

Цель:

- проверить, что slice не протёк в public API drift и не начал подменять
  canonical publish plane.

Файлы:

- `plans/active/implement-managed-backend-contract-and-runtime-delivery/REVIEW.ru.md`

Проверки:

- `make fmt`
- `make test`
- `git diff --check`

Артефакт:

- review bundle с reviewer findings, residual risks и next step.

## 3. Rollback point

Безопасная точка остановки: после появления controller-side contract layer для
managed backend + runtime delivery plan, но до live backend sync, pod mutation и
worker execution.

## 4. Final validation

- `go test ./...` в `images/controller/`
- `make fmt`
- `make test`
- `git diff --check`
