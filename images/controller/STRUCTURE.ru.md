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
  kitops.lock
  install-kitops.sh
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
      uploadstaging/
      auditsink/
    monitoring/
      catalogmetrics/
    publishedsnapshot/
    publicationartifact/
    controllers/
      catalogstatus/
      catalogcleanup/
    adapters/
      k8s/
        auditevent/
        objectstorage/
        ociregistry/
        ownedresource/
        sourceworker/
        uploadsession/
        uploadsessionstate/
        workloadpod/
      sourcefetch/
      modelformat/
      modelprofile/
        common/
        gguf/
        safetensors/
      modelpack/
        kitops/
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

### Composition root и process glue

- `internal/bootstrap/` — composition root manager runtime.
- `internal/cmdsupport/` — только shared process-level glue.
- `cmdsupport` нельзя снова превращать в место, которое знает concrete
  adapters, runtime payload models или source-specific env parsing.

### Domain

- `internal/domain/publishstate/` — publication lifecycle, observations,
  terminal semantics, status/conditions.
- `internal/domain/ingestadmission/` — дешёвые и source-agnostic fail-fast
  admission invariants до heavy byte path.

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
  В live tree здесь теперь один прямой `publishop.Request`.
  Старый `OperationContext` wrapper удалён как пустая оболочка, которая только
  плодила `request.Request.*`.
- `internal/ports/modelpack/` — replaceable `ModelPack` contract:
  publish/remove/materialize.
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

### Controllers

- `internal/controllers/catalogstatus/` — live owner publication lifecycle.
- `internal/controllers/catalogcleanup/` — delete/finalizer owner.

Controller package оправдан только ownership, а не тем, что код “проще читать”.
Если owner не меняется, перенос в новый controller package почти наверняка
будет patchwork.

### Concrete adapters

`internal/adapters/k8s/*` держит concrete Kubernetes shaping и CRUD:

- `sourceworker/`
- `uploadsession/`
- `uploadsessionstate/`
- `ociregistry/`
- `objectstorage/`
- `ownedresource/`
- `workloadpod/`
- `auditevent/`

Non-K8s adapters остаются отдельно:

- `sourcefetch/`
- `modelformat/`
- `modelprofile/*`
- `modelpack/kitops/`
- `modelpack/oci/`
- `uploadstaging/s3/`

Главное правило для adapters:

- не тащить сюда public/status policy;
- не заводить adapter-local request wrappers поверх уже существующих ports;
- не возвращать runtime proxy layers, если concrete adapter и так реализует
  shared contract напрямую.

### Dataplane

- `internal/dataplane/publishworker/`
- `internal/dataplane/uploadsession/`
- `internal/dataplane/artifactcleanup/`

Это controller-owned one-shot runtimes. Их нельзя смешивать с reconciler code и
нельзя откатывать назад в backend scripts.

### Shared support

- `cleanuphandle/`
- `modelobject/`
- `resourcenames/`
- `testkit/`

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
- `DELETE ON SIGHT` новый controller package без нового owner.
- `DELETE ON SIGHT` новые package-local inventories наподобие
  `BRANCH_MATRIX.ru.md`.
  Controller-level evidence уже централизована в `TEST_EVIDENCE.ru.md`.

## 6. Текущие findings по live tree

### `internal/controllers/catalogcleanup/` остаётся главным controller hotspot

- Пакет тяжёлый, но тяжёлый по реальной причине:
  delete lifecycle теперь включает cleanup job, GC request и finalizer release.
- Cleanup `Job` и DMCR GC request `Secret` теперь сходятся через один
  package-local owner-metadata seam и shared `resourcenames` policy, а не через
  две локальные raw label maps с теми же ключами.
- Delete apply path теперь ещё и строит локальный runtime один раз:
  owner/handle prerequisites больше не пересчитываются на каждом apply step, а
  finalizer release не перепарсивает cleanup annotation поверх уже наблюдённого
  handle.
- Delete reconcile path теперь и сам стал честнее на owner layer:
  observe -> decide -> apply больше не таскает разрозненные values между
  методами, а идёт через один package-local finalize flow.
- Upload-staging delete path больше не лезет в DMCR GC observation после
  завершённого cleanup job. Эта ветка остаётся только для backend artifact.
- Delete policy при этом тоже стала явнее на application layer:
  `internal/application/deletion/` теперь собирает finalize protocol через
  package-local step helpers для cleanup job progress и GC progress, а не через
  россыпь повторяющихся `FinalizeDeleteDecision{...}` веток.
- Следующий рост сюда допустим только если он остаётся внутри того же owner.
  Новый fake adapter package ситуацию не улучшит.

### `internal/adapters/sourcefetch/` остаётся самым большим concrete adapter

- Это нормально, пока boundary остаётся про source acquisition.
- Общий raw-stage upload/download glue теперь уже вынесен в локальный
  `rawstage.go`, поэтому provider files снова держат в основном
  URL/auth/metadata semantics, а не второй раз один и тот же object-storage
  handoff.
- `HuggingFace` branch теперь дополнительно выровнен по source-native path:
  package-local Go snapshot downloader живёт внутри того же adapter owner и не
  тащит Python/CLI toolchain, HF-specific public API или лишний runtime shell.
- Сюда нельзя складывать format validation, publish status или runtime policy.

### `internal/adapters/k8s/uploadsession/` и `internal/adapters/k8s/sourceworker/` уже выровнены по общему runtime contract

- Оба concrete adapters теперь принимают прямой `publishop.Request`.
- Upload session adapter теперь ещё и держит controller-owned phase sync
  methods для `publishing/completed/failed`; controller не должен лезть в
  `Secret` напрямую в обход этого runtime seam.
- Возвращение локального wrapper или отдельного mapping layer будет прямым
  регрессом структуры.

### `internal/adapters/modelformat/` надо держать жёстко format-centric

- Boundary оправдан detect/validate logic.
- Общий inspect/validate/select traversal теперь уже сжат в один package-local
  runner, но format files по-прежнему владеют только своими classification и
  required-file rules.
- Сюда нельзя тащить profiling, endpoint policy или backend-specific
  packaging exceptions.

### `internal/publicationartifact/` теперь честно описывает свой смысл

- Пакет больше не притворяется backend-facing слоем.
- В нём больше нет мёртвого `Request`.
- Здесь остаются только:
  - publication runtime result payload;
  - validation этого payload;
  - OCI artifact reference policy.

Если этот документ снова начнёт разрастаться в каталог на сотни строк, это
будет означать не “сложную архитектуру”, а то, что документ снова обслуживает
шум вместо границ.
