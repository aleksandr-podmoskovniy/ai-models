# Структура `images/controller`

Этот документ больше не притворяется инвентарём каждого файла.
Его задача проще и жёстче: зафиксировать живую package map controller runtime и
сразу сказать, что здесь нужно оставить, что нельзя раздувать и что надо
сливать или удалять при следующем касании.

Предыдущая версия на 1000+ строк сама стала шумом. Это уже было не описание
структуры, а защита каждого микрофайла по отдельности. Такой документ не
чистит дерево, а обслуживает его дробление.

Если каталог, пакет или файл нельзя защитить хотя бы по одному из оснований
ниже, его не надо оправдывать, его надо убрать:

1. отдельный execution entrypoint;
2. отдельный runtime contract или handoff model;
3. реальное переиспользование больше чем в одном live path.

## 1. Что это за дерево

`images/controller/` — корень phase-2 controller runtime внутри DKP module.
Это не:

- публичный API модуля;
- packaging внутреннего backend;
- общий toolbox для произвольных утилит;
- место для historical package names из старых refactor slices.

Текущее live дерево на уровне пакетов выглядит так:

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
    domain/publishstate/
    application/
      publishplan/
      publishobserve/
      deletion/
    ports/
      publishop/
      modelpack/
      uploadstaging/
    artifactbackend/
    publishedsnapshot/
    controllers/
      catalogstatus/
      catalogcleanup/
    adapters/
      k8s/
        objectstorage/
        ociregistry/
        ownedresource/
        sourceworker/
        uploadsession/
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

## 2. Что оставить

### Корень

- `KEEP` `README.md`, `STRUCTURE.ru.md`, `TEST_EVIDENCE.ru.md`:
  controller-level docs должны жить рядом с controller tree, а не расползаться
  в `docs/` и не размножаться по пакетам.
- `KEEP` `go.mod`, `go.sum`, `werf.inc.yaml`:
  это image-local build boundary, а не общерепозиторный Go/operator root.
- `RULE` `werf.inc.yaml` должен собирать final runtime images от module-local
  `images/distroless`, а не тянуть `base/distroless` напрямую.
- `KEEP` `kitops.lock`, `install-kitops.sh`:
  текущий `ModelPack` runtime принадлежит controller image, а не backend image.
- `RULE` отдельный `tools/` каталог ради одного installer script не нужен:
  такие tiny build-only seams надо binpack'ать рядом с lock file и `werf.inc.yaml`.
- `RULE` `KitOps` должен приходить в runtime через отдельный artifact stage, а
  не прятаться как побочный шаг внутри Go compilation stage.

### `cmd/`

- `KEEP` `cmd/ai-models-controller/`:
  это отдельный manager entrypoint.
- `KEEP` `cmd/ai-models-artifact-runtime/`:
  это отдельный one-shot runtime binary для `publish-worker`,
  `upload-session` и `artifact-cleanup`.
- `RULE` `cmd/` остаётся тонким shell:
  env/argv parsing, process exit codes, wiring into `internal/*`.
- `RULE` tiny command-only seams надо binpack'ать:
  отдельный файл ради одной константы, одной ошибки или одного локального
  dispatch helper не является архитектурной границей.

### Composition root и shared executable glue

- `KEEP` `internal/bootstrap/`:
  composition root manager runtime.
- `KEEP` `internal/cmdsupport/`:
  shared process-level glue для manager и runtime binary.
- `RULE` `internal/cmdsupport/` не должен знать concrete adapters или
  domain-specific result payloads:
  если helper нужен только `ai-models-artifact-runtime`, он живёт в `cmd/`
  этой binary boundary, а не в shared glue.
- `REJECT` возвращение generic пакета вроде `internal/app`:
  рядом уже есть `internal/application`, второй generic alias только портит
  карту дерева.

### Domain / application / ports

- `KEEP` `internal/domain/publishstate/`:
  publication lifecycle, terminal semantics, status/condition assembly и
  runtime observations должны оставаться domain seam.
- `KEEP` `internal/application/publishplan/`,
  `internal/application/publishobserve/` и
  `internal/application/deletion/`:
  use-case слой здесь реальный, а не декоративный.
  `deletion/` теперь владеет и delete-time DMCR garbage-collection policy, а
  не только созданием cleanup Job.
- `KEEP` `internal/ports/publishop/` и `internal/ports/modelpack/`:
  это live runtime contracts, а не adapter-local types.
- `KEEP` `internal/ports/uploadstaging/`:
  staging handoff для upload path теперь отдельный runtime contract.
  Его нельзя прятать внутрь `publishop`, потому что это уже другой storage
  lifecycle и другой cleanup surface.
