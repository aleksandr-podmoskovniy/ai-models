# Target Architecture

## 1. Честная цель

`ai-models` должен получить не внешний registry contract и не случайный набор
worker Pods, а module-owned hidden artifact plane по паттерну `virtualization`.

Целевой результат:

- пользователь работает с `Model` / `ClusterModel`;
- controller управляет ingestion, publication, cleanup и future delivery;
- опубликованная модель хранится только во внутреннем `DMCR`;
- сам `DMCR` хранит blobs на module-owned `S3`-compatible object storage;
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
- uploader, publisher, cleaner и materializer являются отдельными
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
- standalone materializer runtime for consumers

### Hidden ingest / provenance plane

- shared upload gateway
- controller-owned upload sessions
- controller-owned raw object URI allocation
- deterministic raw storage layout
- optional future append-only audit/log export
- optional future provenance manifests and validation reports

### Hidden storage plane

- internal `DMCR` service
- backend storage:
  - object storage
  - pvc
- raw object storage for direct multipart ingest and resumable source staging
- optional append-only audit/provenance prefix or sink

## 4. Целевые потоки данных

### 4.0 Raw ingest plane

Phase-2 raw ingest должен быть controller-owned и не зависеть от internal
backend entities.

Его целевая роль:

1. controller allocates the canonical raw object URI for a given owner object;
2. raw source bytes пишутся напрямую в object storage under that URI, без
   controller/apiserver/backend proxy;
3. upload concurrency scales through shared upload gateway replicas and
   short-lived session records, not through one Pod per upload;
4. controller records only the minimum internal session/provenance state needed
   to continue publication and cleanup;
5. async publish worker берёт raw source из controlled storage, перепаковывает
   в `ModelPack` и публикует в `DMCR`;
6. `CRD.status` stores the final immutable OCI reference and resolved profile;
7. raw source may be garbage-collected by policy after successful publication
   without changing the published OCI contract.

Что важно:

- `CRD.status` остаётся единственным platform truth;
- `DMCR` остаётся единственным storage truth для published artifact;
- raw object storage не становится вторым user-facing registry;
- optional future audit/provenance hooks не должны превращаться во вторую
  lifecycle/state machine поверх `CRD + DMCR`.

### 4.0.1 Почему здесь не нужен второй lifecycle plane

Для phase-2 здесь нельзя плодить второй online state owner.

Поэтому target не включает:

- run / workspace lifecycle в publish critical path;
- второй registry/version lifecycle besides `CRD + DMCR`;
- отдельные delete/archive semantics besides controller-owned finalizer flow;
- дополнительную auth/availability dependency для завершения publication.

Lineage, audit и future security scanners добавляются позже как append-only
hooks и отдельные async stages, а не как новый источник истины для
publication.

### 4.1 Remote URL -> DMCR

Поток:

1. user creates `Model` / `ClusterModel` with `source.url`;
2. controller performs preflight admission checks:
   - authz / owner binding;
   - quota and declared size limits;
   - `HuggingFace` host allowlist / credential resolution;
3. bounded fetch worker resolves the exact upstream revision, downloads the
   selected snapshot files and stages bytes into the canonical raw object URI
   in controlled object storage;
4. publish worker validates format, extracts profile, builds `ModelPack` and
   publishes to `DMCR`;
5. controller projects `status.artifact`, `status.resolved`,
   `Validated`, `Ready`;
6. cleanup handle and raw-retention state are stored only as internal
   controller-owned state.

### 4.2 Upload -> DMCR

Целевой flow должен быть двухфазным.

1. controller creates exactly one short-lived upload session `Secret` in the
   module namespace;
2. a shared `upload-gateway` exposes:
   - one shared `ClusterIP` `Service`;
   - zero or one shared external `Ingress`;
   - no per-upload `Service`;
   - no per-upload `Ingress`;
   - no per-upload uploader `Pod`;
3. gateway session `Secret` stores only:
   - owner identity (`Model` / `ClusterModel`, UID, generation);
   - canonical raw object URI;
   - declared input format / declared size;
   - expiry timestamp;
   - token hash;
   - multipart upload id;
   - uploaded part manifest (`partNumber`, `etag`, `size`);
   - lifecycle state (`issued`, `probing`, `uploading`, `uploaded`,
     `publishing`, `completed`, `failed`, `aborted`, `expired`);
4. client first calls a small probe step through the gateway:
   - uploads only the first bounded chunk;
   - gateway checks basic file signature / manifest sanity;
   - rejects obviously unsupported or malformed content before the full upload;
