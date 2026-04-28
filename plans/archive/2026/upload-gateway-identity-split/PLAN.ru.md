# Upload gateway identity split

## 1. Current phase

Этап 1 / publication-runtime baseline hardening.

## 2. Orchestration

`solo`: slice ограничен Helm templates/RBAC wiring и не меняет public API,
runtime binary или upload HTTP contract. Сабагенты не вызываются в этом turn,
потому что текущий запрос не просит delegation, а изменение делается как
один template/RBAC slice.

## Active bundle disposition

- `model-metadata-contract`: keep. Отдельный workstream про metadata/capability
  contract для будущего `ai-inference`.
- `live-e2e-ha-validation`: keep. Live e2e должен продолжиться после deploy
  исправлений.
- `upload-gateway-identity-split`: current.

## 3. Slices

### Slice 1. Template split

Цель: вынести upload gateway из controller Deployment в отдельный workload.

Файлы:
- `templates/_helpers.tpl`
- `templates/controller/deployment.yaml`
- `templates/controller/service.yaml`
- `templates/upload-gateway/*`

Проверки:
- `make helm-template`

Артефакт:
- render содержит отдельный `ai-models-upload-gateway` Deployment/Service.

### Slice 2. RBAC least privilege

Цель: upload gateway использует отдельный ServiceAccount и namespaced Role.

Файлы:
- `templates/upload-gateway/serviceaccount.yaml`
- `templates/upload-gateway/rbac.yaml`

Проверки:
- `make helm-template`
- `make kubeconform`, если render доступен.

Артефакт:
- upload gateway не bound к controller ClusterRole.

## 4. Rollback point

Откатить новые `templates/upload-gateway/*` и вернуть sidecar/port/backend в
controller templates. Persisted state и CRD не меняются.

## 5. Final validation

- `make helm-template`
- `make kubeconform`
- `review-gate` по template/RBAC diff.

## 6. Execution evidence

- `make helm-template` — passed.
- `make kubeconform` — passed.
- `go test -count=1 ./cmd/ai-models-controller` — passed.
- `go test -count=1 ./cmd/ai-models-controller ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery` — passed.
- `make verify` — passed.
- Render spot-check:
  - `ai-models-controller` Deployment containers: `controller`,
    `kube-rbac-proxy`; no `upload-gateway`.
  - `ai-models-controller` Service ports: `https-metrics`, `webhook`; no
    `upload`.
  - `ai-models-upload-gateway` has separate ServiceAccount, Role, RoleBinding,
    Service, Deployment and Ingress.
  - Upload gateway Role: `secrets get/update` only in module namespace.
