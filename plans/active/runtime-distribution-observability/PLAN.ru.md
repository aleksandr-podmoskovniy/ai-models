## 1. Current phase

Этап 2. Public API не меняется. Это bounded observability continuation поверх
уже landed publication/runtime baseline.

## 2. Orchestration

`solo`

Причина:

- slice узкий и не требует нового architecture fork;
- меняются monitoring/bootstrap/docs surfaces, но ownership seam понятен;
- read-only delegation здесь даст меньше сигнала, чем прямой bounded
  implementation.

## 3. Slices

### Slice 1. Зафиксировать compact observability bundle

Цель:

- открыть canonical active bundle под первый runtime observability cut без
  sibling drift относительно `phase2-model-distribution-architecture`.

Файлы/каталоги:

- `plans/active/runtime-distribution-observability/*`
- `plans/active/phase2-model-distribution-architecture/*`

Проверки:

- manual consistency review

Артефакт результата:

- compact executable bundle для первого runtime observability slice.

### Slice 2. Реализовать collector для managed node-cache runtime plane

Цель:

- дать module-owned Prometheus signal по здоровью stable per-node runtime
  Pod/PVC plane без смешения с public catalog metrics;
- показать operator-facing drift между desired selected nodes и реально
  managed/ready runtime resources.

Файлы/каталоги:

- `images/controller/internal/monitoring/*`
- `images/controller/internal/bootstrap/*`

Проверки:

- `cd images/controller && go test ./internal/monitoring/... ./internal/bootstrap`

Артефакт результата:

- отдельный collector по managed `node-cache-runtime` `Pod` / `PVC`;
- selector-scoped summary metrics по desired/managed/ready runtime plane;
- bootstrap wiring без нового mixed monitoring shell.

### Slice 3. Синхронизировать docs и evidence

Цель:

- зафиксировать новый live observability seam в current controller docs.

Файлы/каталоги:

- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/monitoring/...`

Артефакт результата:

- docs/evidence перечисляют runtime health collector как current live boundary.

### Slice 4. Repo-level validation

Цель:

- подтвердить, что observability slice не внёс hidden drift.

Файлы/каталоги:

- all touched surfaces in this bundle

Проверки:

- `make verify`
- `git diff --check`

Артефакт результата:

- green repo-level guards after bounded observability cut.

## 4. Rollback point

После Slice 2: можно остановиться на working collector and bootstrap wiring,
ещё не считая docs/evidence finalised.

## 5. Final validation

- `cd images/controller && go test ./internal/monitoring/... ./internal/bootstrap`
- `make verify`
- `git diff --check`
- `review-gate`
