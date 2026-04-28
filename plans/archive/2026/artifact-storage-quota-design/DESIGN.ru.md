# Artifact storage quotas and usage visibility

## 1. Вывод

Для `ai-models` нужен собственный controller-owned quota/accounting слой.
Kubernetes `ResourceQuota` оставить как референс UX (`hard`, `used`,
namespace scope), но не использовать как механизм лимита model bytes: наши
байты лежат в DMCR/S3 prefixes, а не в Kubernetes PVC requests.

Правильная модель:

- admin задаёт quota policy;
- controller резервирует bytes до тяжёлого upload/mirror/publish;
- runtime не пишет сверх reservation;
- published/staged/mirrored bytes попадают в durable ledger;
- пользователи читают агрегированный `ModelStorageUsage`;
- админы видят все usage и quota policies;
- GC пересчитывает usage из ledger и фактических artifacts, но hot path не
  зависит от дорогого prefix scan.

Текущий implementation slice deliberately уже: без namespace quotas и без
новых public usage CRD. Он добавляет только cluster/module-wide capacity
planning:

- `aiModels.artifacts.capacityLimit` как optional общий бюджет;
- module-private Secret ledger для `Published` и upload `Reservation`;
- upload-probe admission: при включённом лимите `sizeBytes` обязателен;
- понятный отказ `507 Insufficient Storage`, если reservation не помещается;
- release reservation на abort/expire/failed/completed;
- release published bytes после delete cleanup перед снятием finalizer;
- low-cardinality backend metrics: capacity known, limit, used, reserved,
  available.

Это intentionally не заменяет будущий per-namespace quota слой, но создаёт
правильный durable accounting primitive, на который этот слой потом ляжет.

## 2. Что показали референсы

### Kubernetes

`ResourceQuota` даёт нужную терминологию:

- `hard` — лимит;
- `used` — уже занятый объём;
- namespace — базовая tenant boundary;
- storage quota работает через PVC requests, включая storageClass-specific
  variants.

Ограничение: это не считает S3/registry bytes. Для `ai-models` это UX pattern,
а не backend.

### Virtualization

Полезные паттерны:

- quota failure не маскируется под generic provisioning failure;
- user-facing resource получает condition/reason `QuotaExceeded`;
- controller watches `ResourceQuota` и переочередит только ресурсы, которые
  застряли на quota;
- DVCR capacity alert смотрит на реальный free space и проценты, а не только на
  object status.

Что не переносится напрямую:

- virtualization опирается на PVC/DataVolume и Kubernetes quota admission;
- `ai-models` пишет в общий object storage, поэтому admission должен быть в
  нашем controller/runtime protocol.

### Deckhouse monitoring

Deckhouse предпочитает агрегированные recording rules на понятных уровнях:
pod, namespace, cluster. Для `ai-models` нужны такие же low-cardinality
metrics по namespace/scope/quota policy без per-object cardinality в alerts.

### Registry/object storage

Registry quota полезна как аналог project quota, но не как единственный source
of truth:

- Harbor явно различает quota usage backend и допускает delayed display при
  high concurrency;
- S3-compatible backends различаются: bucket/user quotas есть не везде и не
  дают переносимого per-namespace prefix limit внутри одного bucket;
- Ceph RGW/MinIO primitives можно использовать как дополнительный guardrail на
  bucket/project, но нельзя делать их обязательной частью DKP API.

## 3. Scope ownership

### Namespaced `Model`

`Model` списывает bytes на namespace владельца.

Если два `Model` в одном namespace ссылаются на один и тот же published digest,
tenant quota считает этот digest один раз для namespace. Это защищает от
само-дублирования без unfair penalty.

Если разные namespaces используют один физический digest, namespace quota
считает logical ownership отдельно. Иначе один namespace мог бы бесплатно
ездить на artifact другого namespace без явного shared-catalog contract.

### `ClusterModel`

`ClusterModel` списывается в отдельный cluster-owned bucket, например
`scope=cluster`. Он не должен попадать в quota случайного namespace, где
создан publish worker или где workload его использует.

Workload consumption не списывает storage quota. Storage quota отвечает за
ownership model artifacts, а не за runtime cache copies.

### Node cache

Node-local cache bytes не входят в tenant quota. Это derived runtime cache с
отдельной capacity/eviction policy и отдельными metrics.

## 4. Public API shape

### Policy: cluster-scoped, admin-owned

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ClusterModelStorageQuota
metadata:
  name: team-default
spec:
  namespaceSelector:
    matchLabels:
      ai.deckhouse.io/quota-tier: team
  hard:
    total: 50Gi
    uploadStaging: 20Gi
    sourceMirror: 50Gi
    models: 20
  mode: Enforce
