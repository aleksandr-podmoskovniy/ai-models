# Artifact storage quotas and usage visibility

## 1. Заголовок

Спроектировать quota/accounting слой для общего model artifact storage.

## 2. Контекст

`ai-models` хранит опубликованные OCI `ModelPack` artifacts, upload staging и
optional source mirror в общем S3-compatible storage. Сейчас модуль показывает
`status.artifact.sizeBytes` и catalog metrics, но не ограничивает суммарный
объём, который namespace или cluster-owned catalog может занять в общем
storage.

Kubernetes `ResourceQuota` покрывает Kubernetes API resources и PVC requests,
но не считает bytes внутри S3/DMCR prefix. Поэтому лимит вида "namespace не
может загрузить больше 50Gi моделей" должен быть controller-owned quota
contract, а не побочный эффект storage backend.

## 3. Постановка задачи

Сформировать production-grade design для:

- hard quota per namespace;
- отдельного cluster-owned лимита для `ClusterModel`;
- видимости used/reserved/available для пользователей и админов;
- безопасного admission перед upload/HF mirror/publish;
- reconciliation с фактическими artifacts после crash/replay/GC;
- RBAC coverage по Deckhouse access levels.

## 4. Scope

- Сверить подходы Kubernetes `ResourceQuota`, virtualization quota UX,
  Deckhouse monitoring rollups и registry/object-store quota practices.
- Выбрать API shape без раздувания `Model.spec`.
- Описать ledger/reservation source of truth.
- Описать enforcement points для HF, Upload, Mirror, Publish и Delete/GC.
- Описать metrics, conditions, events и пользовательский UX.
- Разложить будущую реализацию на hexagonal boundaries.

## 5. Non-goals

- Не реализовывать CRD/controller в этом slice.
- Не менять `Model` / `ClusterModel` API до отдельного API slice.
- Не полагаться на S3/RGW/MinIO per-prefix quotas как на обязательный backend
  contract.
- Не считать runtime node-cache bytes tenant quota: это derived local cache,
  а не owned catalog storage.

## 6. Затрагиваемые области

- `api/core/v1alpha1` для будущего quota/usage API.
- `images/controller/internal/domain/*` для quota math.
- `images/controller/internal/application/*` для admission/use cases.
- `images/controller/internal/adapters/k8s/*` для CRD/status/RBAC shaping.
- `images/controller/internal/dataplane/*` для upload/runtime byte enforcement.
- `templates/rbacv2/*`, legacy `user-authz` fragments.
- `docs/CONFIGURATION*.md`, `docs/development/*`, monitoring rules.

## 7. Критерии приёмки

- Есть design, который объясняет hard/used/reserved/available semantics.
- Namespace quota не может быть изменена обычным namespace Editor.
- Users видят usage своего namespace без доступа к secrets/internal ledger.
- Concurrent uploads не могут oversubscribe quota за счёт reservation.
- Crash/restart controller, worker, upload-gateway или DMCR не теряет quota
  state и сходится после replay.
- `QuotaExceeded` виден как condition/reason/event, а не как generic failed
  publish.
- ClusterModel не списывает байты на случайный namespace.
- Design не требует vendor-specific S3 quota primitives.

## 8. Риски

- Exact accounting по object storage может быть дорогим при prefix scan.
- Logical tenant quota и physical dedupe могут расходиться.
- Upload без declared size требует stricter protocol или bounded pessimistic
  reservation.
- Слишком богатый API может стать преждевременным public contract.
