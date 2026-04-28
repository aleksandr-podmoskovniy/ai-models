# Plan: artifact storage capacity planning and reservations

## 1. Current phase

Задача относится к Phase 1/2 boundary: publication/runtime baseline уже хранит
artifacts в общем storage, но global capacity planning и upload reservation ещё
не стали частью runtime protocol.

## 2. Orchestration

Режим: `solo` для текущего implementation slice.

Причина: текущий запрос не разрешает delegation явно. Slice не добавляет новый
public CRD и intentionally ограничен global capacity guardrail без namespace
quota policy. Перед namespace quota CRD/RBAC нужен `full` режим с read-only
review от `api_designer`, `integration_architect` и `repo_architect`.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep. Это отдельный executable live-check
  workstream.
- `artifact-storage-quota-design` — keep. Это текущий storage capacity
  implementation workstream.
- `diffusers-video-capabilities` — archived to
  `plans/archive/2026/diffusers-video-capabilities`, slice completed.

## 4. Slices

### Slice 1. Config and accounting domain

Status: done.

Цель:

- добавить `artifacts.capacityLimit`;
- сделать pure accounting math для limit/used/reserved/available и
  insufficient-storage decisions.

Файлы:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/deployment.yaml`
- `templates/upload-gateway/deployment.yaml`
- `images/controller/internal/domain/storagecapacity/*`

Проверки:

- `cd images/controller && go test ./internal/domain/storagecapacity`

### Slice 2. Durable ledger adapter

Status: done.

Цель:

- internal Secret-backed ledger with atomic reserve/commit/release.

Файлы:

- `images/controller/internal/adapters/k8s/storageaccounting/*`
- `templates/controller/rbac.yaml`
- `templates/upload-gateway/rbac.yaml`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/storageaccounting`

### Slice 3. Upload reservation

Status: done.

Цель:

- upload probe reserves bytes before multipart init;
- terminal upload paths release reservation.

Файлы:

- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/cmd/ai-models-artifact-runtime/upload_session.go`
- `images/controller/internal/adapters/k8s/uploadsession/*`

Проверки:

- `cd images/controller && go test ./internal/dataplane/uploadsession ./internal/adapters/k8s/uploadsession`

### Slice 4. Published usage and metrics

Status: done.

Цель:

- controller commits/release published artifact bytes;
- metrics expose configured/used/reserved/free capacity.

Файлы:

- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- `images/controller/internal/monitoring/storageusage/*`
- `images/controller/internal/bootstrap/*`

Проверки:

- `cd images/controller && go test ./internal/controllers/catalogstatus ./internal/controllers/catalogcleanup ./internal/monitoring/storageusage ./internal/bootstrap`

### Slice 5. Docs and verification

Status: done.

Цель:

- обновить design/docs и пройти repo verification.

Файлы:

- `plans/active/artifact-storage-quota-design/DESIGN.ru.md`
- `docs/development/*` where relevant

Проверки:

- `git diff --check`
- `make verify`

## 5. Rollback point

Rollback: revert files from this bundle. Runtime state rollback: delete internal
`ai-models-storage-accounting` Secret if the build has been deployed only for
this slice and no newer accounting semantics depend on it.

## 6. Final validation

- `cd images/controller && go test ./...` — passed.
- `cd api && go test ./...` — passed.
- `cd images/dmcr && go test ./...` — passed.
- `make helm-template` — passed.
- `make kubeconform` — passed.
- `make deadcode` — passed.
- `make verify` — passed.
- `review-gate` — passed, residual risks remain in `TASK.ru.md`.