```

Почему так:

- quota меняет общий storage risk, поэтому policy задаёт cluster persona;
- namespace selector повторяет Deckhouse pattern с selector-based scope;
- `total` — главный лимит, `uploadStaging` и `sourceMirror` — guardrails на
  временные хвосты;
- `models` защищает от object-count abuse;
- `mode=Observe` нужен для безопасного rollout до enforcement.

Что не добавлять:

- не добавлять quota поля в `Model.spec`;
- не давать обычному namespace Editor повышать лимит;
- не публиковать backend bucket/prefix в API.

### Usage: namespaced, controller-owned, read-only

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ModelStorageUsage
metadata:
  namespace: team-a
  name: default
status:
  policyRef:
    name: team-default
  hardBytes: 53687091200
  usedBytes: 32212254720
  reservedBytes: 10737418240
  availableBytes: 10737418240
  publishedBytes: 27917287424
  uploadStagingBytes: 2147483648
  sourceMirrorBytes: 2147483648
  modelCount: 7
  lastReconciledAt: "2026-04-28T12:00:00Z"
  conditions:
    - type: WithinQuota
      status: "True"
      reason: Available
```

Почему так:

- users/admins получают один понятный object на namespace;
- status хранит bytes as int64 для метрик и UI без Quantity parsing;
- `reservedBytes` отдельно показывает, почему available уже уменьшился до
  окончания upload;
- `published/uploadStaging/sourceMirror` объясняют, где именно занят объём;
- `conditions` дают UX как в virtualization.

### Cluster usage

Для `ClusterModel` нужен отдельный cluster-scoped status object:

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ClusterModelStorageUsage
metadata:
  name: default
status:
  hardBytes: 1073741824000
  usedBytes: 214748364800
  reservedBytes: 0
  availableBytes: 858993459200
```

### Backend usage for admins

Для админского UX нужен один cluster-scoped aggregate object. Он показывает не
tenant quota, а физическую картину общего backend storage:

```yaml
apiVersion: ai.deckhouse.io/v1alpha1
kind: ModelStorageBackendUsage
metadata:
  name: default
status:
  configuredLimitBytes: 2199023255552
  knownUsedBytes: 912680550400
  physicalPublishedBytes: 751619276800
  uploadStagingBytes: 53687091200
  sourceMirrorBytes: 107374182400
  reservedBytes: 42949672960
  availableBytes: 1286342705152
  capacityKnown: true
  conditions:
    - type: WithinCapacity
      status: "True"
      reason: Available