5. only after probe success does gateway create the multipart upload and issue
   presigned part URLs for the remaining bulk data;
6. client uploads bulk parts directly into object storage and may resume
   through the persisted multipart session state;
7. when multipart upload is completed, controller starts a separate async
   publish worker;
8. publish worker validates/profile/pack/pushes into `DMCR`;
9. raw staging is cleaned by lifecycle/TTL after success or failure.

Ключевое правило:

- upload runtime never keeps heavy publication logic in the same request path.
- primary preflight before full upload stays limited to fail-fast admission:
  authz, owner binding, quota, declared size/format allowlist and basic source
  metadata sanity; deeper malware/content scanning is a later async stage.

### 4.2.1 Exact upload-gateway API

The target control API is fixed:

- `GET /v1/upload/<sessionID>`
  - returns session state, declared limits and resumability metadata;
- `POST /v1/upload/<sessionID>/probe`
  - accepts only the bounded initial probe chunk;
- `POST /v1/upload/<sessionID>/init`
  - creates multipart upload after successful probe;
- `POST /v1/upload/<sessionID>/parts`
  - returns presigned URLs for requested part numbers;
- `POST /v1/upload/<sessionID>/complete`
  - submits final part manifest and closes raw ingest;
- `POST /v1/upload/<sessionID>/abort`
  - aborts multipart upload and expires the session.

The gateway is not a bulk byte proxy after probe. Its job is only:

- session auth;
- bounded probe validation;
- multipart orchestration;
- resumability state persistence.

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

- production baseline for large models: direct multipart upload to a
  controller-owned `raw/` prefix via short-lived presigned session;
- parallel part upload is allowed here;
- controller receives only session metadata and completion signal;
- after completion, async publish worker reads the staged object and publishes
  final `ModelPack` into `DMCR`;
- the same object-storage backend may be reused by `DMCR`, but only under a
  different logical prefix such as `dmcr/`.

#### Upload gateway footprint

- one shared `upload-gateway` Deployment with bounded replicas;
- one shared `Service`;
- zero or one shared external `Ingress`;
- one short-lived session `Secret` per upload, stored in the module namespace;
- one object-storage multipart upload per active transfer;
- no dedicated Pod / Service / Ingress trio per uploaded model.

Concrete baseline for cluster behavior:

- `50` concurrent uploads means:
  - `50` session `Secrets`;
  - `50` object-storage multipart uploads;
  - the same small shared `upload-gateway` replica set;
  - not `50` upload Pods.
- upload gateway replicas are sized for control-plane traffic only:
  auth, probe, presign, complete, abort;
- publish workers are scaled separately through an explicit concurrency limit
  and must not be spawned one-to-one with active uploads.

### What is explicitly not target

- no permanent `port-forward` UX;
- no synchronous `upload pod received final byte -> same pod starts packaging`;
- no design where 50 concurrent uploads create 50 uploader Pods just to hand
  out presigned URLs;
- no made-up “torrent upload into DMCR” requirement;
- no assumption that OCI registry itself is the right abstraction for parallel
  chunk ingest of one huge raw model file;
- no design where raw/audit data becomes a second lifecycle truth besides
  `CRD + DMCR`;
- no design where every raw source is blindly duplicated in full under some
  provenance store just to preserve history.

## 5.1 Publication runtime reality for very large models

Target architecture must state this honestly:

- `ModelPack` publication is not a streaming registry transform;
- current `KitOps` path expects a local model directory and therefore a
  materialized working set on disk;
- any implementation/review must explicitly answer:
  - how many full-size copies may exist at once;
  - where each copy lives;
  - what kind of volume holds it;
  - what requests/limits protect the node.

Correct target for 1+ TB class models:

- raw upload/download lands in controlled staging storage first;
- publish worker uses a dedicated bounded work volume, not an unbounded
  node-local scratch path;
- work-volume sizing and lifecycle are explicit in module config and runtime
  templates;
- single-file inputs must avoid unnecessary second local full copy when
  possible;
- if the concrete `ModelPack` implementation still requires full
  materialization, this is documented as a real operational requirement rather
  than hidden behind optimistic prose.

## 5.2 Recommended single target scenario for multi-terabyte models

Если выбрать один target scenario для этого модуля, он должен быть таким:

### Stage A. Raw ingest

- controller allocates canonical raw object URI under a controller-owned
  `raw/` prefix;
- user or fetch worker writes raw bytes directly into object storage under that
  raw URI;
- upload path uses multipart/presigned URLs;
- remote fetch path writes into the same controlled raw-storage area;
- raw storage is logically isolated from publication storage:
  - same S3 backend is acceptable;
  - same bucket is acceptable;
  - same prefix is not acceptable.

