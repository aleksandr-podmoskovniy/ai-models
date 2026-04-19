# Структура `images/controller`

Этот документ фиксирует живую package map controller runtime и объясняет,
почему границы именно такие.

Это не file-by-file inventory и не historical migration log. Для slice history
используются archived bundles; здесь остаётся только current-state architecture.

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
  internal/
    bootstrap/
    cmdsupport/
    domain/
      publishstate/
      ingestadmission/
    application/
      deletion/
      publishplan/
      publishobserve/
      sourceadmission/
      publishaudit/
    ports/
      publishop/
      modelpack/
      sourcemirror/
      uploadstaging/
      auditsink/
    monitoring/
      catalogmetrics/
    publishedsnapshot/
    publicationartifact/
    nodecache/
    controllers/
      catalogstatus/
      catalogcleanup/
      nodecachesubstrate/
      workloaddelivery/
    adapters/
      k8s/
        auditevent/
        modeldelivery/
        nodecachesubstrate/
        ociregistry/
        ownedresource/
        sourceworker/
        storageprojection/
        uploadsession/
        uploadsessionstate/
      sourcefetch/
      sourcemirror/
        objectstore/
      modelformat/
      modelprofile/
        common/
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
    support/
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
- `cmd/ai-models-controller` и `cmd/ai-models-artifact-runtime` — два
  execution entrypoint.
- `cmd/` должен оставаться thin shell: env/argv parsing, exit codes, wiring
  into `internal/*`.
- `ai-models-artifact-runtime` больше не экспонирует legacy shell вроде
  `--snapshot-dir`; runtime contract режется по реальным current commands, а не
  по старым migration paths.

### Composition root и process glue

- `internal/bootstrap/` — composition root manager runtime.
- `internal/cmdsupport/` — только shared process-level glue.
- `bootstrap` владеет scheme assembly, `ctrl.Manager`, controller registration,
  health/ready checks и metrics wiring.
- `cmdsupport` не должен знать concrete adapters, payload models или
  source-specific orchestration.

### Domain

- `internal/domain/publishstate/` — publication lifecycle, terminal semantics,
  conditions and status projection.
- `internal/domain/ingestadmission/` — cheap source-agnostic fail-fast
  invariants before heavy byte path.

Domain не должен знать concrete Kubernetes objects, pod shaping, secret CRUD
или transport details.

### Application

- `internal/application/publishplan/` — execution planning:
  source worker vs upload session.
- `internal/application/publishobserve/` — observation mapping and
  status-mutation planning.
- `internal/application/sourceadmission/` — cheap preflight for `source.url`.
- `internal/application/publishaudit/` — append-only audit planning.
- `internal/application/deletion/` — delete-time policy seam.

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
  materialization/current-link helper surface, single-writer coordination,
  bounded eviction planning и module-owned maintenance loop;
- `internal/monitoring/catalogmetrics/` — Prometheus collectors over public
  `Model` / `ClusterModel` truth.

### Controllers

- `catalogstatus/` — owner publication lifecycle.
- `catalogcleanup/` — delete/finalizer owner.
- `nodecachesubstrate/` — owner controller for managed local storage substrate
  over `sds-node-configurator` / `sds-local-volume` CR boundaries.
- `workloaddelivery/` — owner controller for top-level workload annotations
  `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel`.

Controller package оправдан ownership, а не удобством чтения. Новый controller
package без нового owner — patchwork.

### Concrete adapters

`internal/adapters/k8s/*` держит concrete Kubernetes shaping и CRUD:

- `sourceworker/`
- `modeldelivery/`
- `nodecacheintent/`
- `nodecachesubstrate/`
- `uploadsession/`
- `uploadsessionstate/`
- `ociregistry/`
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
  shared store layout, marker parsing, отдельный consumer current-link helper,
  single-writer coordination и bounded scan/eviction planning не должны жить
  внутри `cmd/ai-models-artifact-runtime` как скрытый mini cache-manager;
- `modelpack/oci/` остаётся controller-owned OCI adapter boundary:
  native publish/remove over registry HTTP, manifest/config validation,
  inspect/materialize and layer/media-type handling не должны утекать в
  `publicationartifact/` или в worker shell;