```

Если backend не умеет отдавать надёжную capacity, `capacityKnown=false`, а
`availableBytes` не публикуется. Это честнее, чем показывать выдуманный
остаток для RGW/S3, где модуль знает только свои ledger bytes.

## 5. Ledger and reservation

Hot-path enforcement должен опираться на durable reservations, а не на
periodic object-store listing.

Минимальные записи ledger:

- owner scope: namespace или cluster;
- owner UID/generation;
- digest или upload session id;
- component: `Published`, `UploadStaging`, `SourceMirror`, `Reservation`;
- bytes;
- state: `Reserved`, `Committed`, `Released`, `Orphaned`;
- expiry for reservations;
- object references для GC/audit.

Reservation flow:

1. controller считает expected bytes до тяжёлой операции;
2. атомарно создаёт reservation;
3. runtime получает reservation id/limit;
4. runtime aborts if stream exceeds reserved bytes;
5. publish success commits actual bytes and releases delta;
6. failure/delete/expiry releases reservation;
7. reconciler periodically rebuilds usage and marks drift.

Это закрывает concurrent uploads: два upload по 40Gi в namespace с 50Gi hard и
0 used не смогут оба стартовать, потому что первый reservation уменьшит
available до 10Gi.

## 6. Enforcement points

### Upload

- upload session creation requires declared expected size or server-side
  bounded max size;
- multipart gateway tracks uploaded bytes and rejects writes past reservation;
- publish commit compares actual artifact bytes with reserved bytes;
- expired upload releases reservation and exposes status reason.

### HuggingFace direct

- source planning performs HEAD/metadata scan for selected files;
- if size cannot be determined, fail closed unless admin configured a bounded
  pessimistic reservation;
- worker gets reservation id and byte limit.

### Mirror

- mirror bytes consume `sourceMirrorBytes` and `total`;
- mirror completion commits actual bytes;
- publish from mirror does not double-count the same raw mirror as published
  artifact unless both retained objects exist.

### Publish / DMCR

- final `ModelPack` size is authoritative for `Published`;
- transient DMCR 503 remains retryable and must not release reservation until
  publish reaches terminal failed/cleanup;
- direct-upload orphan cleanup updates ledger only after cleanup evidence.

### Delete / GC

- delete marks ledger entries `Releasing`;
- usage may show `pendingReleaseBytes` if object removal is delayed;
- bytes leave `usedBytes` only after controller-owned cleanup evidence or a
  reconciliation proof that object is gone.

## 7. RBAC

Legacy user-authz and rbacv2 should follow one rule: users can see usage,
admins can manage quota policies.

Recommended coverage:

- `User`: get/list/watch `models`, `clustermodels`, namespaced
  `modelstorageusages`; no ledger/secrets.
- `PrivilegedUser`: same quota visibility as `User`.
- `Editor`: can create/update/delete `Model`, but cannot raise quota.
- `Admin`: can read namespace usage and events; optional manage namespaced
  override only if product explicitly wants namespace-admin quota delegation.
- `ClusterEditor`: read all usage, cannot change hard limits by default.
- `ClusterAdmin`: manage `ClusterModelStorageQuota`,
  `ClusterModelStorageUsage` visibility and cluster-owned limits.
- `SuperAdmin`: full access, still subject to namespaceSelector/limitNamespaces
  where Deckhouse applies them.

Internal ledger/reservation resources are denied to module-local users.

## 8. Conditions and events

`Model` / `ClusterModel` should expose quota as first-class lifecycle signal:

- `ArtifactResolved=False, reason=QuotaExceeded`;
- message includes requested, used, reserved, hard and available bytes;
- event reason `QuotaExceeded`;
- requeue on quota policy/usage changes.

`ModelStorageUsage` conditions:

- `WithinQuota`;
- `QuotaExceeded`;
- `AccountingStale`;
- `ReservationExpired`;
- `CleanupPending`.

## 9. Metrics and alerts

Metrics:

- `d8_ai_models_storage_quota_hard_bytes{scope,namespace,policy}`;
- `d8_ai_models_storage_quota_used_bytes{scope,namespace,component}`;
- `d8_ai_models_storage_quota_reserved_bytes{scope,namespace}`;
- `d8_ai_models_storage_quota_available_bytes{scope,namespace}`;
- `d8_ai_models_storage_quota_usage_ratio{scope,namespace,policy}`;
- `d8_ai_models_storage_backend_known_used_bytes{component}`;
- `d8_ai_models_storage_backend_available_bytes`;
- `d8_ai_models_storage_accounting_stale{scope,namespace}`;

Alerts:

- namespace quota usage > 80% warning and > 95% critical;
- global artifact storage free < 20% or < fixed floor, mirroring DVCR style;
- accounting stale for more than one reconciliation period;
- expired reservations accumulating.

Keep alert labels low-cardinality: no model name, digest or upload session.

## 10. Hexagonal implementation layout

Target packages:

- `internal/domain/storagequota`: pure quota math, reservations, usage totals,
  decisions and invariants.
- `internal/application/storagequota`: use cases for reserve, commit, release,
  deny/retry and status planning.
- `internal/ports/storageledger`: durable ledger/reservation port.
- `internal/adapters/k8s/storagequota`: CRD/status projection, RBAC-owned
  shaping, policy selection.
- `internal/controllers/storagequota`: reconciles usage/status and stale
  reservations.
- `internal/adapters/k8s/sourceworker` and `uploadsession`: pass reservation
  identity/limits into runtime contracts.
- `internal/dataplane/uploadsession` and `publishworker`: enforce byte limits
  in streams, but do not decide quota policy.
- `internal/monitoring/storagequota`: quota metrics collectors.

Keep out:

- no quota policy in `support/*`;
- no object-store scanning in domain;
- no K8s object shaping in application;
- no public status policy inside dataplane.

## 11. Implementation slices

1. API design slice:
   add `ClusterModelStorageQuota`, `ModelStorageUsage`,
   `ClusterModelStorageUsage`, `ModelStorageBackendUsage`, RBAC fragments and
   OpenAPI docs.

2. Domain/application slice:
   implement quota math, reservation lifecycle and decision table tests.

3. Ledger adapter slice:
   add module-private durable ledger/reservation store and replay tests.

4. Upload/HF admission slice:
   reserve before upload/source fetch, enforce stream byte limits, expose
   `QuotaExceeded` status.

5. Publish/delete accounting slice:
   commit actual artifact bytes, release on failed/expired/delete/GC, handle
   transient DMCR retry without quota loss.

6. Usage/status/metrics slice:
   aggregate usage objects and Prometheus metrics/alerts.

7. RBAC validation slice:
   check legacy access levels and rbacv2 aggregation for read/manage paths and
   intentional deny of ledger/secrets.

## 12. Open decisions

- Whether namespace `Admin` may manage namespace-local quota override. Default:
  no, only cluster policy.
- Whether cross-namespace digest dedupe should ever discount tenant quota.
  Default: no, except future explicit shared catalog.
- How much unknown-size source fetch to allow. Default: fail closed.
- Whether object-store bucket quota should be configured as a coarse global
  guardrail. Default: optional backend-specific hardening, not DKP API.

## 13. Sources

- Kubernetes ResourceQuota docs:
  https://kubernetes.io/docs/concepts/policy/resource-quotas/
- Harbor project quotas:
  https://goharbor.io/docs/edge/administration/configure-project-quotas/
- Ceph Object Gateway quotas:
  https://docs.ceph.com/en/latest/radosgw/admin/#quota-management
- MinIO bucket quota:
  https://docs.min.io/enterprise/aistor-object-store/administration/bucket-quotas/
- Local virtualization reference:
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact/pkg/controller/vd/internal/watcher/resource_quota_watcher.go`
- Local DVCR capacity alert:
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/monitoring/prometheus-rules/dvcr.yaml`
