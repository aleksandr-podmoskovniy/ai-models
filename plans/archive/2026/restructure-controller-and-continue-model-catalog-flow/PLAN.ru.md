# PLAN

## 1. Current phase

Это phase-2 architectural corrective task поверх уже работающего phase-1
backend и частично живого phase-2 controller shell.

Orchestration mode: `full`.

Причина:

- задача меняет controller boundaries, backend integration, auth/access flow и
  runtime delivery contracts;
- меняются сразу несколько каталогов (`api/`, `images/controller/`,
  `templates/controller/`, `plans/`);
- нужно сверить target shape с virtualization-like паттернами до первого кода.

Read-only subagents:

- `repo_architect` / `integration_architect`
  - проверить controller boundaries, worker/result handoff, auth/access path,
    delete ownership.
- `api_designer`
  - проверить, что public `status`, conditions и owner semantics остаются
    DKP-native и backend-neutral.

## 2. Slices

### Slice 1. Capture corrective architecture and refactor target

Цель:

- оформить новый bundle;
- собрать read-only findings по virtualization-паттернам и текущему drift;
- зафиксировать target shape для controller structure.

Файлы:

- `plans/active/restructure-controller-and-continue-model-catalog-flow/*`
- при необходимости related notes/review docs

Проверки:

- bundle отражает ожидаемый user flow и concrete refactor target.

Результат:

- task bundle с ясными boundaries и rollback point до новых кодовых изменений.

### Slice 2. Split controller ownership and durable publication operation contract

Цель:

- выделить отдельный internal publication operation/result boundary;
- оставить `Model` / `ClusterModel` reconciler владельцем только lifecycle
  объекта и public status projection;
- убрать business-state handoff через pod termination log как primary channel.

Файлы:

- `images/controller/internal/modelpublish/*`
- new internal packages for publication operation/result
- `images/controller/internal/app/*`
- `images/controller/cmd/ai-models-controller/*`

Проверки:

- `go test ./...` in `images/controller`

Результат:

- controller structure с явными boundaries lifecycle vs execution.

### Slice 3. Rework HF-first live path on top of new structure

Цель:

- восстановить рабочий `HuggingFace -> managed backend/object storage` path
  уже через новую operation structure;
- держать cleanup handle и public status в корректных owners.

Файлы:

- `images/controller/internal/hfimportjob/*`
- new/updated operation executors
- affected tests

Проверки:

- `go test ./...` in `images/controller`

Результат:

- HF-first path живой и не зависит от monolithic reconciler.

### Slice 4. Prepare auth/access and runtime delivery boundaries for next sources

Цель:

- отделить artifact access contract от main runtime pod;
- подготовить controller-owned secret/session distribution path по смыслу
  virtualization;
- не внедрять ещё full runtime agent, но сделать правильную boundary.

Файлы:

- `images/controller/internal/runtimedelivery/*`
- new auth/access helper packages if needed
- minimal docs sync

Проверки:

- `go test ./...` in `images/controller`

Результат:

- понятный internal contract для future materializer / agent path.

### Slice 5. Wire and validate module runtime

Цель:

- сохранить HA/metrics/leader election/RBAC wiring после refactor;
- обновить docs и module shell при необходимости.

Файлы:

- `templates/controller/*`
- `docs/*`
- `images/controller/README.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Результат:

- controller по-прежнему выглядит как полноценный DKP module component.

## 3. Rollback point

Безопасная точка остановки: bundle и read-only findings готовы, код ещё не
изменён.

## 4. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
