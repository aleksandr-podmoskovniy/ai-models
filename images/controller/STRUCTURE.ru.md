# Структура `images/controller`

Этот документ фиксирует живую package map controller runtime и объясняет,
почему границы именно такие.

Это не file-by-file inventory, не function inventory и не historical migration
log. Для slice history используются archived bundles; здесь остаётся только
current-state architecture. Если функция требует отдельного объяснения, это
должно жить рядом с кодом как Go doc, именованный test case или короткий
package-local comment, а не в этом документе.

Если пакет нельзя защитить хотя бы по одному из оснований ниже, его не надо
объяснять, его надо убрать:

1. отдельный execution entrypoint;
2. отдельный runtime contract или handoff model;
3. реальное переиспользование в нескольких live paths.

## 1. Что это за дерево

`images/controller/` — корень phase-2 controller runtime внутри DKP module.
Это не:

- публичный API модуля;
- packaging внутреннего backend;
- место для historical package names;
- общий toolbox для случайных helpers.

Текущее live дерево на уровне пакетов:

```text
images/controller/
  README.md
  STRUCTURE.ru.md
  TEST_EVIDENCE.ru.md
  go.mod
  go.sum
  werf.inc.yaml
  cmd/
    ai-models-controller/
    ai-models-artifact-runtime/
    ai-models-node-cache-runtime/
  internal/
    bootstrap/
    cmdsupport/
    domain/
      publishstate/
      ingestadmission/
      modelsource/
      storagecapacity/
    application/
      deletion/
      publishobserve/
      publishaudit/
    ports/
      publishop/
      modelpack/
      sourcemirror/
      uploadstaging/
      auditsink/
    monitoring/
      collectorhealth/
      catalogmetrics/
      runtimehealth/
      storageusage/
    publishedsnapshot/
    publicationartifact/
    nodecache/
    workloaddelivery/
    controllers/
      catalogstatus/
      catalogcleanup/
      nodecacheruntime/
      nodecachesubstrate/
      workloaddelivery/
    adapters/
      k8s/
        auditevent/
        cleanupstate/
        directuploadstate/
        modeldelivery/
        nodecacheruntime/
        nodecachesubstrate/
        ociregistry/
        ownedresource/
        sourceworker/
        storageaccounting/
        storageprojection/
        uploadsession/
        uploadsessionstate/
      sourcefetch/
      sourcemirror/
        objectstore/
      modelformat/
      modelprofile/
        common/
        diffusers/
        gguf/
        safetensors/
      modelpack/
        oci/
      uploadstaging/
        s3/
    dataplane/
      publishworker/
      uploadsession/
      artifactcleanup/
      backendprefix/
      nodecachecsi/
      nodecacheruntime/
    support/
      archiveio/
      cleanuphandle/
      modelobject/
      resourcenames/
      testkit/
      uploadsessiontoken/
```

## 2. Почему границы именно такие

### Корень и `cmd/`

- `README.md`, `STRUCTURE.ru.md`, `TEST_EVIDENCE.ru.md` остаются рядом с
  runtime tree.
- `go.mod`, `go.sum`, `werf.inc.yaml` образуют image-local build boundary.
- `cmd/ai-models-controller` — long-running controller manager.
- `cmd/ai-models-artifact-runtime` — shared short-lived runtime shell for
  `publish-worker`, `upload-session` / `upload-gateway`, `artifact-cleanup`
  and `materialize-artifact`.
- `cmd/ai-models-node-cache-runtime` — dedicated long-running node runtime
  shell for the privileged node-cache image and kubelet-facing CSI service.
- `cmd/` должен оставаться thin shell: env/argv parsing, exit codes, wiring
  into `internal/*`.
- `ai-models-artifact-runtime` больше не экспонирует legacy shell вроде
  `--snapshot-dir` или `node-cache-runtime`; runtime contract режется по
  реальным current commands, а не по старым migration paths.

### Composition root и process glue

- `internal/bootstrap/` — composition root manager runtime.
- `internal/cmdsupport/` — только shared process-level glue.
- `bootstrap` владеет scheme assembly, `ctrl.Manager`, controller registration,
  health/ready checks и metrics wiring.
- `cmdsupport` не должен знать concrete adapters, payload models или
  source-specific orchestration. Его logging glue нормализует envelope
  (`ts`, `level`, `msg`, `logger`) и error attribute как `err`, чтобы runtime
  logs оставались совместимыми с platform-style structured logs.

### Domain

- `internal/domain/publishstate/` — publication lifecycle, terminal semantics,
  conditions and status projection.
