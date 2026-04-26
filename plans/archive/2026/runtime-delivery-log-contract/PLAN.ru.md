## 1. Current phase

Этап 1. Это corrective observability/logging slice внутри уже рабочего
publication/runtime baseline.

## 2. Orchestration

`solo`

Причина:

- меняется один узкий controller boundary;
- задача про log/event contract, а не про новый runtime/API design;
- основная сложность в точной semantic dedupe, а не в multi-area architecture.

## 3. Slices

### Slice 1. Зафиксировать текущий log-spam path

Цель:

- понять, где именно repeated signal рождается и какие поля реально меняются.

Файлы/каталоги:

- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`

Проверки:

- code inspection
- existing tests around `ApplyToPodTemplate`

Артефакт результата:

- зафиксирован exact reconciliation/logging path for repeated apply signal.

### Slice 2. Ввести semantic delivery-signal dedupe

Цель:

- оставить `info`/event только для meaningful workload-facing state transition.

Файлы/каталоги:

- `images/controller/internal/controllers/workloaddelivery/*`

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`

Артефакт результата:

- controller log/event contract no longer spams identical apply entries.

### Slice 3. Зафиксировать regression tests

Цель:

- доказать, что unchanged delivery contract no longer emits repeated signal.

Файлы/каталоги:

- `images/controller/internal/controllers/workloaddelivery/*`
- при необходимости `images/controller/internal/adapters/k8s/modeldelivery/*`

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery`

Артефакт результата:

- focused tests cover repeated reconcile and meaningful change paths.

## 4. Rollback point

После Slice 1: можно остановиться с точным diagnosis без изменения runtime
semantics.

## 5. Final validation

- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery`
- `git diff --check`
- `review-gate`
