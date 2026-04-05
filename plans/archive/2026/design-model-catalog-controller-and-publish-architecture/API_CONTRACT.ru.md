# API CONTRACT

## 1. Общие принципы

- `Model` и `ClusterModel` должны быть semantically aligned.
- `spec` хранит только desired state.
- `status` хранит вычисленное состояние.
- Public API не должен требовать знания:
  - `MLflow`;
  - `PayloadRepositoryTag`;
  - internal registry repo plumbing;
  - controller job names.

## 2. Scope split

### `Model`

Namespaced object.

Используется для:

- моделей, которыми владеет конкретная команда/namespace;
- team-local inference and training consumption;
- default namespaced access semantics.

### `ClusterModel`

Cluster-scoped object.

Используется для:

- curated shared models;
- platform-provided base models;
- models, которые должны потребляться разными namespace'ами.

## 3. Common spec shape

Ниже — target shape, а не финальная YAML schema один-в-один.

```yaml
spec:
  displayName: "DeepSeek R1 Qwen3 8B"
  description: "Shared text-generation baseline for vLLM and KServe"

  source:
    type: HuggingFace | Upload | OCIArtifact

    huggingFace:
      repoID: deepseek-ai/DeepSeek-R1-0528-Qwen3-8B
      revision: <optional commit/tag>
      authSecretRef:
        name: hf-token
      allowGated: true

    upload:
      expectedFormat: ModelKit | HuggingFaceDirectory
      expectedSizeBytes: <optional>

    ociArtifact:
      ref: payload-registry.example/d8-ai-models/staging/...@sha256:...

  package:
    type: ModelPack
    layout: HuggingFaceCheckpoint

  publish:
    repositoryClass: internal-payload-registry
    channel: draft | candidate | champion

  runtimeHints:
    task: text-generation
    engines:
    - kserve
    - kuberay

  access:
    namespaces: []
    serviceAccounts: []
    groups: []
```

## 4. Source semantics

## 4.1. `source.huggingFace`

Назначение:

- canonical import path из HF Hub.

Особенности:

- data plane идёт внутри кластера через publish worker;
- для gated repo используется SecretRef;
- controller сам владеет packaging/publication.

## 4.2. `source.upload`

Назначение:

- upload модели с локального компьютера или из внешнего producer, у которого уже
  есть локальные bytes.

Особенности:

- пользователь не пишет final published artifact напрямую;
- контроллер переводит объект в `WaitForUpload`;
- status выдаёт upload session и command;
- рекомендуемый upload path — локальная упаковка в `ModelKit` и push в staging
  repo.

Важно:

- upload здесь — это orchestration contract, а не browser HTTP upload API.

## 4.3. `source.ociArtifact`

Назначение:

- интеграция с external producers;
- training pipeline уже собрал `ModelKit` и хочет только catalog admission.

Это минимальный путь для интеграции с training, не превращая `ai-models` в
training orchestrator.

## 5. Publish semantics

`spec.publish` определяет desired publication intent, но не раскрывает внутренний
repo layout полностью.

Минимальный contract:

- контроллер выбирает actual repository path;
- статус отдаёт final `ociRef` и digest;
- если later сменится registry backend, public API не должен ломаться.

## 6. Access semantics

## 6.1. `Model`

Если `spec.access` не задан:

- published artifact доступен service accounts из того же namespace.

Если `spec.access` задан:

- controller добавляет explicit grants поверх default policy либо заменяет её —
  это нужно решить на implementation task, но contract должен быть явным.

Рекомендация для первой реализации:

- default same-namespace read grant;
- explicit `spec.access` only broadens access, not narrows default.

Это даёт простой UX без сложной отрицательной логики.

## 6.2. `ClusterModel`

Для cluster-scoped model explicit access обязателен.

Если `spec.access` пуст:

- artifact не считается globally consumable;
- `Ready` не должен означать “доступно всем”.

## 7. Status shape

```yaml
status:
  observedGeneration: 1
  phase: Pending | WaitForUpload | Publishing | Syncing | Ready | Failed | Deleting

  source:
    resolvedType: HuggingFace
    resolvedRevision: 093f9f...

  upload:
    expiresAt: "2026-04-01T12:00:00Z"
    repository: payload-registry.../d8-ai-models/staging/model/<uid>:upload
    command: d8 ai-models model upload ...

  artifact:
    repository: payload-registry.../d8-ai-models/catalog/namespaced/team-a/deepseek
    tag: candidate
    digest: sha256:...
    ociRef: payload-registry.../d8-ai-models/catalog/namespaced/team-a/deepseek@sha256:...
    mediaType: <artifact media type>
    sizeBytes: 123

  metadata:
    task: text-generation
    framework: transformers
    family: qwen3
    license: MIT
    parameterCount: 8000000000
    quantization: null
    contextLength: 32768
    sourceRepoID: deepseek-ai/DeepSeek-R1-0528-Qwen3-8B

  backend:
    synchronized: true
    ref: <internal backend object ref or opaque id>

  access:
    grantsHash: <opaque hash>

  conditions: []
```

## 8. Conditions

Рекомендуемый набор:

- `Accepted`
  - object validated and admitted by controller
- `UploadReady`
  - only for `source.upload`
- `ArtifactStaged`
  - staging artifact exists and is consumable by controller
- `ArtifactPublished`
  - final immutable OCI ref published
- `MetadataReady`
  - metadata calculated and written to status
- `AccessConfigured`
  - registry read grants configured
- `BackendSynchronized`
  - internal backend mirror updated
- `Ready`
  - published artifact and required catalog metadata are ready

Стабильные `Reason` должны быть machine-readable, например:

- `SpecAccepted`
- `WaitingForUserUpload`
- `UploadExpired`
- `StageArtifactMissing`
- `PublicationSucceeded`
- `PublicationFailed`
- `MetadataInspectionFailed`
- `AccessGrantFailed`
- `BackendSyncFailed`

## 9. Immutability

После того как объект принят controller'ом, immutable должны быть:

- `spec.source`
- `spec.package`
- `spec.runtimeHints`
- repository-affecting parts of `spec.publish`

Mutable можно оставить только:

- `displayName`
- `description`
- non-functional metadata
- возможно `spec.access` для `ClusterModel`, если решим, что access grants —
  это lifecycle, а не artifact identity

Рекомендация:

- artifact-producing spec fields immutable;
- access policy можно менять отдельным reconcile path.

## 10. Ownership

### Creator owns

- `metadata`
- `spec`

### Controller owns

- `status`
- final repository path
- staging session lifecycle
- child jobs
- registry Role/RoleBindings
- backend sync side effects

### Consumers own

- только чтение artifact ref из status;
- не mutation public catalog object.

## 11. Delete semantics

Удаление `Model` / `ClusterModel` должно инициировать controlled cleanup:

1. revoke staging and consumer grants;
2. delete published OCI tag/repo objects if object is the owner;
3. trigger registry GC by normal module policy, not inline hard-delete layers;
4. delete backend mirror record;
5. clear upload session leftovers.

Важно:

- deletion не должен опираться на user manual cleanup в registry;
- cleanup policy должна быть controller-owned, а не implicit.

## 12. Что intentionally оставляем за пределами v1

- version streams внутри одного объекта;
- promotion workflows с несколькими children objects;
- signing/verification policy;
- cross-registry replication;
- browser upload UX;
- runtime-specific deployment policy.