- `internal/domain/ingestadmission/` — cheap source-agnostic fail-fast
  invariants before heavy byte path.
- `internal/domain/modelsource/` — pure `Model` / `ClusterModel` source
  classification: exactly-one source validation, trusted remote source type
  detection and provider-specific URL parsing for `HuggingFace` / `Ollama`.
  It does not plan runtime Pods, fetch bytes or shape Kubernetes objects.
- `internal/domain/storagecapacity/` — pure artifact storage ledger math:
  capacity limit, committed bytes, active reservations, available bytes and
  `InsufficientStorage` decision.

Domain не должен знать concrete Kubernetes objects, pod shaping, secret CRUD
или transport details.

### Application

- `internal/application/publishobserve/` — runtime observation mapping and
  public status-mutation planning. Controller-local runtime-mode selection
  stays in `controllers/catalogstatus`; concrete source-worker plan shaping
  stays in `adapters/k8s/sourceworker`.
- `internal/application/publishaudit/` — append-only audit planning.
- `internal/application/deletion/` — delete-time policy seam.
- Empty application placeholders are not boundaries. A new application package
  appears only together with a real use-case contract and tests.

Это use-case seams. Их нельзя растворять ни в controllers, ни в adapters.

### Ports и handoff models

- `internal/ports/publishop/` — shared control-plane/runtime contract.
- `internal/ports/modelpack/` — replaceable `ModelPack` publish/remove/
  materialize contract.
- `internal/ports/sourcemirror/` — durable source-ingest contract over mirror
  manifest/state and persisted phase.
- `internal/ports/uploadstaging/` — staging contract для upload path.
  Live tree уже требует не только multipart shell, но и streaming-capable
  object reads там, где publish/runtime живут на object-source path.
- `internal/ports/auditsink/` — append-only audit sink contract.

Отдельно от ports остаются:

- `internal/publishedsnapshot/` — полный immutable publication snapshot;
- `internal/publicationartifact/` — более узкий artifact result payload и OCI
  destination policy;
- `internal/nodecache/` — shared node-local cache contract for digest-addressed
  shared store layout, ready markers, separate consumer
  materialization plus internal current-link и stable workload-model-link
  helper surface, single-writer coordination, per-digest retry/backoff,
  bounded eviction planning и module-owned maintenance loop;
- `internal/workloaddelivery/` — shared workload-delivery contract for resolved
  annotations, delivery mode/reason constants and HMAC signing. Controllers,
  K8s delivery adapter and node-cache runtime import this package instead of
  duplicating annotation strings or inventing per-adapter contracts;
- `internal/monitoring/collectorhealth/` — общий low-cardinality scrape-health
  contract для controller collectors: `collector_up`, scrape duration and last
  successful scrape timestamp. Он не знает public objects и не заменяет
  domain metrics;
- `internal/monitoring/catalogmetrics/` — Prometheus collectors over public
  `Model` / `ClusterModel` truth.
- `internal/monitoring/runtimehealth/` — Prometheus collectors over managed
  runtime-plane health for stable per-node node-cache agents and their shared
  `PVC`, including selector-scoped desired/managed/ready summary signal and
  aggregated workload-delivery mode/reason counts over managed top-level
  workloads plus module-private `DMCR` garbage-collection request lifecycle
  counts/age.
- `internal/monitoring/storageusage/` — Prometheus collectors over the
  module-private artifact storage ledger: configured limit, committed bytes,
  active reservations and available bytes.
- `cmd/ai-models-node-cache-runtime/` — dedicated thin executable shell for
  the privileged node-cache CSI/runtime image; it must stay separate from the
  shared publication/materialize runtime image.

### Controllers

- `catalogstatus/` — owner publication lifecycle.
- `catalogcleanup/` — delete/finalizer owner.
- `nodecachesubstrate/` — owner controller for managed local storage substrate
  over `sds-node-configurator` / `sds-local-volume` CR boundaries.
- `nodecacheruntime/` — owner controller for per-node runtime Pod/PVC,
  runtime-readiness Node label and node-selection reconciliation.
- `workloaddelivery/` — owner controller for built-in Kubernetes workloads
  with stable PodTemplate fields and annotations `ai.deckhouse.io/model` /
  `ai.deckhouse.io/clustermodel` / `ai.deckhouse.io/model-refs`. External
  controllers must render one of the supported Kubernetes workload templates
  or use a future trusted delivery API; ai-models не должен снова тащить
  controller-specific shims для `RayService`, `RayCluster` или любых других
  сторонних CRD.

