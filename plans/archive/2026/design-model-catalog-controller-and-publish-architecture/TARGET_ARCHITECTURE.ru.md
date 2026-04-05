# TARGET ARCHITECTURE

## 1. Executive summary

Для phase 2 `ai-models` должен стать не "обёрткой над raw MLflow UI", а
platform control plane для опубликованных моделей.

Ключевое решение:

- public contract платформы — `Model` и `ClusterModel`;
- canonical publish artifact — `ModelPack/ModelKit`, опубликованный в
  `payload-registry` как OCI artifact;
- `payload-registry` используется как DKP-native distribution plane и auth/RBAC
  boundary;
- internal managed backend остаётся внутренним metadata/provenance/evaluation
  backend и не должен быть canonical serving storage;
- `KServe` — первый class-A consumer published ModelKit;
- `KubeRay` поддерживается через adapter path, а не определяет весь контракт.

Если попытаться строить phase 2 вокруг raw MLflow artifact layout, получится:

- сложный user-facing UX;
- слабый RBAC на data plane;
- неудобные URI для serving;
- прямое протекание внутренних backend сущностей в platform contract.

Поэтому phase 2 должен отделить:

- public catalog API;
- publish/distribution plane;
- internal backend integration;
- external serving planes.

## 2. Главные проектные принципы

### 2.1. Public API не знает о внутреннем backend layout

Пользователь работает с `Model` и `ClusterModel`, а не с:

- `Experiment`;
- `Run`;
- `Logged Model`;
- `Workspace`;
- `Model Registry Version`.

Эти сущности могут существовать внутри backend, но не являются public contract.

### 2.2. Published artifact должен иметь стабильный OCI ref

Платформе нужен нормальный publish/distribution contract:

- стабильный `ociRef`;
- digest;
- registry auth/RBAC;
- возможность использовать runtime consumers без bucket-policy gymnastics.

Сырые `s3://.../models/m-<uuid>/artifacts/model` для platform UX не подходят.

### 2.3. Data plane локального upload не должен идти через браузер и не должен
проходить через controller API server path

Для локального upload больших моделей нужен controller-owned handoff:

- controller создаёт upload session;
- пользователь получает понятную upload command;
- data plane идёт напрямую в registry staging path;
- controller потом принимает и публикует артефакт.

Это тот же класс решения, который в virtualization используется для DVCR:
контроллер владеет orchestration, а не байтами браузерного upload.

### 2.4. canonical publish artifact и internal backend artifact — разные вещи

Если выбрать `ModelPack/ModelKit` как canonical published form, то internal
backend не должен пытаться оставаться вторым canonical storage того же artifact.

Иначе возникает двойное владение:

- published OCI artifact;
- дублирующий MLflow/S3 artifact.

Для phase 2 правильнее:

- public serving artifact живёт в OCI registry;
- internal backend хранит metadata, provenance, evaluation outputs и ссылки на
  published artifact;
- если позже понадобится double-write в backend artifacts, это должен быть
  отдельный осознанный decision, а не скрытый побочный эффект.

## 3. Компоненты и их роли

## 3.1. `Model` / `ClusterModel`

Это public DKP API.

Они описывают:

- откуда приходит модель;
- как она должна быть упакована;
- куда она должна быть опубликована;
- какие access grants нужны;
- в каком состоянии находится publish flow;
- какие metadata и validation results доступны платформе.

Они не описывают внутренние MLflow объекты.

## 3.2. `ai-models-controller`

Контроллер под `images/controller/` — единственный владелец orchestration.

Он:

- валидирует `spec`;
- создаёт временные upload grants;
- создаёт publish jobs;
- вычисляет конечный OCI ref;
- рассчитывает status metadata;
- управляет cleanup;
- управляет sync в internal backend;
- настраивает registry read access для consumers.

Контроллер не должен быть data-plane proxy для model bytes.

### 3.2.1. Внутренняя структура controller runtime

Внутри runtime нужны отдельные logical parts:

- `ModelReconciler`
  - lifecycle namespaced `Model`
- `ClusterModelReconciler`
  - lifecycle cluster-scoped `ClusterModel`
- `UploadSessionManager`
  - выдаёт и отзывает staging grants, TTL и upload commands
- `PublicationOrchestrator`
  - запускает/наблюдает publish worker jobs
- `RegistryAccessManager`
  - materializes read grants для consumers
- `BackendSyncAdapter`
  - делает secondary sync в internal backend

Эти части могут жить в одном controller binary, но ownership обязан быть явным.

### 3.2.2. Adapter boundaries

Чтобы не прибить public API к текущему implementation choice, внутри controller
нужны явные adapter interfaces:

- `SourceResolver`
  - `HuggingFaceSourceResolver`
  - `UploadSourceResolver`
  - `OCIArtifactSourceResolver`
- `Publisher`
  - initial implementation: `PayloadRegistryPublisher`
