# PLAN

## 1. Current phase

Это phase-2 implementation slice для второго live source path поверх уже
исправленной operation-based architecture controller-а.

Orchestration mode: `full`.

Причина:

- bounded architecture уже выровнена в предыдущем slice, но в этом шаге всё
  равно затрагиваются execution boundary, backend worker и auth secret
  projection;
- есть риск снова смешать controller lifecycle с worker/runtime concerns;
- нужен read-only review по execution boundary и auth/CA handling.

Read-only subagents:

- execution boundary review:
  - не превращаем `modelpublish` обратно в fat reconciler;
  - решаем, нужен ли generalized import contract или source-specific worker
    path.
- auth/CA handling review:
  - проверяем минимально корректный shape для `http.authSecretRef` и
    `caBundle`.

Final review:

- `review-gate`
- read-only reviewer pass

## 2. Slices

### Slice 1. Capture scope and keep Upload out of the path

Цель:

- зафиксировать, что live `HTTP` идёт через existing `publicationoperation`;
- явно ограничить `Upload` как отдельную future task.

Проверки:

- bundle и acceptance criteria зафиксированы.

### Slice 2. Add HTTP-capable publication worker path

Цель:

- обобщить worker runtime так, чтобы он умел принимать `HTTP` source;
- не сломать existing `HuggingFace` path.

Файлы:

- `images/backend/scripts/*`
- `images/controller/internal/*job*`
- `images/controller/internal/publicationoperation/*`

Проверки:

- `go test ./...` in `images/controller`

### Slice 3. Validate status projection and runtime shell assumptions

Цель:

- убедиться, что public reconciler продолжает проецировать success/failure
  одинаково для `HuggingFace` и `HTTP`;
- обновить docs/tests, если поменялись internal names/contracts.

Файлы:

- affected tests under `images/controller/internal/*`
- `images/controller/README.md` if needed

Проверки:

- `go test ./...` in `images/controller`

## 3. Rollback point

Безопасная точка остановки: bundle создан, код ещё не менялся.

## 4. Final validation

- `go test ./...` in `images/controller`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
