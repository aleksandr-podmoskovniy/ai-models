# Структура `images/controller`

Этот документ фиксирует живую package map controller runtime и объясняет,
почему границы именно такие. Это не file-by-file inventory и не оправдание
каждого микрофайла.

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
- место для historical package names, которые уже потеряли свой смысл;
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
    controllers/
      catalogstatus/
      catalogcleanup/
      workloaddelivery/
    adapters/
      k8s/
        auditevent/
        modeldelivery/
        ociregistry/
        ownedresource/
        sourceworker/
        storageprojection/
        uploadsession/
        uploadsessionstate/
        workloadpod/
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
- `cmd/ai-models-controller` и `cmd/ai-models-artifact-runtime` — это два
  отдельных execution entrypoint.
- `cmd/` должен оставаться thin shell:
  env/argv parsing, exit codes, wiring into `internal/*`.
- В live tree `ai-models-controller/` уже разложен по отдельным shell-файлам:
  `env.go`, `config.go`, `resources.go`, `run.go`.
  Это важно, потому что controller runtime contract не должен снова
  схлопываться в giant `main.go`.
- `ai-models-artifact-runtime/` тоже уже не один dispatcher file:
  `dispatch.go` держит только command routing,
  `staging_config.go` — staging env contract,
  `materialize_coordination.go` — shared-cache coordination для materialize
  path.
  Это live execution shell того же runtime image, а не “временные helper files”.
- если entrypoint снова начинает смешивать:
  - env contract
  - quantity/resource parsing
  - bootstrap option shaping
  в одном `run.go`, это уже живой structural drift, а не harmless wiring.

### Composition root и process glue

- `internal/bootstrap/` — composition root manager runtime.
- `internal/cmdsupport/` — только shared process-level glue.
- В live tree `bootstrap.go` владеет ровно тем, чем и должен владеть
  production composition root:
  scheme assembly, `ctrl.Manager`, owner controller registration,
  health/ready checks и metrics collector wiring.
- Там же теперь централизованы manager-level controller defaults:
  bridged logger для `controller-runtime`, panic recovery, priority queue и
  enlarged cache sync timeout.
  Это production runtime shell, а не detail отдельного owner package.
- `cmdsupport` нельзя снова превращать в место, которое знает concrete
  adapters, runtime payload models или source-specific env parsing.
- Внутри `cmdsupport/` process/runtime helpers, env helpers и structured
  logging тоже не должны снова схлопываться в один oversized `common.go`.

### Domain

- `internal/domain/publishstate/` — publication lifecycle, observations,
  terminal semantics, status/conditions.
- `internal/domain/ingestadmission/` — дешёвые и source-agnostic fail-fast
  admission invariants до heavy byte path.
Внутри `publishstate/` policy validation тоже не должна снова смешивать
top-level policy evaluation, inferred model capability mapping и
normalization/intersection helpers в одном oversized `policy_validation.go`.

Domain не должен знать concrete Kubernetes objects, Pod shaping, Secret CRUD
или конкретный transport.

### Application

- `internal/application/publishplan/` — planning execution path:
  source worker vs upload session.
- `internal/application/publishobserve/` — orchestration runtime ports,
  observation mapping и status-mutation planning.
- `internal/application/sourceadmission/` — cheap preflight orchestration для
  `source.url`.
- `internal/application/publishaudit/` — append-only audit/event planning.
- `internal/application/deletion/` — delete-time policy seam:
  finalizer decision table, cleanup-job progress steps и GC follow-up policy.

Это реальные use-case seams. Их нельзя растворять ни в controllers, ни в
adapter packages.

### Ports и handoff models

- `internal/ports/publishop/` — shared runtime contract между control plane и
  concrete worker/session adapters.
  В live tree здесь остаются только прямые shared request/status/phase
  contracts.
  Старые пустые оболочки вроде `OperationContext` и мёртвый `publishop.Result`
  удалены, потому что они только плодили ложную shared boundary.
- `internal/ports/modelpack/` — replaceable `ModelPack` contract:
  publish/remove/materialize.
- `internal/ports/sourcemirror/` — durable source-ingest contract:
  immutable snapshot manifest, persisted mirror state и object-storage-backed
  retry boundary для remote sources.
