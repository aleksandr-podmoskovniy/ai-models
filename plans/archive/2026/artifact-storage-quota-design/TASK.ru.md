# Artifact storage capacity planning and reservations

## 1. Заголовок

Реализовать первый controller-owned storage planning slice для общего model
artifact storage: глобальная capacity, usage visibility и upload reservation.

## 2. Контекст

`ai-models` хранит опубликованные OCI `ModelPack` artifacts, upload staging и
optional source mirror в общем S3-compatible storage. Design уже зафиксировал,
что namespace quotas будут отдельным слоем. Сейчас нужен меньший production
slice: модуль должен знать общий настроенный предел storage, показывать
used/reserved/available и не принимать новый upload, если для него нет места.

Kubernetes `ResourceQuota` покрывает Kubernetes API resources и PVC requests,
но не считает bytes внутри S3/DMCR prefix. Поэтому даже глобальный storage
guardrail должен быть controller-owned: через durable accounting Secret,
reservation до upload и low-cardinality metrics для capacity UX.

## 3. Постановка задачи

Сделать минимальную реализацию без namespace quotas:

- user-facing `artifacts.capacityLimit`;
- internal durable storage accounting ledger;
- upload probe reservation по declared/probed `sizeBytes`;
- понятный отказ `InsufficientStorage` до multipart upload;
- metrics для configured/used/reserved/available bytes.

## 4. Scope

- `openapi/*` и templates для `artifacts.capacityLimit`.
- `domain/storagecapacity` для pure accounting math.
- `adapters/k8s/storageaccounting` для internal Secret-backed ledger.
- upload-gateway reservation/release на probe/abort/expire/fail/complete.
- controller-side published artifact commit/release.
- storage metrics collector.

## 5. Non-goals

- Не реализовывать per-namespace quota policy.
- Не добавлять `ModelStorageUsage` / `ClusterModelStorageQuota` CRD.
- Не менять `Model` / `ClusterModel.spec`.
- Не полагаться на S3/RGW/MinIO per-prefix quotas как на обязательный backend
  contract.
- Не считать runtime node-cache bytes tenant quota: это derived local cache,
  а не owned catalog storage.
- Не делать точный physical object-store scanner; ledger остаётся hot-path
  source of truth.

## 6. Затрагиваемые области

- `openapi/config-values.yaml`, `openapi/values.yaml`, templates.
- `images/controller/internal/domain/storagecapacity`.
- `images/controller/internal/adapters/k8s/*` для CRD/status/RBAC shaping.
- `images/controller/internal/dataplane/uploadsession`.
- `images/controller/internal/controllers/catalogstatus`,
  `catalogcleanup`.
- `images/controller/internal/monitoring/*`.
- `docs/development/*`.

## 7. Критерии приёмки

- При `artifacts.capacityLimit` модуль публикует metrics
  limit/used/reserved/available/capacityKnown.
- Upload probe с `sizeBytes` резервирует bytes до multipart init.
- Concurrent upload probes не могут пройти сверх global limit.
- Upload probe без `sizeBytes` при включённом capacityLimit получает понятную
  ошибку до multipart.
- Abort/expire/fail/completed upload release reservation.
- Ready artifact commit увеличивает committed published bytes.
- Delete cleanup release committed published bytes.
- RBAC не выдаёт людям доступ к internal accounting Secret.
- Изменение проходит targeted tests и `make verify`.

## 8. Риски

- Ledger может разойтись с физическим object store до будущего reconciliation
  scanner.
- HF/mirror reservation до remote HEAD остаётся следующим slice.
- Без configured `artifacts.capacityLimit` enforcement отключён, metrics
  показывают only known used/reserved.