Controller package оправдан ownership, а не удобством чтения. Новый controller
package без нового owner — patchwork.

### Concrete adapters

`internal/adapters/k8s/*` держит concrete Kubernetes shaping и CRUD:

- `sourceworker/`
- `cleanupstate/`
- `directuploadstate/`
- `modeldelivery/`
- `nodecacheruntime/`
- `nodecachesubstrate/`
- `uploadsession/`
- `uploadsessionstate/`
- `ociregistry/`
- `storageaccounting/`
- `storageprojection/`
- `ownedresource/`
- `auditevent/`

Non-K8s adapters:

- `sourcefetch/`
- `sourcemirror/objectstore/`
- `modelformat/`
- `modelprofile/*`
- `modelpack/oci/`
- `uploadstaging/s3/`

Главные правила:

- adapters не тащат public/status policy;
- `internal/nodecache/` остаётся отдельной reusable boundary: digest-addressed
  shared store layout, marker parsing, отдельные consumer-facing stable
  model-link и internal current-link helpers, single-writer coordination и
  bounded scan/eviction planning не должны жить внутри
  `cmd/ai-models-artifact-runtime` как скрытый mini cache-manager;
- `modelpack/oci/` остаётся controller-owned OCI adapter boundary:
  native publish/remove over `DMCR` direct-upload v2 с поздним digest и
  one-pass raw publish для range-capable тяжёлых слоёв плюс registry metadata
  path, mixed bundle/raw object-source и archive-source layer handling,
  legacy raw/tar плюс internal chunk-index/chunk-pack `ModelPack` layout
  validation/materialize routing, manifest/config validation, inspect/materialize
  и layer/media-type handling не должны утекать в `publicationartifact/` или в
  worker shell. Chunked layout остаётся internal immutable storage/distribution
  contract: workloads после materialize видят обычный каталог модели, а не
  custom runtime format;
- `sourcefetch/` остаётся boundary для remote source fetch, archive inspection,
  remote summary extraction и object-source planning;
- `k8s/nodecachesubstrate/` держит только shaping и live-state extraction для
  внешних storage CR (`LVMVolumeGroupSet`, `LVMVolumeGroup`,
  `LocalStorageClass`) и не тянет туда runtime delivery policy;
- `k8s/nodecacheruntime/` держит только concrete shaping для stable per-node
  runtime Pod/PVC и bounded runtime-side вычитку набора опубликованных
  артефактов, реально нужных live Pod'ам на текущей ноде; cache maintenance
  policy и public workload semantics не должны утекать туда. Единственный
  допустимый cross-boundary контракт здесь — `internal/workloaddelivery`
  resolved-annotation/signature contract; adapter-to-adapter imports из
  `modeldelivery` или controller-local annotation mirrors запрещены;
- `k8s/cleanupstate/` держит только module-private Secret-backed cleanup state:
  cleanup handle, completed marker and upload-stage handoff. Public status и
  delete policy остаются в `application/deletion` и controller owner;
- `k8s/storageaccounting/` держит только module-private Secret-backed
  capacity ledger: committed published bytes and active upload reservations.
  Quota policy, public usage API and object-storage scans сюда не входят;
- `k8s/directuploadstate/` держит только secret-backed checkpoint для
  direct-upload: owner-generation reset, persisted current-layer session state,
  committed-layer journal и terminal phase handoff; public progress/status
  policy не должна утекать туда;
- `k8s/modeldelivery/` остаётся boundary для workload mutation и теперь держит
  module-managed local bridge volume injection отдельно от storage substrate
  CR shaping, плюс стабильный workload-facing env contract: legacy primary
  model остаётся в `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST` и
  `AI_MODELS_MODEL_FAMILY`, а multi-model delivery отдаёт alias paths
  `/data/modelcache/models/<alias>`, `AI_MODELS_MODELS_DIR`,
  `AI_MODELS_MODELS` со списком alias/path/digest/family и per-alias
  `AI_MODELS_MODEL_<ALIAS>_{PATH,DIGEST,FAMILY}`. Bridge paths создают
  alias symlink поверх shared-store layout, а managed SharedDirect использует
  отдельные inline CSI volume attributes на каждый alias. Raw
  cache-root/current internals остаются внутри `internal/nodecache` and CSI
  runtime, не в workload mutation policy;
- live `HuggingFace` publish path больше не держит локальный
  `workspace/model` fallback: canonical path — direct or mirrored object source,
  cluster-level default теперь `Direct`, planning failure explicit, а
  потоковые multi-file источники публикуются как ограниченный bundle для
  мелких companion-файлов плюс отдельные raw-слои для крупных payload-файлов;
