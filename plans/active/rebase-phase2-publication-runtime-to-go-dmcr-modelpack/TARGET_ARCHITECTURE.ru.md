# Target Architecture

## 1. Честная цель

`ai-models` должен получить не внешний registry contract и не случайный набор
worker Pods, а module-owned hidden artifact plane по паттерну `virtualization`.

Целевой результат:

- пользователь работает с `Model` / `ClusterModel`;
- controller управляет ingestion, publication, cleanup и future delivery;
- опубликованная модель хранится только во внутреннем `DMCR`;
- сам `DMCR` хранит blobs на module-owned backend storage:
  - `S3`-compatible object storage;
  - `PersistentVolumeClaim`;
- runtime consumer не знает про исходный upload/source flow и получает только
  локальный materialized path из опубликованного `ModelPack`.

Что не является целью:

- наружу не выдаётся raw `DMCR` как user-facing product;
- пользователю не выдаются постоянные registry credentials;
- upload path не идёт через `kubectl port-forward` и не держит
  `receive + validate + pack + push` в одном request lifecycle;
- target architecture не обещает shard/torrent download into registry:
  параллелизм, если он нужен, делается на staging/object-storage уровне, а не
  как фантазийный registry feature.

## 2. Архитектурные принципы

### Гексагональная граница

Control plane должен быть разбит на:

- `domain`
  - publication state
  - upload-session state
  - delete / GC state
  - policy validation
- `application`
  - start remote ingest
  - issue upload session
  - observe upload/ingest/publish completion
  - finalize delete
  - request materialization
- `ports`
  - source fetch
  - upload session issuing
  - staging store
  - model format validation
  - model profile extraction
  - modelpack publication
  - artifact cleanup / GC request
  - runtime materialization
- `adapters`
  - K8s runtime supplements
  - DMCR auth/trust projection
  - object-storage multipart staging
  - PVC-backed staging
  - concrete `ModelPack` implementation

### SOLID / anti-monolith

- controller manager остаётся thin orchestration shell;
- uploader, publisher, cleaner и future materializer являются отдельными
  bounded runtimes, даже если временно лежат в одном image/subcommand shell;
- никакой runtime не должен одновременно быть:
  - edge upload endpoint;
  - heavy validator/profiler;
  - pack/push worker;
  - cleanup executor.

### Reuse from virtualization, but honestly

Берём из `virtualization` не branding, а работающие boundaries:

- hidden module-owned registry backend;
- controller-owned uploader/importer lifecycles;
- session-specific supplements: `Pod`, `Service`, `Ingress`, temporary auth;
- on-demand secret/CA projection across namespaces;
- maintenance/read-only mode for physical GC;
- cleanup as explicit controller-owned lifecycle.

Не копируем слепо:

- CDI/DataVolume-specific PVC import path;
- VM/disk-specific CR graph;
- consumer contract, завязанный на image/disk semantics.

## 3. Компоненты target system

### Public API / control plane

- `Model`
- `ClusterModel`
- controller manager

Публичный contract:

- `spec.source`
  - `source.url`
  - `source.upload`
- `spec.inputFormat`
- `spec.runtimeHints`
- live policy contract:
  - `spec.modelType`
  - `spec.usagePolicy`
  - `spec.launchPolicy`
  - `spec.optimization`

Публичный status:

- source acceptance / upload readiness
- publication progress
- resolved technical profile
- final published artifact reference
- validation / readiness conditions

### Hidden data plane

- remote fetch worker
- upload edge runtime
- async publish worker
- artifact cleanup job
- future materializer runtime for consumers

### Hidden storage plane

- internal `DMCR` service
- backend storage:
  - object storage
  - pvc
- optional staging area for uploads before final publication

## 4. Целевые потоки данных

### 4.1 Remote URL -> DMCR

Поток:

1. user creates `Model` / `ClusterModel` with `source.url`;
2. controller resolves provider and credentials;
3. fetch worker downloads source bytes into controlled workspace/staging;
4. publish worker validates format, extracts profile, builds `ModelPack`,
   publishes to `DMCR`;
5. controller projects `status.artifact`, `status.resolved`,
   `Validated`, `Ready`;
