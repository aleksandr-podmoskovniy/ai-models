## 1. Current phase

Этап 1, corrective hardening и quality-gate cleanup внутри publication baseline.

## 2. Orchestration

`solo`

Причина:

- задача технически локальна и механическая по сути;
- архитектурная граница уже ясна, нужен именно surgical refactor под
  quality gate;
- delegation здесь замедлит работу сильнее, чем добавит сигнала.

## 3. Slices

### Slice 1. Разрезать direct-upload transport helpers

Цель:

- убрать лишнюю complexity из двух `oci` функций без изменения
  resume/recovery/checkpoint semantics.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`

Проверки:

- `go test ./internal/adapters/modelpack/oci` в `images/controller/`

Артефакт результата:

- прямой upload path проходит tests и не валит complexity gate.

### Slice 2. Разрезать sourceworker GetOrCreate

Цель:

- вынести из `GetOrCreate` независимые decision steps и сохранить поведение.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/sourceworker/*`

Проверки:

- `go test ./internal/adapters/k8s/sourceworker` в `images/controller/`

Артефакт результата:

- sourceworker lifecycle остаётся тем же, complexity ниже порога.

### Slice 3. Прогнать repo-level verify

Цель:

- подтвердить, что quality gate закрыт целиком, а не только локально.

Файлы/каталоги:

- без дополнительного scope кроме уже затронутого кода

Проверки:

- `make verify`

Артефакт результата:

- зелёный репозиторный verify run.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: direct-upload cleanup уже
изолирован, sourceworker ещё не тронут.

## 5. Final validation

- `gofmt -w` на изменённых Go-файлах
- `go test ./internal/adapters/modelpack/oci ./internal/adapters/k8s/sourceworker` в `images/controller/`
- `make verify`