- `uploadstaging/s3/` и `sourcemirror/objectstore/` не должны сталкиваться по
  семантике: staging — upload/session surface, mirror — persisted source ingest.

### Dataplane

- `internal/dataplane/publishworker/`
- `internal/dataplane/uploadsession/`
- `internal/dataplane/artifactcleanup/`
- `internal/dataplane/backendprefix/`
- `internal/dataplane/nodecachecsi/`
- `internal/dataplane/nodecacheruntime/`

`publishworker`, `uploadsession` и `artifactcleanup` — controller-owned
one-shot runtimes. `backendprefix` — узкий helper для module-private DMCR
metadata prefix, общий для publish/cleanup dataplane; он не должен решать
cleanup policy или знать Kubernetes. `nodecachecsi` и `nodecacheruntime` —
long-running node-local dataplane inside the dedicated node-cache runtime image.
Все dataplane packages нельзя смешивать с reconciler code и нельзя откатывать
назад в backend scripts.

Live rules для `publishworker`:

- canonical publish paths — streaming/object-source first;
- successful upload/HF publish paths больше не идут через `checkpointDir`;
- valid upload path не должен silently деградировать в local materialization;
- consumer-side materialization остаётся отдельным runtime concern и живёт в
  `materialize-artifact`, а общий node-cache contract теперь живёт в
  `internal/nodecache`, а не в publish shell; bounded maintenance runtime тоже
  опирается на тот же package, а не собирает свою вторую cache semantics
  surface.

Live rules для node-cache dataplane:

- `nodecacheruntime` owns process wiring: env/argv parsing, logger setup, CSI
  server start, desired-artifact loading, per-digest retry/backoff and
  maintenance loop invocation;
- `nodecachecsi` owns only CSI identity/node service and bind-mount semantics:
  authorize requested digest, wait for ready digest, bind-mount read-only model
  path into kubelet target and update last-used marker;
- cache layout, ready markers, usage markers, eviction planning and
  materialization coordination stay in `internal/nodecache`.

### Shared support

- `cleanuphandle/`
- `archiveio/`
- `modelobject/`
- `resourcenames/`
- `testkit/`
- `uploadsessiontoken/`

`support/*` допустим только как shared helper layer. `archiveio` допустим здесь
только как reusable archive/range-reader primitive for source inspection and
extraction; если пакет решает policy, runtime semantics или status logic, это
уже не support.

## 3. Test discipline

Тесты считаются частью архитектуры, а не шумом вокруг production-кода.

Жёсткие правила:

- `_test.go` files живут под тем же LOC-budget, что и production files;
- tests режутся по decision surface, а не по helper reuse;
- helper-only test files допустимы только как thin shared seam внутри package;
- package-local branch matrix создавать нельзя: canonical evidence живёт только
  в `TEST_EVIDENCE.ru.md`.

Текущий live pattern:

- `publishobserve` split на source-worker runtime observation, upload-session
  observation и status mutation; runtime-mode gate теперь controller-local в
  `catalogstatus`, а source-worker plan shaping живёт в K8s adapter;
- `ingestadmission` split на common invariants, upload-session validation,
  upload probe classification and probe-shape validation instead of one mixed
  upload test file;
- `catalogstatus` split на source-worker/status, upload/status handoff и thin
  helper seams for runtime fakes, reconciler setup and termination payloads;
- `catalogcleanup` helper seams split на reconciler/build shell and cleanup
  runtime-state/assert helpers вместо одного helper-only file;
- `application/deletion` split на finalizer ownership, delete progress
  decision tables и backend-artifact vs upload-staging lifecycle proofs;
- `publishworker` split на HF fetch/streaming, remote profile, upload probe,
  upload streaming и result shaping;
- `uploadsession` split на session info, probe/init, multipart completion,
  handoff rejection and thin helper seams instead of one API test monolith;
- `backendprefix` tests prove safe OCI reference parsing for module-private
  DMCR metadata cleanup paths and reject schemes/path traversal;
- `k8s/uploadsession` tests split на get-or-create, projected persisted phase,
  delete semantics and controller-owned phase sync instead of one lifecycle
  matrix file;
- `workloaddelivery` tests split на apply/cleanup/topology вместо одного
  reconciler-local monolith;
- `sourcefetch` archive tests split на unpack, safetensors inspect and
  format-specific archive inspection helpers, direct `HuggingFace` proofs split
  на object-source happy path vs fail-closed planning, while mirror tests split
  on persisted snapshot-store, staging client/object-read helpers and HTTP
  multipart upload helpers;