6. cleanup handle is stored only as internal controller-owned state.

### 4.2 Upload -> DMCR

Целевой flow должен быть двухфазным.

1. controller creates upload session;
2. session exposes:
   - `external` URL via `Ingress`;
   - `inCluster` URL via `Service`;
3. upload runtime only accepts bytes and writes them into durable staging;
4. when upload is completed, controller starts a separate async publish worker;
5. publish worker validates/profile/pack/pushes into `DMCR`;
6. upload staging is cleaned by lifecycle/TTL after success or failure.

Ключевое правило:

- upload runtime never keeps heavy publication logic in the same request path.

### 4.3 DMCR -> consumer runtime

Поток:

1. consumer integration asks for a published model by internal artifact ref;
2. controller/materializer receives read-only registry auth and trust bundle;
3. materializer pulls `ModelPack` from `DMCR`;
4. materializer expands it into a local path in the original model format;
5. inference pod consumes only the local path, not the registry protocol.

Это отдельный future slice, но target contract уже должен быть именно таким.

### 4.4 Delete / cleanup

Поток:

1. CR delete triggers finalizer;
2. controller starts logical artifact removal;
3. controller requests physical blob GC;
4. module hooks switch `DMCR` into maintenance/read-only mode;
5. `dmcr-cleaner` executes registry garbage collection;
6. controller removes finalizer only after GC completion.

## 5. Upload architecture for large models

### What is target

Для больших моделей правильный target не stream через apiserver и не
single-pod critical section, а `staging-first`.

#### ObjectStorage mode

- best-effort target: direct multipart upload to module-owned staging bucket or
  staging prefix via short-lived presigned session;
- parallel part upload is allowed here;
- controller receives only session metadata and completion signal;
- after completion, async publish worker reads the staged object and publishes
  final `ModelPack` into `DMCR`.

#### PVC mode

- upload goes to a dedicated uploader runtime through `Service` / `Ingress`;
- uploader writes only to module-owned staging volume;
- it does not pack or push to registry;
- a separate publish worker later reads from staging and publishes to `DMCR`.

### What is explicitly not target

- no permanent `port-forward` UX;
- no synchronous `upload pod received final byte -> same pod starts packaging`;
- no made-up “torrent upload into DMCR” requirement;
- no assumption that OCI registry itself is the right abstraction for parallel
  chunk ingest of one huge raw model file.

## 6. Auth, trust, authorization, isolation

### 6.1 User upload authorization

Upload authorization must be session-scoped.

Target:

- controller creates a short-lived upload session token/capability;
- user gets only:
  - short-lived upload URL;
  - optional short-lived upload secret/token;
- session is bound to one owner object and expires automatically;
- uploader rejects uploads after completion/expiry.

Pattern reused from `virtualization`:

- `Ingress` + secret URL / session identity in status;
- controller-owned uploader Pod lifecycle.

### 6.2 DMCR write authorization

Write access to `DMCR` is internal-only.

Target:

- only controlled runtimes receive write credentials:
  - publish worker
  - upload-to-staging finalizer if needed
  - cleanup runtime
- credentials live in module-owned Secret of type
  `kubernetes.io/dockerconfigjson`;
- cross-namespace access happens through controller-created copies or
  projections, as in `virtualization`, not via cluster-global unrestricted
  secret reuse;
- controller removes derived secrets after runtime completion.

### 6.3 DMCR read authorization

Read auth for consumers is separate from write auth.

Target:

- materializer/init runtime gets read-only credentials;
- consumer namespaces never receive write credentials;
- cluster-scoped publication does not mean cluster-global write access.

### 6.4 TLS and CA trust

Target:

- `DMCR` serves HTTPS;
- internal clients trust a projected CA bundle;
- upload ingress uses normal TLS termination;
- CA/config projection is explicit and controller-owned, not hidden in shell;
- if some compatibility mode temporarily uses insecure internal transport, that
  is documented as debt and never described as target architecture.

### 6.5 Network isolation

Target:

- `DMCR`, uploader and one-shot runtimes are isolated with `NetworkPolicy`;
- metrics stay behind `kube-rbac-proxy`;
- external traffic reaches only the upload ingress, not the registry service;
- controller talks to internal services only via cluster-local endpoints.

