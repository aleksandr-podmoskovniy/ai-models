# REVIEW

## Findings

### Exact matches

- ADR и текущий CRD совпадают в главной платформенной рамке:
  - публичные объекты — `Model` и `ClusterModel`;
  - `spec` хранит desired state, `status` — computed state;
  - артефакт модели живёт вне etcd;
  - после принятия объекта controller'ом artifact-defining части `spec`
    считаются immutable.
- ADR требует короткий `phase` и `metav1.Condition`; текущий contract это
  сохраняет через `status.phase`, `status.conditions` и
  `status.observedGeneration`.
- ADR требует скрыть internal backend/storage механику от потребителя модели;
  текущий CRD и controller-side contracts это соблюдают: в public status нет
  `MLflow`, `workspace`, `run ID`, cleanup handle или job name.

### Major divergences

- ADR описывает первую итерацию как `spec.artifact.type=OCI` only с
  digest-pinned artifact source в `spec`. Текущий CRD уже ушёл на
  orchestration-oriented source contract:
  - `spec.source={HuggingFace|Upload|OCIArtifact}`
  - `status.artifact={kind,uri,digest,mediaType,sizeBytes}`
- ADR ожидает более inference-oriented API:
  - `spec.modelType`
  - `spec.usagePolicy`
  - `spec.launchPolicy`
  - `spec.optimization.speculativeDecoding`
  - `status.resolved.*`
  В текущем CRD этих блоков нет. Вместо них есть более узкие
  `package`, `publish`, `runtimeHints`, `access`, `metadata`.
- ADR описывает lifecycle как:
  - `ArtifactResolved`
  - `MetadataResolved`
  - `Validated`
  - `Ready`
  Текущий contract использует publication-oriented lifecycle:
  - `Accepted`
  - `UploadReady`
  - `ArtifactStaged`
  - `ArtifactPublished`
  - `MetadataReady`
  - `AccessConfigured`
  - `BackendSynchronized`
  - `CleanupCompleted`
  - `Ready`
- ADR вообще не описывает delete cleanup lifecycle. Текущий controller уже
  вводит finalizer-based cleanup и internal cleanup handle.
- ADR не фиксирует always-local runtime delivery. Текущий controller internals
  уже трактуют local materialization как canonical runtime-delivery path.

## Classification

### Intentional design shifts

- Уход от OCI-only `spec.artifact` к `spec.source` + publish/upload/import
  orchestration contract.
- Backend-neutral `status.artifact.kind/uri/...` вместо OCI-only public shape.
- Always-local runtime delivery как internal controller/runtime contract.
- Delete cleanup semantics как controller-owned lifecycle concern.

Эти сдвиги подтверждаются текущим design bundle:

- [API_CONTRACT.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md)
- [USER_FLOWS.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/design-model-catalog-controller-and-publish-architecture/USER_FLOWS.ru.md)
- [REVIEW.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/REVIEW.ru.md)

### Implementation gaps

- Publish/sync owner ещё не пишет live `status.artifact` end-to-end.
- Publish/sync owner ещё не записывает cleanup handle на published object.
- OCI/internal-backend cleanup path ещё не реализован; live cleanup есть только
  для MLflow-shaped handle.

### Stale or unresolved ADR parts

- Блоки `modelType`, `usagePolicy`, `launchPolicy`,
  `optimization.speculativeDecoding` и `status.resolved.*` не выглядят как
  “ещё не реализовано”. По текущему repo это больше похоже на устаревшую,
  более inference-centric API-модель, которую design bundle уже заменил.

## Residual risks

- Пока ADR не обновлён, он будет создавать ложное ожидание, что первая
  итерация CRD должна быть OCI-only и содержать richer inference policy fields.
- Если следующий implementation slice продолжит текущий design bundle без
  синхронизации ADR, drift станет уже не документным, а process-level.

## Next step

- Либо обновить ADR под текущий phase-2 design bundle.
- Либо явно принять решение, что ADR остаётся источником истины, и тогда
  потребуется отдельный redesign current CRD contract.
