# PLAN

## 1. Current phase

Это phase-2 implementation slice для public API и controller contracts.

Orchestration mode: `full`.

Причина:

- задача меняет больше одного каталога;
- меняется public API/CRD и controller boundaries;
- нужны read-only subagents по API и integration risk до правок.

Read-only subagents:

- `api_designer`
  - быстро проверить proposed `status` rebaseline в `api/`.
- `integration_architect`
  - быстро проверить `publication` / `runtimedelivery` contract split.

## 2. Slices

### Slice 1. Capture current code context

Цель:

- сверить текущие `types.go`, conditions, CRD and controller contracts;
- собрать read-only review по главным рискам.

Проверки:

- mismatch points понятны.

### Slice 2. Rework public API status

Цель:

- обновить `types.go` и `conditions.go`;
- регенерировать CRD.

Файлы:

- `api/core/v1alpha1/types.go`
- `api/core/v1alpha1/conditions.go`
- `crds/*`

Проверки:

- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `go test ./...` in `api`

### Slice 3. Rework internal contracts

Цель:

- обновить `publication` and `runtimedelivery`;
- поправить tests.

Файлы:

- `images/controller/internal/publication/*`
- `images/controller/internal/runtimedelivery/*`
- affected tests

Проверки:

- `go test ./...` in `images/controller`

### Slice 4. Final validation

Цель:

- убедиться, что slice интегрируется с repo-level checks.

Проверки:

- `make fmt`
- `make test`
- `git diff --check`

## 3. Rollback point

Безопасная точка остановки: bundle создан, read-only review собран, код ещё не
изменён.