- `KEEP` `internal/artifactbackend/` и `internal/publishedsnapshot/`:
  это разные handoff models. Backend transport contract и internal published
  snapshot нельзя смешивать в один generic пакет `publication`.

### Controllers

- `KEEP` `internal/controllers/catalogstatus/`:
  это текущий live owner publication lifecycle для `Model` и `ClusterModel`.
- `KEEP` `internal/controllers/catalogcleanup/`:
  это отдельный delete/finalizer owner.
  После перехода на internal DMCR он владеет не только cleanup Job, но и
  lifecycle garbage-collection request до снятия finalizer.
- `KEEP` `internal/controllers/catalogcleanup/job.go` внутри controller package:
  отдельный `adapters/k8s/cleanupjob` был бы fake boundary без второго
  потребителя.
- `REJECT` возврат `publishrunner`, `publicationops` или второй persisted bus
  только ради перемещения кода между папками:
  если ownership не меняется, rename пакета не является улучшением.

### Concrete adapters

- `KEEP` `internal/adapters/k8s/sourceworker/` и
  `internal/adapters/k8s/uploadsession/`:
  это concrete runtime adapters behind shared ports.
- `KEEP` `internal/adapters/k8s/objectstorage/`,
  `internal/adapters/k8s/ociregistry/`,
  `internal/adapters/k8s/ownedresource/`,
  `internal/adapters/k8s/workloadpod/`:
  эти пакеты оправданы только потому, что убирают реальное повторение между
  несколькими K8s adapters.
- `KEEP` `internal/adapters/sourcefetch/`, `internal/adapters/modelformat/`,
  `internal/adapters/modelprofile/*`, `internal/adapters/modelpack/kitops/`,
  `internal/adapters/uploadstaging/s3/`:
  это concrete non-K8s adapters. Их нельзя утаскивать в `publishworker` или
  `support/*`.
- `REJECT` adapter-local request wrappers, owner wrappers, local naming helpers
  и runtime proxy layers:
  всё это уже один раз было вычищено и не должно возвращаться под новыми
  именами.

### Dataplane

- `KEEP` `internal/dataplane/publishworker/`,
  `internal/dataplane/uploadsession/`,
  `internal/dataplane/artifactcleanup/`:
  это controller-owned one-shot runtimes.
- `REJECT` перенос этих runtime paths назад в backend scripts или смешивание их
  с reconciler packages:
  execution plane и control plane уже разведены и должны такими остаться.

### Shared support

- `KEEP` `internal/support/cleanuphandle/`,
  `internal/support/modelobject/`,
  `internal/support/resourcenames/`:
  эти helper packages реально разделяются несколькими live paths.
- `KEEP` `internal/support/testkit/`:
  shared scheme/object/fake-client fixtures допустимы именно как test-only
  слой.
- `REJECT` превращение `support/*` во второй business layer:
  если логика решает lifecycle policy, формат публикации, runtime observation
  или delete semantics, ей не место в `support/*`.

## 3. Что надо сливать или выбрасывать при следующем касании

- `MERGE ON TOUCH` micro-files внутри одного command/package boundary, если они
  держат только константы, ошибки или один локальный helper.
- `MERGE ON TOUCH` документацию, которая пытается объяснять каждый test file по
  отдельности.
  `STRUCTURE.ru.md` должен оставаться картой границ, а не second source tree.
- `DELETE ON SIGHT` package-local `BRANCH_MATRIX.ru.md` и подобные file-level
  inventories.
  Controller-level evidence уже централизована в `TEST_EVIDENCE.ru.md`.
- `DELETE ON SIGHT` новые generic package names вроде `publication`, `runtime`,
  `common`, `app`, если за ними не стоит отдельная архитектурная граница.
- `DELETE ON SIGHT` локальные `names.go`, `request.go`, `types.go`, если это
  просто зеркала уже существующих shared contracts.
- `REJECT` новый `cleanupjob` adapter package без второго live consumer.
- `REJECT` новый controller package только для того, чтобы назвать ту же
  логику по-новому; сначала должен появиться новый owner или новый contract.

## 4. Что сознательно выкинуто из этого документа

- Полный file-by-file inventory production и test файлов.
- Повторяющееся объяснение “почему не в соседнем пакете” там, где граница уже
  очевидна на уровне package map.
- Historical names и мёртвые refactor seams, которых больше нет в live tree.
- Попытка объяснить архитектуру через каждую вспомогательную константу.

Если этот документ снова начнёт расти в каталог на сотни строк, это не признак
лучшей структуры, а сигнал, что документ опять обслуживает шум.

## 5. Текущие жёсткие findings по live tree

Ниже не wishlist, а текущие hotspots по фактическому дереву на
`2026-04-10`.

### 1. `internal/controllers/catalogcleanup/` всё ещё главный controller hotspot