- `sourcefetch/` остаётся boundary для source acquisition, archive inspection,
  remote summary extraction и object-source planning;
- `k8s/nodecachesubstrate/` держит только shaping и live-state extraction для
  внешних storage CR (`LVMVolumeGroupSet`, `LVMVolumeGroup`,
  `LocalStorageClass`) и не тянет туда runtime delivery policy;
- `k8s/nodecacheintent/` держит только concrete shaping/load для
  module-owned per-node `ConfigMap` с desired artifact set и не должен
  затягивать туда cache maintenance policy;
- `k8s/modeldelivery/` остаётся boundary для workload mutation и теперь держит
  module-managed local fallback volume injection отдельно от storage substrate
  CR shaping и отдельно от будущего workload-facing node-shared cache mount
  service;
- live `HuggingFace` publish path больше не держит локальный
  `workspace/model` fallback: canonical path — direct or mirrored object source,
  cluster-level default теперь `Direct`, а planning failure explicit;
- `uploadstaging/s3/` и `sourcemirror/objectstore/` не должны сталкиваться по
  семантике: staging — upload/session surface, mirror — persisted source ingest.

### Dataplane

- `internal/dataplane/publishworker/`
- `internal/dataplane/uploadsession/`
- `internal/dataplane/artifactcleanup/`

Это controller-owned one-shot runtimes. Их нельзя смешивать с reconciler code и
нельзя откатывать назад в backend scripts.

Live rules для `publishworker`:

- canonical publish paths — streaming/object-source first;
- successful upload/HF publish paths больше не идут через `checkpointDir`;
- valid upload path не должен silently деградировать в local materialization;
- consumer-side materialization остаётся отдельным runtime concern и живёт в
  `materialize-artifact`, а общий node-cache contract теперь живёт в
  `internal/nodecache`, а не в publish shell; bounded maintenance runtime тоже
  опирается на тот же package, а не собирает свою вторую cache semantics
  surface.

### Shared support

- `cleanuphandle/`
- `modelobject/`
- `resourcenames/`
- `testkit/`
- `uploadsessiontoken/`

`support/*` допустим только как shared helper layer. Если пакет решает policy,
runtime semantics или status logic, это уже не support.

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
  observation, fail-closed/runtime-error proofs, reconcile gate, status
  mutation и thin helper seam вместо одного mixed ensure-runtime file;
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
- `modelpack/oci` split на publish/materialize/validation/session shells,
  отдельные layer-matrix publish/validation/archive-helper proofs и thin
  file, registry-server, registry dispatch, registry upload-state and
  registry content-state helper seams вместо одного mixed helper shell.

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

## 5. Что надо удалять при следующем касании

- `MERGE ON TOUCH` micro-files, которые держат только одну константу, одну
  ошибку или один helper внутри уже понятной boundary.
- `DELETE ON SIGHT` новые local request wrappers и owner wrappers поверх
  shared contracts.
- `DELETE ON SIGHT` misleading names, которые конфликтуют с live boundaries.
- `DELETE ON SIGHT` новый controller package без нового owner.
- `DELETE ON SIGHT` package-local inventories наподобие `BRANCH_MATRIX.ru.md`.

## 6. Текущие hotspots

### `internal/controllers/catalogcleanup/`

- Пакет тяжёлый по реальной причине:
  delete lifecycle включает cleanup job, GC request и finalizer release.
- Рост сюда допустим только внутри того же owner.
- Всё, что не является owner-level delete orchestration, надо выносить в
  `application/deletion`, `resourcenames` или concrete adapters.

### `internal/adapters/sourcefetch/`

- Это всё ещё самый тяжёлый concrete adapter.
- Это допустимо, пока boundary остаётся только про source acquisition.
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

## 7. Текущий вердикт

На `2026-04-18` controller/runtime tree уже соответствует live reset baseline:

- native `ModelPack` publisher живёт в `modelpack/oci`;
- `HF` и canonical upload paths выровнены на streaming/object-source semantics;
- legacy backend/publication shell вычищен из live controller tree;
- current documentation and test evidence должны обслуживать только этот
  baseline, а не transition history.
