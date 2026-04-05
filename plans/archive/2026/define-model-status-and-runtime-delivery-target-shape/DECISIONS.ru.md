# DECISIONS

## 1. Current mismatch

Текущее состояние расходится с целевым направлением сразу в нескольких местах.

### Public status

- В `api/core/v1alpha1/types.go` public `status` пока держит:
  - `source`;
  - `upload`;
  - `artifact`;
  - `metadata`.
- Но он не содержит нормального machine-readable technical profile из ADR.
- Одновременно enum `ModelArtifactLocationKind=S3` слишком конкретно протаскивает
  transport/backend detail в public API.

### Public conditions / phase

- Сейчас в public API есть:
  - `ArtifactStaged`;
  - `AccessConfigured`;
  - `BackendSynchronized`;
  - phase `Syncing`.
- Это уже не platform UX, а внутренняя оркестрация контроллера.

### Internal runtime delivery

- В `images/controller/internal/runtimedelivery/plan.go` весь auth path сводится
  к одному флагу `RequiresCredentialProjection`.
- Это слишком грубо: OCI/payload-registry и S3-compatible delivery требуют
  разных access modes.

## 2. Target Public Status Shape

Целевой public `status` должен описывать только:

- где лежит опубликованный артефакт;
- какой технический профиль у модели;
- в каком lifecycle state она находится;
- нужен ли сейчас user upload;
- готова ли модель к использованию.

### Proposed shape

```go
type ModelStatus struct {
    ObservedGeneration int64                  `json:"observedGeneration,omitempty"`
    Phase              ModelPhase             `json:"phase,omitempty"`
    Source             *ResolvedSourceStatus  `json:"source,omitempty"`
    Upload             *ModelUploadStatus     `json:"upload,omitempty"`
    Artifact           *ModelArtifactStatus   `json:"artifact,omitempty"`
    Resolved           *ModelResolvedStatus   `json:"resolved,omitempty"`
    Conditions         []metav1.Condition     `json:"conditions,omitempty"`
}
```

### `ResolvedSourceStatus`

```go
type ResolvedSourceStatus struct {
    ResolvedType     ModelSourceType `json:"resolvedType,omitempty"`
    ResolvedRevision string          `json:"resolvedRevision,omitempty"`
}
```

Смысл:

- блок остаётся небольшим;
- он нужен только чтобы показать, какую revision/import target реально взял
  controller.

### `ModelUploadStatus`

```go
type ModelUploadStatus struct {
    ExpiresAt  *metav1.Time `json:"expiresAt,omitempty"`
    Repository string       `json:"repository,omitempty"`
    Command    string       `json:"command,omitempty"`
}
```

Смысл:

- это public UX только для `spec.source.type=Upload`;
- этого достаточно, чтобы сделать virtualization-like handoff;
- upload credentials и grants сюда не попадают.

### `ModelArtifactStatus`

```go
type ModelArtifactStatus struct {
    Kind      ModelArtifactClass `json:"kind,omitempty"`   // OCI | ObjectStorage
    URI       string             `json:"uri,omitempty"`
    Digest    string             `json:"digest,omitempty"`
    MediaType string             `json:"mediaType,omitempty"`
    SizeBytes *int64             `json:"sizeBytes,omitempty"`
}
```

Решение:

- `S3` как public enum надо заменить на `ObjectStorage`;
- `uri` остаётся стабильным locator;
- credentials, access grants и backend entities сюда не входят.

### `ModelResolvedStatus`

```go
type ModelResolvedStatus struct {
    Task                          string   `json:"task,omitempty"`
    Framework                     string   `json:"framework,omitempty"`
    Family                        string   `json:"family,omitempty"`
    License                       string   `json:"license,omitempty"`
    SourceRepoID                  string   `json:"sourceRepoID,omitempty"`
    Architecture                  string   `json:"architecture,omitempty"`
    Format                        string   `json:"format,omitempty"`
    Quantization                  string   `json:"quantization,omitempty"`
    ParameterCount                *int64   `json:"parameterCount,omitempty"`
    ContextWindowTokens           *int64   `json:"contextWindowTokens,omitempty"`
    SupportedEndpointTypes        []string `json:"supportedEndpointTypes,omitempty"`
    CompatibleRuntimes            []string `json:"compatibleRuntimes,omitempty"`
    CompatibleAcceleratorVendors  []string `json:"compatibleAcceleratorVendors,omitempty"`
    CompatiblePrecisions          []string `json:"compatiblePrecisions,omitempty"`
    MinimumLaunch                 *ModelMinimumLaunch `json:"minimumLaunch,omitempty"`
}

type ModelMinimumLaunch struct {
    PlacementType         string `json:"placementType,omitempty"`
    AcceleratorCount      *int32 `json:"acceleratorCount,omitempty"`
    AcceleratorMemoryGiB  *int32 `json:"acceleratorMemoryGiB,omitempty"`
    SharingMode           string `json:"sharingMode,omitempty"`
}
```