- `k8s/sourceworker` tests split на owner identity, auth projection,
  concurrency/queued-handle semantics and runtime roundtrip вместо одного
  mixed service shell;
- `catalogmetrics` tests split на state-metric emission, incomplete-status
  behavior and thin gather/assert helpers вместо одного mixed collector file;
- `collectorhealth` tests prove reusable scrape-health semantics and duplicate
  collector registration safety through per-collector const labels;
- `runtimehealth` tests split на managed runtime resource emission and thin
  gather/assert helpers instead of mixing runtime-plane health with public
  catalog truth;
- `modelpack/oci` split на publish/materialize/validation/session shells,
  отдельные layer-matrix publish/validation/archive-helper proofs и thin
  file, registry-server, registry dispatch, registry upload-state and
  registry content-state helper seams вместо одного mixed helper shell;
- `modelpack/oci` chunked layout tests prove that legacy manifests remain
  valid, chunk-index/chunk-pack manifests validate separately from file layers,
  invalid chunk indexes fail closed, and synthetic chunked artifacts materialize
  back into the stable `model/` contract path.

## 4. Жёсткие правила на следующий refactor

- Не добавлять новый пакет ради одной пустой оболочки поверх уже живого
  контракта.
- Не возвращать generic names вроде `app`, `common`, `runtime`, `publication`,
  если за ними не стоит новая граница ownership.
- Не плодить локальные mirrors существующих shared contracts.
- Не держать в shared port package мёртвые handoff types.
- Не тащить concrete K8s object shaping в `application/*`.
- Не тащить lifecycle policy в `support/*`.
- Не вводить второй persisted bus или второй lifecycle source of truth между
  controller, upload session и publish worker.

## 5. Текущие hotspots

### `internal/controllers/catalogcleanup/`

- Пакет тяжёлый по реальной причине:
  delete lifecycle включает cleanup operation, GC request и finalizer release.
- Рост сюда допустим только внутри того же owner.
- Всё, что не является owner-level delete orchestration, надо выносить в
  `application/deletion`, `resourcenames` или concrete adapters.

### `internal/adapters/sourcefetch/`

- Это всё ещё самый тяжёлый concrete adapter.
- Это допустимо, пока boundary остаётся только про remote source fetch.
- Сюда нельзя складывать format validation, publish status или runtime policy.
- Даже внутри `sourcefetch/` archive dispatch, remote summary, mirror transfer и
  direct object-source planning не должны снова схлопываться в один giant file.

### `internal/dataplane/publishworker/`

- Пакет остаётся ключевым byte-path runtime.
- Здесь нельзя снова смешивать:
  - HF orchestration
  - upload probing
  - provenance shaping
  - profile resolution
  - publish invocation
  в один oversized `run.go`.

### `internal/controllers/workloaddelivery/`

- Пакет остаётся owner-level reconciler for supported Kubernetes workloads.
- Сюда нельзя возвращать branches под конкретные сторонние CRD/controllers
  вроде `RayService` / `RayCluster`: это снова создаст протекание абстракций.
- Если external runtime требует интеграцию, он должен либо рендерить supported
  PodTemplate workload, либо получить отдельный explicit delivery API slice.

### `internal/nodecache/`

- Пакет стал durable shared contract для node-local cache и поэтому может быть
  больше простого helper package.
- Рост допустим только по двум осям:
  - stable cache layout / marker / usage / eviction contract;
  - runtime loop policy that is shared by materialize and node runtime.
- CSI protocol details, K8s Pod discovery and controller scheduling policy сюда
  переносить нельзя.

### `internal/dataplane/nodecachecsi/`

- Пакет должен оставаться kubelet-facing protocol adapter.
- Он не должен читать Kubernetes API напрямую, решать placement или выбирать
  source of truth для desired artifacts.
- Любая логика выше `authorize -> ready digest -> bind mount` должна жить в
  controller/runtime contract packages.

## 6. Текущий вердикт

Controller/runtime tree соответствует текущему runtime baseline:

- native `ModelPack` publisher живёт в `modelpack/oci`;
- `HF` и canonical upload paths выровнены на streaming/object-source semantics;
- legacy backend/publication shell вычищен из live controller tree;
- stable node-cache delivery split now has controller owner, K8s shaping
  adapter, shared cache contract, dedicated node runtime process and CSI node
  dataplane;
- current documentation and test evidence должны обслуживать только этот
  baseline, а не transition history.
