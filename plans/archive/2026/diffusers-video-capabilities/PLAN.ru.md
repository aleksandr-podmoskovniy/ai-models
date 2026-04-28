# Plan: Diffusers layout and generation capabilities

## 1. Current phase

Phase 1/2 boundary: publication/runtime baseline должен уметь безопасно
публиковать популярные model artifact layouts, но не должен решать runtime
placement/serving за будущий `ai-inference`.

## 2. Orchestration

Режим: `solo` для текущего implementation slice.

Причина: текущий запрос не разрешает delegation явно. Перед более широким
runtime/API compatibility matrix нужен `full` режим с `api_designer`,
`integration_architect` и `repo_architect`.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep, отдельный live validation workstream.
- `artifact-storage-quota-design` — keep, отдельный storage quota design.
- `diffusers-video-capabilities` — keep, текущий executable implementation
  slice.

## 4. Slices

### Slice 1. API enums and public projection

Файлы:

- `api/core/v1alpha1/types.go`
- `images/controller/internal/domain/publishstate/conditions_ready.go`

Проверки:

- `cd api && go test ./...`

### Slice 2. Format detection and source summaries

Файлы:

- `images/controller/internal/adapters/modelformat/*`
- `images/controller/internal/adapters/sourcefetch/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelformat ./internal/adapters/sourcefetch`

### Slice 3. Diffusers profile and publish worker

Файлы:

- `images/controller/internal/adapters/modelprofile/diffusers/*`
- `images/controller/internal/adapters/modelprofile/common/*`
- `images/controller/internal/dataplane/publishworker/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelprofile/... ./internal/dataplane/publishworker`

### Slice 4. Docs and generated CRDs

Файлы:

- `docs/development/MODEL_METADATA_CONTRACT.ru.md`
- `crds/*.yaml`
- generated deepcopy if needed.

Проверки:

- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `git diff --check`

## 5. Rollback point

Rollback: revert files from this bundle. The slice does not require live
cluster changes and does not mutate runtime state.

## 6. Final validation

- Targeted Go tests for changed packages.
- CRD generation verification.
- `git diff --check`.
- `review-gate`.

## 7. Выполнено

- Добавлен public catalog format `Diffusers` и public capability values
  `VideoGeneration`, `AudioGeneration`, `VideoInput`, `VideoOutput`.
- Diffusers layout определяется по root `model_index.json` плюс
  `.safetensors` weights и не смешивается с plain `Safetensors`.
- HF/profile/upload paths читают Diffusers summary без локального выполнения
  Python-кода и fail-closed для `.bin` weights.
- Image/video/audio generation capabilities выводятся только из declared task
  или распознанного Diffusers pipeline class.

Проверки:

- `cd api && go test ./...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./...`
- `cd images/dmcr && go test ./...`
- `make deadcode`
- `make verify`
