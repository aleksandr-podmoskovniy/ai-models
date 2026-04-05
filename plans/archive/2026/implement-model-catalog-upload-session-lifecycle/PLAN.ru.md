# PLAN

## 1. Current phase

Задача относится к phase 2: controller-side orchestration для public catalog API
поверх phase-1 backend.

Orchestration mode: `full`.

Причина:

- меняется public API status contract;
- задача затрагивает controller ownership boundary вокруг upload lifecycle;
- задача задевает auth/storage boundary через temporary staging grants;
- нужен внешний read-only review до production-code edits и финальный reviewer
  pass после реализации.

Read-only subagents:

- `api_designer`
  - сверить `status.upload`, phase/conditions и virtualization-style UX с
    current API bundle;
- `integration_architect`
  - проверить boundaries для staging grant planning, publisher identity model и
    payload-registry-specific rendering без premature materialization.

Выводы delegation, которые фиксируем до реализации:

- `status.upload` в этом slice доводим до трёх полей:
  - `expiresAt`
  - `repository`
  - `command`
- `status.phase=WaitForUpload` имеет смысл только для `spec.source.type=Upload`;
- минимальные публичные conditions в этом slice:
  - `Accepted=True` / `SpecAccepted`
  - `UploadReady=True` / `WaitingForUserUpload`
  - `UploadReady=False` / `UploadExpired` при истечении TTL;
- publisher upload grant не смешиваем с consumer read-access planning;
- publisher identity model должна поддерживать `User`, `Group`,
  `ServiceAccount`;
- payload-registry-oriented upload intent уже сейчас должен нести stable
  registry namespace для будущей RBAC materialization;
- upload grant ограничиваем exact staging repository path и push-only
  capability;
- helper command остаётся user-facing подсказкой, а не raw registry login
  plumbing.

## 2. Slices

### Slice 1. Зафиксировать upload-session bundle и boundaries

Цель:

- оформить следующий phase-2 step как исполнимый bounded slice.

Файлы:

- `plans/active/implement-model-catalog-upload-session-lifecycle/TASK.ru.md`
- `plans/active/implement-model-catalog-upload-session-lifecycle/PLAN.ru.md`

Проверки:

- согласованность с `AGENTS.md`
- согласованность с design bundle

Артефакт:

- bundle для upload session lifecycle.

### Slice 2. Реализовать public status contract и upload planning libraries

Цель:

- довести `status.upload` до agreed shape;
- добавить pure libraries для upload session planning и staging grant intent.

Файлы:

- `api/core/v1alpha1/*`
- `api/README.md`, `api/scripts/*`, если потребуется verify wiring
- `images/controller/internal/*`
- `images/controller/cmd/*`, если понадобится minimal option plumbing

Проверки:

- `go generate ./...` в `api`
- `go test ./...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `images/controller/`

Артефакт:

- tested upload-session planning slice без live cluster writes.

### Slice 3. Закрыть review gate

Цель:

- проверить, что slice не ушёл в reconcile/runtime implementation и не
  деформировал public contract.

Файлы:

- `plans/active/implement-model-catalog-upload-session-lifecycle/REVIEW.ru.md`

Проверки:

- `make fmt`
- `make test`
- `git diff --check`

Артефакт:

- review bundle с reviewer findings, residual risks и next step.

## 3. Rollback point

Безопасная точка остановки: после появления tested upload-session planning
library и public `status.upload.repository`, но до live RBAC materialization и
upload detection in reconcile loop.

## 4. Final validation

- `go generate ./...` в `api`
- `go test ./...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `images/controller/`
- `make fmt`
- `make test`
- `git diff --check`
