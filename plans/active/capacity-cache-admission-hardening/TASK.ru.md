# Capacity and cache admission hardening

## 1. Заголовок

Fail-fast планирование storage и node-cache capacity перед тяжёлой публикацией
и workload delivery.

## 2. Контекст

Live e2e `20260428193910` доказал, что HF Direct, Upload, Diffusers image/video,
controlled interruptions и delete/GC работают после последней выкладки. Но он
оставил production-риск: модуль должен заранее понимать, хватит ли места для
публикации модели и для node-local cache, а не доходить до DMCR/object-store или
CSI mount failure после многогигабайтной загрузки.

В проекте уже есть controller-owned `storagecapacity` ledger и
`artifacts.capacityLimit` для общего artifact storage. Это правильная база:
Kubernetes `ResourceQuota` не считает bytes в S3/DMCR prefix. Нужно довести
эту базу до всех publish paths и добавить похожий fail-fast guardrail для
node-cache без tenant quotas.

## 3. Постановка задачи

Исправить найденные post-e2e дефекты и сделать первый production slice:

- предварительный расчёт publish size для HF Direct/Mirror и Upload;
- reservation/admission отказ до тяжёлого byte path, если общего artifact
  storage не хватает;
- node-cache preflight/admission для SharedDirect: workload не должен получать
  SharedDirect, если cache capacity на целевой ноде не может вместить нужные
  digest artifacts;
- понятные conditions/events/logs для `InsufficientStorage` и
  `InsufficientNodeCache`;
- убрать лишний GC/log noise, где это возможно без смены публичного API;
- зафиксировать forced Mirror test как обязательный следующий live e2e slice.

## 4. Scope

- `images/controller/internal/domain/storagecapacity`
- `images/controller/internal/adapters/k8s/storageaccounting`
- `images/controller/internal/controllers/catalogstatus`
- `images/controller/internal/controllers/workloaddelivery`
- `images/controller/internal/nodecache`
- `images/controller/internal/adapters/k8s/nodecacheruntime`
- `images/controller/internal/dataplane/uploadsession`
- `images/dmcr/internal/garbagecollection`
- `templates/*` only if RBAC/env wiring is needed
- active e2e/runbook notes only as evidence, not as primary design source

## 5. Non-goals

- Не реализовывать namespace quotas.
- Не добавлять новый публичный quota CRD в этом slice.
- Не считать physical object-store scanner source of truth.
- Не пытаться гарантировать точную модель будущего ai-inference scheduler.
- Не включать SharedDirect в кластере без SDS/node-cache readiness evidence.
- Не смешивать DMCR log cleanup с изменением registry GC algorithm.

## 6. Затрагиваемые области

- Controller domain/application/adapters around storage accounting.
- Workload delivery topology and node-cache shared contracts.
- DMCR GC logging result shaping.
- Tests for capacity math, reservation idempotency, workload delivery
  fail-fast and node-cache capacity decisions.
- `plans/active/live-e2e-ha-validation` only for next-run evidence updates.

## 7. Критерии приёмки

- HF Direct/Mirror publication reserves estimated artifact size before worker
  starts raw-layer upload; insufficient capacity returns stable failure reason
  before expensive transfer.
- Upload reservation remains idempotent and rejects missing/too-large size when
  `artifacts.capacityLimit` is configured.
- Published artifact commit/release keeps ledger consistent after Ready/delete.
- SharedDirect is selected only when node-cache runtime is ready and the
  per-node cache budget can hold the requested digest set or an explainable
  eviction plan exists.
- Missing node-cache capacity produces a clear event/condition and falls back
  only if the configured topology allows MaterializeBridge.
- DMCR GC evidence remains structured while giant raw registry output is not
  emitted as the primary operator signal.
- No user-facing RBAC grants are added for storage accounting or node-cache
  internal objects.
- Targeted Go tests pass for changed packages; render/kubeconform pass if
  templates changed.

## 8. Риски

- HF repositories can report incomplete size before file listing is resolved;
  fail-fast must happen after cheap metadata/listing and before byte transfer.
- Ledger can drift from physical object storage until a future reconciliation
  scanner exists.
- Node-cache capacity is local and derived: it must be treated as delivery
  readiness, not as catalog ownership/quota.
- Strict fail-fast can reject a publication that would have succeeded after
  GC; the message must tell the operator to wait for cleanup or increase limit.
