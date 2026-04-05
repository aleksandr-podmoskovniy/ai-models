# PLAN

## 1. Current phase

Это phase-2 implementation slice для первого live publication path поверх уже
работающего phase-1 backend.

Orchestration mode: `full`.

Причина:

- задача меняет controller runtime, internal backend integration и module shell;
- появляется live reconciler для `Model` / `ClusterModel`;
- меняется runtime/RBAC wiring controller deployment;
- нужно read-only review по API/status ownership и integration boundaries до
  первого кода.

Read-only subagents:

- `api_designer`
  - проверить status/condition ownership и creator semantics у live reconciler.
- `integration_architect`
  - проверить Job/result handoff, managed backend boundary и cluster runtime
    wiring.

Read-only review findings before implementation:

- publish reconciler не трогает delete lifecycle: при `deletionTimestamp` он
  должен выйти и не переписывать `phase=Deleting` или `CleanupCompleted`;
- public status не должен раскрывать MLflow/workspace/run/backend identities;
- cleanup handle можно писать только после успешного publication и только в
  полностью валидном виде;
- нужны аккуратные public reasons для `MetadataReady` и publishing-in-progress,
  без фальшивых failure reasons;
- every failed/succeeded status update должен stamp'ить current
  `observedGeneration` и очищать stale computed state;
- integration review timeout'нулся, но локальный code context уже подтверждает
  ожидаемый implementation path:
  - controller-owned HF import Job;
  - structured result handoff через pod termination message или эквивалентный
    minimal result channel;
  - controller RBAC должен уметь читать publish Job и его pod result;
  - controller deployment должен получить managed backend endpoint и publish
    job runtime wiring.

## 2. Slices

### Slice 1. Capture bundle and current integration boundary

Цель:

- зафиксировать точный live HF scope;
- собрать read-only review по controller/backend integration risk.

Проверки:

- scope и next-step boundaries понятны.

### Slice 2. Add HF import job/result contract

Цель:

- ввести controller-owned Job builder для HF import;
- сделать structured result handoff из backend import runtime в controller.

Файлы:

- `images/backend/scripts/ai-models-backend-hf-import.py`
- new job/result package under `images/controller/internal/`

Проверки:

- `go test ./...` in `images/controller`
- relevant backend smoke via repo-level validation later

### Slice 3. Add live publication reconciler

Цель:

- реализовать reconcile для `Model` / `ClusterModel` с `source=HuggingFace`;
- создавать Job, отслеживать completion/failure, писать public status и cleanup
  handle.

Файлы:

- new reconciler package under `images/controller/internal/`
- `images/controller/internal/app/*`
- affected tests

Проверки:

- `go test ./...` in `images/controller`

### Slice 4. Wire controller runtime in the module shell

Цель:

- убедиться, что controller deployment/RBAC/args уже достаточны для live HF
  path в кластере.

Файлы:

- `images/controller/cmd/ai-models-controller/run.go`
- `templates/controller/*`
- minimal docs sync if needed

Проверки:

- `make helm-template`
- `make kubeconform`

## 3. Rollback point

Безопасная точка остановки: task bundle создан, read-only review собран, код
ещё не изменён.

## 4. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
