## 1. Current phase

Этап 2. `Model` / `ClusterModel`: меняется сам public API contract и
controller-owned metadata projection.

## 2. Orchestration

`solo`

Задача архитектурная, но request уже конкретный: public `spec` нужно
схлопнуть до source-only surface. В текущем рабочем режиме делаю это одним
bounded refactor без параллельных subagents.

## 3. Slices

### Slice 1. Свести API к source-only spec

- Цель:
  - убрать из public types и CRD все spec-driven metadata/policy поля;
  - оставить только `source.url`, `source.authSecretRef`, `source.upload`.
- Файлы:
  - `api/core/v1alpha1/*`
  - `api/core/*`
  - `api/scripts/*`
  - `crds/*`
- Проверки:
  - `cd api && go test ./...`
  - `cd api && ./scripts/update-codegen.sh`
  - `cd api && ./scripts/verify-crdgen.sh`
- Артефакт:
  - generated API/CRD без старых spec fields.

### Slice 2. Убрать controller/runtime зависимость от старого spec

- Цель:
  - убрать policy validation и runtime/publication planning, завязанные на
    удалённые spec fields;
  - выровнять upload-session contract без declared format / expected size из
    public spec;
  - перевести input-format/task resolution на calculated path.
- Файлы:
  - `images/controller/internal/application/publishplan/*`
  - `images/controller/internal/application/publishobserve/*`
  - `images/controller/internal/domain/ingestadmission/*`
  - `images/controller/internal/domain/publishstate/*`
  - `images/controller/internal/controllers/catalogstatus/*`
  - `images/controller/internal/adapters/k8s/sourceworker/*`
  - `images/controller/internal/adapters/k8s/uploadsession/*`
  - `images/controller/internal/adapters/k8s/uploadsessionstate/*`
  - `images/controller/internal/dataplane/publishworker/*`
  - `images/controller/internal/dataplane/uploadsession/*`
  - `images/controller/internal/adapters/modelprofile/*`
  - `images/controller/internal/support/testkit/*`
- Проверки:
  - `cd images/controller && go test ./internal/application/publishplan ./internal/application/publishobserve ./internal/domain/ingestadmission ./internal/domain/publishstate ./internal/controllers/catalogstatus ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/publishworker ./internal/dataplane/uploadsession ./internal/adapters/modelprofile/... ./internal/support/testkit`
- Артефакт:
  - publish flow больше не ждёт removed public knobs.

### Slice 3. Синхронизировать docs и evidence

- Цель:
  - убрать из docs/examples старый heavy spec;
  - описать metadata как calculated `status.resolved` contract.
- Файлы:
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
- Проверки:
  - `rg -n "inputFormat|runtimeHints|modelType|usagePolicy|launchPolicy|optimization|expectedSizeBytes|declaredInputFormat" docs images/controller api/core/v1alpha1`
- Артефакт:
  - repo docs and ADR explain the same source-only public contract as live code.

## 4. Rollback point

После Slice 1 можно безопасно остановиться только если controller/runtime ещё
не начал ссылаться на удалённые поля. После начала Slice 2 API and runtime
must land together.

## 5. Final validation

- `cd api && go test ./...`
- `cd api && ./scripts/verify-crdgen.sh`
- `cd images/controller && go test ./...`
- `make verify`