- `MetadataInspector`
  - initial implementation: `KitOpsInspector`
- `BackendMirror`
  - initial implementation: `MLflowMetadataMirror`

Это важно по двум причинам:

- `payload-registry` может позже смениться на `Harbor` без смены public API;
- internal backend может эволюционировать отдельно от publish plane.

## 3.3. Publisher worker image

Нужен отдельный image-owned worker/runtime, который умеет:

- скачивать модель из Hugging Face;
- собирать `ModelKit`;
- пушить artifact в staging repository;
- инспектировать artifact через `kit inspect`;
- промоутить staging artifact в final immutable repository/tag;
- отдавать structured report контроллеру.

Этот runtime должен жить под `images/*`, а не inside `api/`.

### 3.3.1. Worker modes

Чтобы не плодить одноразовые binaries, worker лучше сделать multi-mode entrypoint:

- `hf-import`
- `promote-upload`
- `inspect`
- `cleanup`

Тогда controller orchestration reuse'ит один runtime contract, а не зоопарк из
несвязанных helper scripts.

## 3.4. `payload-registry`

Используется как OCI distribution plane.

Что он нам реально даёт:

- DKP-native auth через Kubernetes tokens;
- path-based authz через `PayloadRepositoryAccess`;
- OCI registry API;
- K8s API extension (`PayloadRepositoryTag`) для listing/tag inspection;
- избавление от S3 prefix ACL для serving consumers.

Что он не должен знать:

- semantics `Model` / `ClusterModel`;
- внутренний backend;
- controller conditions.

## 3.5. Internal managed backend

Internal backend остаётся внутри `ai-models` для:

- provenance;
- experiment/evaluation history;
- optional UI/debug/admin flows;
- будущих research-oriented workflows.

Но phase-2 public controller не должен зависеть от raw backend UX как от primary
publish path.

Целевая роль backend в phase 2:

- metadata mirror;
- evaluation/provenance sink;
- secondary integration condition.

Не canonical serving storage.

## 3.6. Serving consumers

### `KServe`

Первый class-A consumer.

Target contract:

- published `ModelKit` / `kit://` or OCI-backed integration;
- registry auth через service account / registry credentials;
- consumer не знает о MLflow.

### `KubeRay`

Не должен ломать основной publish contract.

Для него target path такой:

- canonical artifact всё равно published в OCI registry;
- KubeRay consumption идёт через adapter:
  - init-container unpack `ModelKit` в shared volume;
  - далее local path для Ray runtime.

Если нужен временный operational shortcut, можно оставить direct S3 bucket path,
но это не должно определять public API и canonical publish form.

## 4. Почему canonical publish form — `ModelPack/ModelKit`

### 4.1. Что это даёт

- стабильный registry URI;
- OCI ecosystem и digest semantics;
- нормальный pull auth/RBAC;
- естественную интеграцию с `KServe`;
- controller-friendly packaging discipline;
- возможность считать published model как immutable promotion artifact.

### 4.2. Что это не заменяет

`ModelPack/ModelKit` не заменяет:

- catalog API;
- evaluation history;
- platform status;
- access policy;
- controller orchestration.

Поэтому `KitOps` — packaging/distribution layer, а не весь `ai-models`.

## 5. Registry ownership model

### 5.1. Dedicated stable root namespace

Нельзя публиковать модели под namespace команд напрямую, потому что
`payload-registry` удаляет содержимое root prefix при удалении namespace.

Поэтому published artifacts должны жить под стабильным module-owned namespace,
например:

- `d8-ai-models`

или под отдельным стабильным namespace, созданным модулем.

Главное свойство:

- lifetime published artifacts управляется модулем, а не user namespace.

### 5.2. Repository layout

Рекомендуемый layout:

- staging uploads:
  - `payload-registry.<domain>/d8-ai-models/staging/<kind>/<uid>:upload`
- published namespaced models:
  - `payload-registry.<domain>/d8-ai-models/catalog/namespaced/<namespace>/<name>:<tag>`
- published cluster models:
  - `payload-registry.<domain>/d8-ai-models/catalog/cluster/<name>:<tag>`

Canonical ref в status должен быть digest-based:

- `...@sha256:<digest>`

Tag нужен для human/debug convenience, но не должен быть единственным
machine-facing identifier.

### 5.3. Staging -> final promotion

Пользователь и source jobs не должны писать сразу в final repo.

Правильный path:

1. source artifact попадает в staging repository;
2. controller/worker проверяет структуру и metadata;
3. controller промоутит artifact в final repo;
4. status получает final digest ref;
5. staging grant убирается.

Это важно для:

- immutability;
- policy gates;
- cleanup discipline;
- отделения user upload от final publication.

## 6. Access model и роли

## 6.1. Роли

### Publisher

Кто создаёт `Model` / `ClusterModel`.

Права:

- создавать и удалять catalog CRs;
- инициировать upload session;
- смотреть status, metadata и conditions.