- `internal/ports/uploadstaging/` — отдельный staging contract для upload path.
  В live tree он уже включает не только presign/complete shell, но и чтение
  server-side multipart manifest для resumability/state sync upload session.
- `internal/ports/auditsink/` — append-only audit sink contract.

Отдельно от ports остаются два handoff model packages:

- `internal/publishedsnapshot/` — полный immutable publication snapshot,
  который потом проецируется в status и delete lifecycle.
- `internal/publicationartifact/` — controller-owned artifact result payload
  плюс OCI destination reference policy.
  Это бывший `artifactbackend`; старое имя удалено, потому что пакет больше не
  описывает никакой backend-facing contract.
- `internal/monitoring/catalogmetrics/` — module-owned Prometheus collectors
  over public `Model` / `ClusterModel` truth:
  phase one-hot gauges, ready/validated booleans, small info metrics и
  published artifact size without parsing logs or exposing hidden
  controller/session internals.

`publishedsnapshot` и `publicationartifact` не надо сливать:
первый хранит полный internal publication snapshot, второй описывает более
узкий runtime payload/artifact-ref seam.
`monitoring/catalogmetrics` тоже не надо растворять в `bootstrap/`:
collector boundary — это отдельный runtime contract между manager cache и
platform observability surface.
Внутри `catalogmetrics/` listing Kubernetes objects, descriptor shell и
per-kind metric emission тоже не должны снова схлопываться обратно в один
oversized `collector.go`.

### Controllers

- `internal/controllers/catalogstatus/` — live owner publication lifecycle.
- `internal/controllers/catalogcleanup/` — delete/finalizer owner.
- `internal/controllers/workloaddelivery/` — owner controller for top-level
  workload annotations `ai.deckhouse.io/model` /
  `ai.deckhouse.io/clustermodel`, который мутирует только workloads с
  mutable `PodTemplateSpec` (`Deployment`, `StatefulSet`, `DaemonSet`,
  `CronJob`) и реиспользует shared `k8s/modeldelivery` service; generic
  workload delivery намеренно не уходит в admission webhook surface и вместо
  этого держит узкий watch scope: только opt-in/managed workloads и связанные
  `Model` / `ClusterModel`.

Controller package оправдан только ownership, а не тем, что код “проще читать”.
Если owner не меняется, перенос в новый controller package почти наверняка
будет patchwork.
Для secondary watches правило жёстче:
если mapping использует только metadata, watch не должен тащить full object
cache. Именно поэтому `catalogstatus` держит pod watch как metadata-only path.

### Concrete adapters

`internal/adapters/k8s/*` держит concrete Kubernetes shaping и CRUD:

- `sourceworker/`
- `modeldelivery/`
- `uploadsession/`
- `uploadsessionstate/`
- `ociregistry/`
- `storageprojection/`
- `ownedresource/`
- `workloadpod/`
- `auditevent/`

Non-K8s adapters остаются отдельно:

- `sourcefetch/`
- `sourcemirror/objectstore/`
- `modelformat/`
- `modelprofile/*`
- `modelpack/oci/`
- `uploadstaging/s3/`

Главное правило для adapters:

- не тащить сюда public/status policy;
- `k8s/modeldelivery/` остаётся reusable consumer-side runtime seam:
  он держит concrete `PodTemplateSpec` mutation service и render helpers
  поверх уже существующих `modelpack` и `ociregistry`, reuses user-provided
  workload storage mounted at `/data/modelcache`, включая cross-namespace
  read-only DMCR auth/CA projection в runtime namespace, topology checks для
  per-pod mounts / StatefulSet claim templates / direct shared PVC и RWX
  single-writer coordination прямо на shared cache root, а не invents новый
  inference-owned API,
  второй volume contract или отдельный auth shell;
- `modelpack/oci/` остаётся controller-owned OCI adapter boundary:
  native publish/remove over registry HTTP, published-artifact inspect,
  manifest/config validation и layer materialization не должны снова
  схлопываться обратно в один oversized `adapter.go` и не должны утекать ни в
  `publicationartifact/`, ни в `dataplane/publishworker/`;
- `modelprofile/safetensors/` остаётся concrete profile resolver, но внутри не
  должен снова смешивать top-level `Resolve`, checkpoint config parsing/value
  helpers и model-capability inference в одном oversized `profile.go`;
