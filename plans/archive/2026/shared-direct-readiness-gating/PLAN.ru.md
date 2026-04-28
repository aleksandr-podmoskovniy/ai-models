# SharedDirect readiness gating

## 1. Current phase

Этап 1 / runtime baseline hardening. Slice закрывает guardrail перед
полноценным live e2e node-cache/shared-direct.

## 2. Orchestration

`solo`: изменение узкое, без public API/RBAC/templates и без нового runtime
entrypoint. Delegation не используется, потому что текущий запрос не просит
сабагентов, а риск локализован в одном adapter boundary.

## Active bundle disposition

- `model-metadata-contract`: keep. Это отдельный workstream про полезную
  metadata/capability surface для будущего `ai-inference`; текущий slice его не
  закрывает.
- `live-e2e-ha-validation`: keep. Это live-test workstream; он должен
  продолжиться после этого guardrail и следующего deploy.
- `shared-direct-readiness-gating`: current. Единственный executable slice в
  этой задаче.

## 3. Slices

### Slice 1. Workload-aware ready node check

Цель: заменить global ready-node check на проверку ready нод против итогового
Pod template.

Файлы:
- `images/controller/internal/adapters/k8s/modeldelivery/service.go`
- новый узкий helper file в `modeldelivery`, если это уменьшит монолитность.

Проверки:
- `go test ./internal/adapters/k8s/modeldelivery`

Артефакт:
- scheduling gate снимается только при наличии совместимой ready node-cache
  ноды.

### Slice 2. Regression coverage

Цель: доказать selector/affinity/taint branches и owner-level reconcile
behavior.

Файлы:
- `images/controller/internal/adapters/k8s/modeldelivery/*_test.go`
- `images/controller/internal/controllers/workloaddelivery/*_test.go`

Проверки:
- `go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`

Артефакт:
- tests фиксируют fail-closed поведение до появления подходящей ready ноды.

## 4. Rollback point

Откатить изменения в `modeldelivery` helper/tests и удалить bundle. Это не
меняет CRD, RBAC, templates или persisted state.

## 5. Final validation

- `go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`
- `review-gate` по diff текущего slice.

## 6. Execution evidence

- `go test ./internal/adapters/k8s/modeldelivery` — passed.
- `go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery` — passed.
- `go test -count=1 ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery` — passed.
- `make verify` — passed after this slice and the upload-gateway identity split.