Не права:

- писать в final published registry path напрямую.

### Controller

Module-owned actor.

Права:

- создавать staging grants;
- запускать publish workers;
- публиковать final artifacts;
- удалять published artifacts;
- создавать access Role/RoleBinding для consumers;
- синхронизировать metadata в backend.

### Consumer

Inference workload или другой runtime, который читает already published
artifact.

Права:

- pull published artifact;
- не push;
- не управляет catalog object.

### Platform admin

Права:

- создавать `ClusterModel`;
- управлять cluster-wide access policy;
- управлять module defaults и cleanup policies.

## 6.2. Registry RBAC

Используем не внешние object-storage ACL, а registry authz.

Базовый механизм:

- auth: Kubernetes tokens;
- authz: `PayloadRepositoryAccess`.

Практический pattern:

- controller создаёт Role/RoleBinding в stable registry namespace;
- subjects:
  - `system:serviceaccounts:<namespace>` для namespaced consumers;
  - отдельные ServiceAccounts;
  - группы пользователей, если нужен human pull;
- `resourceNames` ограничивают repo path patterns.

## 6.3. Namespaced vs cluster-scoped defaults

### `Model`

Default read policy:

- published artifact доступен service accounts того же namespace, что и `Model`.

Это соответствует namespaced ownership и даёт простой default UX.

### `ClusterModel`

Default read policy:

- ничего автоматически не выдаётся всем;
- доступ описывается явно в `spec.access`.

Такой split лучше, чем “всем всё” по умолчанию.

## 7. Internal backend integration

## 7.1. Что sync'им

Controller должен sync'ить в internal backend:

- identity model object;
- source provenance;
- published OCI ref и digest;
- optional validation/evaluation reports;
- status of publication.

## 7.2. Чего не делаем

Не делаем phase-2 contract зависящим от того, что internal backend также хранит
все published bytes как canonical serving artifact.

Если оставить backend только как metadata/provenance sink, мы избегаем:

- двойной data ownership;
- лишней зависимости serving от raw MLflow layout;
- путаницы между public API и internal research backend.

## 7.3. Как отражать это в status

Sync в internal backend должен быть отдельной condition:

- `BackendSynchronized=True/False`

Не смешивать её с:

- `ArtifactPublished`;
- `Ready`.

Возможна ситуация:

- published OCI artifact уже готов и безопасен для serving;
- backend sync временно деградировал.

Это не должно лишать пользователя published artifact.

## 8. Канонический runtime picture

### 8.1. HF source

`Model/ClusterModel -> controller -> publish worker -> HF snapshot -> ModelKit -> staging -> final repo -> status -> optional backend sync`

### 8.2. Local upload source

`Model/ClusterModel -> WaitForUpload -> user runs upload command -> ModelKit in staging repo -> controller verify/promote -> final repo -> status -> optional backend sync`

### 8.3. Serving path

`published OCI ref -> consumer-specific adapter`

- `KServe`: direct KitOps/OCI integration
- `KubeRay`: init-container unpack or later dedicated adapter

## 9. Что не делать

- Не делать public API равным “MLflow-as-a-CRD”.
- Не хранить user-facing contract на raw `s3://.../m-uuid/...`.
- Не делать browser upload больших artifacts.
- Не давать users push в final published repo.
- Не делать `KubeRay` ограничением, которое запрещает использовать canonical
  OCI publish artifact.

## 10. Implementation direction

### Первая implementable цель

1. `Model` / `ClusterModel` API.
2. Controller + worker для:
   - HF source;
   - upload source via staging repo.
3. Publication into `payload-registry`.
4. Status metadata and conditions.
5. Default access grants for namespaced `Model`.
6. `KServe` first-class consumption.

### Suggested implementation order

1. API types and validation/defaulting.
2. Registry namespace/path conventions and access manager.
3. Upload session lifecycle.
4. HF publish worker.
5. Upload promote worker.
6. Status metadata calculation.
7. `KServe` consumption helper/reference docs.
8. Backend metadata mirror.

### Следующий slice

1. `ClusterModel` access policy hardening.
2. `KubeRay` adapter path.
3. Backend metadata sync.
4. Validation jobs and reports.

## 11. References

- `payload-registry` docs in local repo:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/payload-registry/docs/README.md`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/payload-registry/docs/AUTH.md`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/payload-registry/docs/KUBERNETES-API.md`
- `KitOps` / `ModelPack`:
  - https://modelpack.org/
  - https://kitops.org/docs/overview/
  - https://kitops.org/docs/integrations/kserve/
  - https://kitops.org/docs/integrations/k8s-init-container/
- `Ray Serve LLM` model loading:
  - https://docs.ray.io/en/master/serve/llm/user-guides/model-loading.html
- `virtualization` upload/DVCR reference:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/docs/internal/dvcr_auth.md`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/core/v1alpha2/virtual_image.go`
