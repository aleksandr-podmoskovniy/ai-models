# Node-cache desired-artifacts contract

## 1. Current phase

Этап 1 / runtime baseline hardening.

## 2. Orchestration

`solo`: изменение локализовано в shared node-cache contract и двух adapter
packages; public API, RBAC, templates и runtime entrypoints не меняются.

## Active bundle disposition

- `model-metadata-contract`: keep. Отдельный workstream по metadata/capability
  contract.
- `live-e2e-ha-validation`: keep. Live e2e должен продолжиться после deploy
  исправлений.
- `node-cache-desired-artifacts-contract`: current.

## 3. Slices

### Slice 1. Shared parser

Цель: перенести desired-artifacts annotation parsing в `internal/nodecache`.

Файлы:
- `images/controller/internal/nodecache/desired_artifacts.go`
- `images/controller/internal/nodecache/desired_artifacts_test.go`

Проверки:
- `go test ./internal/nodecache`

Артефакт:
- pure parser из `map[string]string` без Kubernetes imports.

### Slice 2. Adapter decoupling

Цель: убрать import `modeldelivery` из `k8s/nodecacheruntime`, сохранив
совместимость modeldelivery writer constants.

Файлы:
- `images/controller/internal/adapters/k8s/modeldelivery/options.go`
- `images/controller/internal/adapters/k8s/nodecacheruntime/desired_artifacts.go`
- `images/controller/internal/adapters/k8s/nodecacheruntime/desired_artifacts_test.go`

Проверки:
- `go test ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery`

Артефакт:
- nodecacheruntime зависит от `internal/nodecache`, а не от modeldelivery.

## 4. Rollback point

Откатить shared parser и вернуть локальный parser в nodecacheruntime. Persisted
state, CRD и templates не меняются.

## 5. Final validation

- `go test -count=1 ./internal/nodecache ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery`
- `review-gate` по slice diff.

## 6. Execution evidence

- `go test -count=1 ./internal/nodecache ./internal/adapters/k8s/nodecacheruntime ./internal/adapters/k8s/modeldelivery` — passed.
- `make verify` — passed.