Required logical separation:

- `raw/` subtree for original bytes and resumable session data;
- optional `audit/` or equivalent append-only subtree for small manifests and
  future audit records;
- `dmcr/` or equivalent isolated publication subtree for published OCI blobs.

### Stage B. Validation and normalization

- a separate async publish worker starts only after raw ingest is complete;
- it mounts one dedicated bounded work volume or scratch PVC;
- it downloads or materializes exactly one working copy of the source into that
  work volume;
- whenever the input is already a single-file final input, the worker must not
  create an unnecessary second full local copy.

### Stage C. Publication

- the worker validates and profiles the source;
- it may emit small provenance manifests or audit events, but these stay
  append-only and non-authoritative;
- it publishes the final `ModelPack` into `DMCR`;
- after successful publication, the worker removes temporary work-volume data;
- raw object retention is governed separately from publish success, with the
  simplest target policy being: delete raw after successful publication unless
  retention/debug policy says otherwise.

### Copy budget

The recommended operational target is:

- one durable raw copy in object storage;
- one temporary bounded working copy in the publish worker;
- one durable published copy in `DMCR`.

Anything above that must be treated as debt and explicitly justified.

### Why this is the right target

- shortest critical path from upload completion to OCI publication;
- no second lifecycle engine;
- no second durable "prepared artifact" storage tier;
- no controller/apiserver data proxying;
- works with current `KitOps` constraint that still expects a local materialized
  model directory;
- leaves a clean migration path toward a future native OCI encoder.

### What is not acceptable as the target

- upload -> local Pod disk -> second local normalized copy -> third durable
  intermediate store -> `DMCR`;
- raw bytes proxied through controller/apiserver/backend server;
- blind duplication of raw payload into provenance/audit artifacts;
- assumption that current `KitOps` can stream directly from S3 to OCI without a
  local working set.

### 5.2.1 Streaming publication nuance

With the current `KitOps` CLI, the publish worker still needs a local
materialized model directory before `pack`.

Therefore the correct near-term target is:

- controller-owned raw URI in object storage;
- one bounded local working copy in the publish worker;
- direct push of the final OCI artifact to `DMCR`;
- no extra durable "prepared artifact" storage layer.

The stronger future target is:

- replace the current CLI adapter with a native OCI encoder that can read from
  the controller-owned raw object layout and stream directly into `DMCR`.

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
- controller-owned upload capability lifecycle.

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

### 6.7 Audit / provenance and session isolation

Target:

- phase-2 controller and runtimes rely only on module-owned machine
  credentials for raw storage, upload gateway and `DMCR`;
- optional future audit/provenance export is append-only and never gates
  publication or cleanup completion;
- the first concrete audit seam may be controller-owned `Kubernetes Events`
  plus minimal internal runtime provenance (`RawURI`, raw object count,
  total raw size), but these records remain best-effort append-only history
  and never become readiness gates or public `status`;
- namespaced and cluster-scoped publication state remains owned by the
  corresponding `Model` / `ClusterModel`, not by an external history system;
- browser/SRE visibility for future audit logs is a separate UX concern and
  must not become part of the publication critical path.

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
- `upload-gateway`
- `artifact-cleanup`
- `model-materializer`

Если несколько subcommands живут в одном image, это допустимо только пока
каждый остаётся thin bounded runtime, а не превращается в один giant
process-manager.

## 10. Что reuse из virtualization буквально

- hidden registry backend pattern;
- shared `Service + Ingress + session capability` pattern for uploads;
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
- upload path should reuse the uploader/session discipline from
  `virtualization`, but in our case the optimal target is a shared upload
  gateway plus direct-to-object-storage multipart ingest;
- publication remains async because our pipeline has an extra
  `validate + profile + pack` stage that virtualization uploader does not have
  in the same form.

## 12. Honest current drifts against the target

- upload path now exposes session URLs and no longer depends on
  `kubectl port-forward`;
- upload runtime now only owns multipart/session control and controller
  continues through a separate async publish worker;
- direct multipart/presigned staging is now landed for the object-storage
  staging path;
- current live upload path still creates per-session upload runtime objects,
  while the target large-model path prefers one shared upload gateway and only
  short-lived per-upload session records;
- PVC-specific uploader/staging path is still not landed;
- `KitOps` is still a concrete CLI adapter;
- consumer-side materializer/runtime delivery to `ai-inference` is still not
  landed;
- consumer-side read-only auth/trust projection into the standalone
  materializer is
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