### 6.6 Namespace ownership

Target:

- namespaced `Model` gets namespaced supplements;
- `ClusterModel` may use module namespace for cluster-owned runtimes;
- copied secrets/CA bundles are owned by the corresponding session/job and
  deleted afterwards;
- no long-lived credentials are spread across user namespaces without a direct
  owner object.

## 7. DMCR responsibilities

`DMCR` in target architecture is responsible for:

- storing published `ModelPack` artifacts;
- serving them over internal OCI registry API;
- storing blobs on object storage or pvc;
- switching into maintenance/read-only mode for physical GC;
- staying invisible as a user-facing product surface.

`DMCR` is not responsible for:

- source download;
- user upload orchestration beyond registry-side storage;
- format validation;
- model profiling;
- inference delivery policy.

## 8. ModelPack implementation seam

Target contract remains:

- `ModelPack` is the internal publication artifact contract;
- concrete tooling is replaceable.

Acceptable implementations:

- current `KitOps` adapter as intermediate step;
- future native Go implementation;
- other bounded implementation only behind the same port.

Target rule:

- business/application logic never depends on a concrete CLI brand;
- runtime image may carry an implementation tool only as an adapter detail, not
  as the architecture itself.

## 9. Package-level target ownership

### Controller module

- `internal/domain/publishstate`
- `internal/domain/uploadstate`
- `internal/domain/deletestate`
- `internal/application/publishplan`
- `internal/application/publishobserve`
- `internal/application/uploadsession`
- `internal/application/deletion`
- `internal/application/materialization`
- `internal/ports/*`
- `internal/controllers/catalogstatus`
- `internal/controllers/catalogcleanup`

### K8s adapters

- `internal/adapters/k8s/sourceworker`
- `internal/adapters/k8s/uploadsession`
- `internal/adapters/k8s/materializer`
- `internal/adapters/k8s/ociregistry`
- `internal/adapters/k8s/ownedresource`
- `internal/adapters/k8s/workloadpod`

### Data-plane runtimes

- `publish-worker`
- `upload-uploader`
- `artifact-cleanup`
- `model-materializer`

Если несколько subcommands живут в одном image, это допустимо только пока
каждый остаётся thin bounded runtime, а не превращается в один giant
process-manager.

## 10. Что reuse из virtualization буквально

- hidden registry backend pattern;
- `uploader Pod + Service + Ingress` pattern;
- on-demand auth secret copy/projection pattern;
- CA bundle projection pattern;
- explicit GC lifecycle with maintenance/read-only mode;
- separation between controller decisions and runtime binaries.

## 11. Что отличается от virtualization принципиально

- published artifact is `ModelPack`, not VM image/disk;
- before final publication we must validate format and compute AI-specific
  profile metadata;
- consumer side ends in local model directory/file for inference runtime,
  not in CDI/DataVolume import;
- upload path should adopt the uploader/session discipline from
  `virtualization`, but publication remains async because our pipeline has an
  extra `validate + profile + pack` stage that virtualization uploader does not
  have in the same form.

## 12. Honest current drifts against the target

- upload path now exposes session URLs and no longer depends on
  `kubectl port-forward`;
- upload runtime now only stages bytes and controller continues through a
  separate async publish worker;
- direct multipart/presigned staging for very large uploads is still not
  landed;
- `KitOps` is still a concrete CLI adapter;
- materializer/runtime delivery to `ai-inference` is still not landed;
- consumer-side read-only auth/trust projection into the future materializer is
  still not landed.

## 13. Definition of architectural done

Target architecture is only considered landed when all of the following are
true:

- `source.upload` uses session URLs and no longer depends on `port-forward`;
- upload runtime only receives bytes and writes them to staging;
- async publish worker is separate from upload edge runtime;
- `DMCR` auth is split into write vs read credentials;
- cross-namespace secret/CA projection is controller-owned and cleaned up;
- `DMCR` GC works through explicit maintenance/read-only lifecycle;
- consumer delivery/materialization exists as a separate bounded runtime;
- public docs no longer claim any of the above before the code actually lands.