- `modelprofile/*` публикует platform-facing metadata о модели:
  semantic endpoint types допустимы, а runtime/topology guesswork нет.
  Deployment topology вроде `KubeRay` не должна masquerade as runtime
  compatibility модели;
- не заводить adapter-local request wrappers поверх уже существующих ports;
- не возвращать runtime proxy layers, если concrete adapter и так реализует
  shared contract напрямую.
- не держать misleading names:
  K8s package, который только проецирует env/volumes/secrets в pod, не должен
  называться как реальный storage adapter.

### Dataplane

- `internal/dataplane/publishworker/`
- `internal/dataplane/uploadsession/`
- `internal/dataplane/artifactcleanup/`

Это controller-owned one-shot runtimes. Их нельзя смешивать с reconciler code и
нельзя откатывать назад в backend scripts.
Внутри `publishworker/` top-level worker contract, HF-specific remote path,
upload path, raw provenance и profile/publish resolution тоже не должны снова
схлопываться обратно в один oversized `run.go`.
`materialize-artifact` остаётся рядом с ними в том же runtime image contract,
но не переезжает в controller manager:
это byte-path runtime, а не reconcile owner.

### Shared support

- `cleanuphandle/`
- `modelobject/`
- `resourcenames/`
- `testkit/`
- `uploadsessiontoken/`

`support/*` допустим только как shared helper layer. Если пакет решает policy,
runtime semantics или status logic, это уже не support.

## 3. Тестовое дерево и discipline

Тесты здесь считаются частью архитектуры, а не шумом вокруг production-кода.
Если test tree снова превращается в монолит, это такой же structural regession,
как fat adapter или controller-local god object.

Жёсткие правила:

- `_test.go` файлы в `images/controller/internal` живут под тем же LOC-budget,
  что и production files:
  - production: `tools/check-controller-loc.sh`
  - tests: `tools/check-controller-test-loc.sh`
  - budget: `< 350` строк без allowlist-first мышления
- tests режутся по decision surface, а не по “что удобно было положить рядом”:
  - runtime observe
  - status projection
  - owner reconcile
  - adapter IO contract
  - command/process shell
- helper-only test files допустимы только как thin shared seam внутри одного
  package, когда они реально обслуживают несколько decision files;
- если пакетный test file начинает смешивать:
  - разные lifecycle phases
  - разные runtime kinds
  - разные owners
  - разные transport APIs
  значит его надо делить, а не наращивать allowlist;
- канонический coverage inventory живёт только в `TEST_EVIDENCE.ru.md`.
  Новый branch matrix рядом с package создавать нельзя.

Текущий live pattern после refactor:

- `publishobserve` split на:
  - shared helpers
  - source-worker observation
  - upload-session observation
- `catalogstatus` split на:
  - source-worker/status projection
  - upload handoff/status sync
  - shared reconcile helpers/fakes
- `dataplane/uploadsession` split на:
  - handler helpers and tiny pure tests
  - session API
  - multipart API
  - expiry/abort semantics

## 4. Жёсткие правила на следующий refactor

- Не добавлять новый пакет ради одной пустой оболочки поверх уже живого
  контракта.
- Не возвращать generic names вроде `app`, `common`, `runtime`,
  `publication`, если за ними не стоит новая граница ownership.
- Не плодить локальные mirrors существующих shared contracts.
  Если есть `publishop.Request`, adapter не должен изобретать второй request
  type только ради “удобных имён”.
- Не держать в shared port package мёртвые handoff types.
  Если payload реально живёт в `publishedsnapshot` или `publicationartifact`,
  его не надо дублировать третьим `Result` в `publishop`.
- Не смешивать handoff models:
  `publishedsnapshot` и `publicationartifact` остаются разными пакетами по
  разным причинам.
- Не тащить concrete K8s object shaping в `application/*`.
- Не тащить lifecycle policy в `support/*`.
- Не вводить второй persisted bus или второй lifecycle source of truth между
  controller, upload session и publish worker.

## 5. Что надо удалять при следующем касании

- `MERGE ON TOUCH` micro-files, которые держат только одну константу,
  одну ошибку или один helper внутри уже понятной boundary.
- `DELETE ON SIGHT` новые local request wrappers и owner wrappers поверх
  `publishop.Request`, `publishop.Owner` и других shared contracts.
- `DELETE ON SIGHT` исторические имена, если их исходный смысл уже умер.
  Именно так в этом slice был удалён `artifactbackend`.