Смысл:

- это один machine-readable block для planner у `ai-inference`;
- он объединяет catalog-style metadata и runtime-relevant technical profile;
- отдельный `status.metadata` в public API больше не нужен.

## 3. Target Public Phases And Conditions

### Phases

Оставить:

- `Pending`
- `WaitForUpload`
- `Publishing`
- `Ready`
- `Failed`
- `Deleting`

Убрать:

- `Syncing`

Причина:

- backend sync, grants и mirror steps должны растворяться внутри `Publishing`,
  а не торчать отдельным user-facing lifecycle state.

### Conditions

Оставить public conditions:

- `Accepted`
- `UploadReady`
- `ArtifactPublished`
- `MetadataReady`
- `Validated`
- `Ready`
- `CleanupCompleted`

Убрать из public contract:

- `ArtifactStaged`
- `AccessConfigured`
- `BackendSynchronized`

Причина:

- это controller-internal orchestration, а не platform UX.

## 4. Target Internal Publication Contract

`images/controller/internal/publication` должен стать источником truth для
controller-side publication result.

### Proposed shape

```go
type Snapshot struct {
    Identity Identity
    Source   SourceProvenance
    Artifact PublishedArtifact
    Resolved ResolvedProfile
    Cleanup  CleanupHandle
    Result   Result
}

type PublishedArtifact struct {
    Class     ArtifactClass // OCI | ObjectStorage
    URI       string
    Digest    string
    MediaType string
    SizeBytes int64
}

type ResolvedProfile struct {
    // mirrors public status.resolved
}

type CleanupHandle struct {
    Class ArtifactClass
    Data  map[string]string
}
```

Смысл:

- `MetadataSnapshot` лучше заменить на `ResolvedProfile`;
- cleanup handle должен быть internal-only и не попадать в public status;
- mirror/backend adapters получают publication result, но не диктуют public
  status shape.

## 5. Target Internal Runtime Delivery Contract

`images/controller/internal/runtimedelivery` должен выдавать не просто
“нужна проекция кредов”, а точный delivery/auth plan.

### Proposed shape

```go
type Plan struct {
    Consumer         ConsumerKind
    Mode             DeliveryMode            // LocalMaterialized
    Artifact         publication.PublishedArtifact
    LocalPath        string
    Materializer     MaterializerKind
    SharedVolumeKind SharedVolumeKind
    Access           AccessPlan
    Verification     VerificationPlan
}

type AccessPlan struct {
    Class        AccessClass // OCIRegistry | ObjectStorage
    Mode         AccessMode  // WorkloadIdentity | DockerConfigSecret | WebIdentitySession | PresignedURL
    TTLSeconds   *int64
    Audience     string
    NeedsCleanup bool
}

type VerificationPlan struct {
    EnforceDigest bool
    Digest        string
}
```

### Access semantics by artifact class

#### OCI / payload-registry

Базовый mode:

- `Class=OCIRegistry`
- `Mode=WorkloadIdentity`

Fallback:

- `Mode=DockerConfigSecret`

Смысл:

- materializer agent использует свой Kubernetes identity;
- controller отдельно materialize'ит repo-scoped registry grant;
- dockerconfigjson допускается только как compatibility layer.

#### ObjectStorage / S3-compatible

Базовый mode:

- `Class=ObjectStorage`
- `Mode=WebIdentitySession`

Fallback:

- `Mode=PresignedURL`

Смысл:

- short-lived credentials только для materializer;
- основной runtime никогда не получает object-storage creds;
- если federation unavailable, controller может выдать ephemeral presigned path.

## 6. What Should Move Out Of Public API

Не должны оставаться в public `status`:

- backend sync markers;
- registry/S3 credentials;
- names of `PayloadRepositoryAccess` or grant objects;
- cleanup handle internals;
- конкретная pod wiring форма materializer.

## 7. Immediate Implementation Follow-ups

Следующий implementation slice должен сделать минимум следующее:

1. В `api/core/v1alpha1/types.go`:
   - заменить `Metadata` на `Resolved`;
   - заменить artifact kind `S3` на `ObjectStorage`;
   - убрать `Syncing`;
   - вычистить internal-only conditions.
2. В `images/controller/internal/publication/*`:
   - заменить `MetadataSnapshot` на `ResolvedProfile`;
   - сделать internal cleanup handle частью publication result.
3. В `images/controller/internal/runtimedelivery/*`:
   - заменить `RequiresCredentialProjection bool` на `AccessPlan` +
     `VerificationPlan`;
   - разделить OCI and ObjectStorage auth modes.