- Сейчас это самый тяжёлый controller package: примерно `836` non-test LOC.
- Причина роста не cosmetic:
  после internal DMCR slice delete lifecycle теперь реально включает
  `cleanup Job -> GC request -> GC complete -> remove finalizer`.
- Хорошая часть:
  сам `Reconcile` остаётся тонким, а decision table живёт в
  `application/deletion/`.
- Плохая часть:
  package уже больше не держит один generic `io.go`, но всё ещё тяжёлое:
  observation, apply path, status updates и K8s-specific resource shaping
  теперь честно разложены по `observe.go`, `apply.go`, `status.go`,
  `job.go` и `gc_request.go`.
- Вердикт:
  пакет оставляем, но это теперь первое место, куда нельзя добавлять новую
  delete-time branching.
  Если сюда пойдёт ещё рост, выносить надо не в новый fake adapter package, а
  в более узкие helpers внутри той же boundary или в `application/deletion/`,
  если меняется policy.

### 2. `internal/adapters/sourcefetch/` остаётся самым тяжёлым concrete adapter

- Пакет держит примерно `1024` non-test LOC и по-прежнему крупнейший во всём
  controller tree.
- Размер сам по себе тревожный, но boundary пока остаётся связной:
  remote ingest, provider-specific download и archive hardening действительно
  относятся к source acquisition.
- Вердикт:
  пакет пока оставляем, но запрещаем складывать сюда format validation,
  profiling, publication orchestration и status logic.
  При добавлении новых providers резать надо по provider/transport seams, а не
  бесконечно наращивать один пакет.

### 3. `internal/adapters/k8s/uploadsession/` теперь главный concrete K8s hotspot upload path

- Пакет уже держит примерно `676` non-test LOC и перегнал `sourceworker/`.
- Причина роста не случайная:
  session `Secret/Service/Ingress/Pod`, public URL projection, replay/reuse и
  object-storage staging wiring теперь действительно живут в одной adapter
  boundary.
- Вердикт:
  пакет оставляем, но запрещаем тащить сюда publication policy, staging cleanup
  semantics и source/profile logic.
  Если будет следующий рост, резать надо по object-shaping helpers внутри этой
  же boundary, а не возвращать fake runtime proxies или local request mirrors.

### 4. `internal/application/publishobserve/` большой, но это правильный рост

- Пакет держит примерно `570` non-test LOC.
- Это не повод дробить его на шумовые подпакеты:
  reconcile gate, runtime observation, staged-upload transition,
  status-mutation planning и runtime orchestration действительно составляют
  один application seam.
- Вердикт:
  пакет оставляем как единый use-case owner.
  Следующий рост допустим только пока он не тащит назад K8s object shaping,
  concrete Pod wiring или controller status persistence.

### 5. `internal/adapters/modelformat/` стал отдельным живым hotspot и это надо признать

- Пакет уже держит примерно `593` non-test LOC.
- Это не generic dump:
  detect/validate logic действительно образует один input-format adapter seam.
  После текущего corrective slice он хотя бы разложен по честным
  format-centric файлам:
  `common`, `safetensors`, `gguf`, `detect`, `validation`.
- Но здесь легко начать складывать то, чему не место рядом с validation:
  profiling, runtime metadata, source-specific heuristics и backend-specific
  packaging exceptions.
- Вердикт:
  boundary оставляем, но держим её жёстко format-centric.
  Любая логика про endpoint types, accelerator compatibility или publish result
  сюда попадать не должна.

### 6. `internal/adapters/k8s/objectstorage/` и `internal/adapters/uploadstaging/s3/` малы, но это уже не шум

- `internal/adapters/k8s/objectstorage/` держит примерно `126` non-test LOC.
- `internal/adapters/uploadstaging/s3/` держит примерно `211` non-test LOC.
- `internal/ports/uploadstaging/` держит примерно `58` non-test LOC.
- Это не временный слой ради красоты:
  staging-first upload path теперь реально использует отдельный storage
  contract, отдельный cleanup handle kind и отдельный runtime handoff между
  upload session и publish worker.
- Вердикт:
  эти seams оставляем.
  Их нельзя схлопывать обратно в `uploadsession/` или `cmdsupport/`, потому что
  тогда controller снова потеряет честную границу между upload edge, durable
  staging и publish execution.

## 6. Практическое правило на следующий slice

Если при следующем изменении возникает соблазн:

- добавить новый пакет ради одного helper-файла;
- добавить новый документ ради одного пакета;
- утащить domain/application policy в `support/*`;
- назвать пакет generic словом и надеяться, что “контекст всё объяснит”;
- оправдать отдельный файл только тем, что “так удобнее читать diff”;

значит, скорее всего, это не новая архитектурная граница, а шум. Его нужно
слить в уже существующую live boundary.