- `DELETE ON SIGHT` package names, которые сталкиваются с уже существующими
  live boundaries.
  Именно так `k8s/objectstorage` был заменён на `k8s/storageprojection`:
  старое имя конфликтовало с реальными object-storage adapters
  `uploadstaging/s3` и `sourcemirror/objectstore`.
- `DELETE ON SIGHT` новый controller package без нового owner.
- `DELETE ON SIGHT` новые package-local inventories наподобие
  `BRANCH_MATRIX.ru.md`.
  Controller-level evidence уже централизована в `TEST_EVIDENCE.ru.md`.

## 6. Текущие findings по live tree

### `internal/controllers/catalogcleanup/` остаётся главным controller hotspot

- Пакет тяжёлый по реальной причине:
  delete lifecycle включает cleanup job, GC request и finalizer release.
- Рост сюда допустим только внутри того же owner.
  Новый helper/controller package без нового owner будет patchwork.
- Всё, что не является owner-level delete orchestration, надо выносить
  обратно в `application/deletion`, `resourcenames` или concrete adapters.

### `internal/adapters/sourcefetch/` остаётся самым большим concrete adapter

- Это допустимо, пока boundary остаётся только про source acquisition.
- Сюда нельзя складывать format validation, publish status или runtime policy.
- Любой новый кусок здесь должен либо усиливать acquisition path, либо
  уходить в отдельный port/adapter seam вроде `sourcemirror/`.
- Даже внутри `sourcefetch/` archive dispatch, extraction safety и
  single-file materialization не должны снова схлопываться в один `archive.go`:
  routing, extraction и file materialization должны оставаться хотя бы в
  соседних files того же acquisition package.
- То же правило для HF path:
  info API helpers, snapshot orchestration и local staging/materialization не
  должны снова схлопываться обратно в один `huggingface.go`.

### `internal/adapters/k8s/uploadsession/` и `internal/adapters/k8s/sourceworker/` уже выровнены по общему runtime contract

- Оба concrete adapters теперь принимают прямой `publishop.Request`.
- Upload session adapter теперь ещё и держит controller-owned phase sync
  methods для `publishing/completed/failed`; controller не должен лезть в
  `Secret` напрямую в обход этого runtime seam.
- Внутри `uploadsession/` orchestration, secret lifecycle и handle/token
  projection больше не должны снова схлопываться в один oversized `service.go`:
  `GetOrCreate` остаётся thin, а concrete lifecycle/handle shaping живут в
  соседних files того же package.
- Внутри `sourceworker/` pod orchestration, runtime env/volume shaping и
  source-specific argv тоже не должны снова схлопываться в один oversized
  `build.go`: orchestration остаётся thin, а pod shaping и source arg shaping
  живут в соседних files того же package.
- Возвращение локального wrapper или отдельного mapping layer будет прямым
  регрессом структуры.

### `internal/controllers/catalogstatus/` теперь жёстче режет watch pressure

- Pod secondary watch здесь нужен только как enqueue trigger по owner metadata.
- Поэтому live setup использует metadata-only watch вместо полного pod cache.
- Возврат на full pod watch без новой live потребности в `spec`/`status` будет
  production regression по памяти и cache noise.

### `internal/adapters/k8s/storageprojection/` должен остаться тупым projection glue

- Этот пакет существует только для env/volume projection object-storage creds и
  CA в pod spec.
- Он не делает IO, multipart, mirror state или bucket lifecycle.
- Любая попытка тащить сюда реальные object-storage операции снова создаст
  structural collision с `uploadstaging/s3` и `sourcemirror/objectstore`.

### `internal/ports/sourcemirror/` и `internal/adapters/sourcemirror/objectstore/` фиксируют новый ingest boundary

- `sourcemirror` появился не ради новой абстракции “на будущее”, а потому что
  restart-safe source ingest нельзя дальше размазывать между `sourcefetch/`,
  `publishworker/` и raw staging objects без отдельного persisted contract.
- Port держит только:
  - snapshot locator;
  - immutable manifest;
  - persisted phase/file state;
  - store interface.
- Object-store adapter реализует этот contract поверх уже существующего
  object-storage substrate, не таща policy обратно в S3 adapter и не смешивая
  mirror state с upload-session multipart state.
- `sourcefetch` теперь использует эту boundary уже не только для JSON state,
  но и для resumable mirror bytes:
  - HTTP `Range` resume against source
  - object-storage multipart upload
  - local materialization уже из mirror, а не из единственной pod-local truth
- Следующий slice может добавлять parallelism и throughput tuning, но не
  должен снова уничтожать эту boundary.

### `internal/adapters/modelformat/` надо держать жёстко format-centric

- Boundary оправдан detect/validate logic.
- Общий inspect/validate/select traversal теперь уже сжат в один package-local
  runner, но format files по-прежнему владеют только своими classification и
  required-file rules.
- Сюда нельзя тащить profiling, endpoint policy или backend-specific
  packaging exceptions.

### `internal/publicationartifact/` теперь честно описывает свой смысл

- Здесь остаются только:
  - publication runtime result payload;
  - validation этого payload;
  - OCI artifact reference policy.
- Если package снова начнёт принимать backend-specific semantics, это будет
  прямой structural regression.

## 7. Сверка с production patterns `virtualization`

### Что совпадает с production pattern

- Отдельный composition root для controller manager уже есть:
  `cmd/ai-models-controller/*` собирает config/runtime shell, а
  `internal/bootstrap/` регистрирует owners, probes и metrics.
  По обязанностям это тот же production concern, что и
  `cmd/virtualization-controller/main.go`, только без giant-shell drift.
- Owner controllers уже вырезаны по ownership, как и в
  `../virtualization/images/virtualization-artifact/pkg/controller/<owner>`:
  `catalogstatus/`, `catalogcleanup/`, `workloaddelivery/`.
- Watch/indexer logic держится рядом с owner setup, а не размазывается по
  случайным helpers:
  это совпадает с production-паттерном `workload-updater` и controller-local
  `internal/watcher` packages в `virtualization`.
- Manager runtime теперь также держит production controller defaults
  централизованно, а не оставляет их implicit defaults:
  logger bridge, panic recovery, priority queue и cache sync timeout задаются
  в composition root так же осознанно, как в `virtualization` они задаются на
  controller setup path.
- Metrics collector живёт отдельной runtime boundary, как и
  `../virtualization/images/virtualization-artifact/pkg/monitoring/metrics/*`:
  list/read path, metric descriptors и emission split остаются не в
  reconciler code.

### Что отличается намеренно и почему это не drift

- `ai-models` держит runtime code под `internal/`, а не под `pkg/`, потому что
  это module-local executable surface, а не экспортируемая library boundary.
- `ai-models` жёстче режет command shell, чем текущий
  `virtualization-controller/main.go`:
  env parsing, resource parsing, bootstrap wiring и artifact-runtime dispatch
  уже вынесены в соседние files.
  Для этого репозитория это production improvement, а не отклонение вниз.
- `ai-models` оставляет явные `application/`, `ports/`, `adapters/`,
  `dataplane/` seams между controller owners и byte-path runtimes.
  В `virtualization` часть похожих ролей живёт controller-local под
  `pkg/controller/<owner>/internal`, но здесь shared publication/upload
  contracts реально переиспользуются несколькими owners и runtime paths, так
  что отдельные cross-owner boundaries оправданы.
- `catalogmetrics/` остаётся одним package вместо дерева per-kind collectors,
  потому что live public truth surface здесь узкая:
  только `Model` и `ClusterModel`.
  Раздробление в `metrics/model` / `metrics/clustermodel` сейчас было бы
  premature patchwork.
- Отдельный global package вроде `pkg/controller/watchers` из `virtualization`
  здесь пока не нужен:
  текущие watch paths не имеют достаточного cross-owner reuse и лучше остаются
  рядом со своим owner.

### Текущий вердикт

- На `2026-04-16` controller/runtime code не требует forced rewiring под
  `virtualization`, но уже потребовал два bounded hardening changes:
  - explicit manager/controller defaults в composition root;
  - metadata-only pod watch для `catalogstatus`.
- Если появится реальное cross-owner reuse watchers/metrics/service seams, его
  надо будет выносить так же жёстко, как это сделано в `virtualization`, но
  делать это заранее “ради похожести” нельзя.

Если этот документ снова начнёт разрастаться в каталог на сотни строк, это
будет означать не “сложную архитектуру”, а то, что документ снова обслуживает
шум вместо границ.
