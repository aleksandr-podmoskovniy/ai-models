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
    artifactbackend/
    publishedsnapshot/
    controllers/
      catalogstatus/
      catalogcleanup/
    adapters/
      k8s/
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
- `KEEP` `internal/ports/publishop/` и `internal/ports/modelpack/`:
  это live runtime contracts, а не adapter-local types.
- `KEEP` `internal/artifactbackend/` и `internal/publishedsnapshot/`:
  это разные handoff models. Backend transport contract и internal published
  snapshot нельзя смешивать в один generic пакет `publication`.

### Controllers

- `KEEP` `internal/controllers/catalogstatus/`:
  это текущий live owner publication lifecycle для `Model` и `ClusterModel`.
- `KEEP` `internal/controllers/catalogcleanup/`:
  это отдельный delete/finalizer owner.
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
- `KEEP` `internal/adapters/k8s/ociregistry/`,
  `internal/adapters/k8s/ownedresource/`,
  `internal/adapters/k8s/workloadpod/`:
  эти пакеты оправданы только потому, что убирают реальное повторение между
  несколькими K8s adapters.
- `KEEP` `internal/adapters/sourcefetch/`, `internal/adapters/modelformat/`,
  `internal/adapters/modelprofile/*`, `internal/adapters/modelpack/kitops/`:
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
`2026-04-07`.

### 1. `internal/controllers/catalogstatus/` остаётся главным structural hotspot

- Сейчас это самый тяжёлый controller package: примерно `562` non-test LOC.
- После последних corrective cuts отсюда уже ушли runtime result decode,
  upload-expiration policy, reconcile entry/skip gate и runtime
  source-vs-upload orchestration вместе со status-mutation planning в
  `application/publishobserve/`, но пакет всё ещё держит status persistence
  shell.
- Вердикт:
  пакет оставляем, но это первое место, куда нельзя бездумно добавлять новую
  business branching. Следующий рост должен уходить в domain/application/ports,
  а не в reconciler.

### 2. `internal/adapters/sourcefetch/` велик, но пока ещё защищаем

- Это самый крупный concrete adapter package: примерно `1019` non-test LOC.
- Размер сам по себе уже тревожный, но boundary пока остаётся связной:
  remote ingest, provider-specific download и archive hardening действительно
  относятся к source acquisition.
- Вердикт:
  пакет пока оставляем, но запрещаем складывать сюда format validation,
  profiling и worker orchestration. При добавлении новых providers резать надо
  по provider/transport seams, а не бесконечно наращивать один пакет.

### 3. `internal/adapters/k8s/sourceworker/` и `uploadsession/` ещё живы, но уже на грани

- `sourceworker/` держит примерно `571` non-test LOC.
- `uploadsession/` держит примерно `554` non-test LOC.
- Оба пакета всё ещё оправданы:
  каждый реализует один concrete runtime port и владеет своей группой K8s
  supplements.
- Дублирующий runtime options contract уже вынесен в
  `internal/adapters/k8s/workloadpod/`; возвращать локальные копии этих полей
  назад в adapters нельзя.
- Вердикт:
  новые фичи сюда вносить можно только через shared helpers или прямой binpack
  fake seams. Нельзя возвращать adapter-local request models, proxy runtimes,
  local naming policy и предварительные replay-only read branches.

## 6. Практическое правило на следующий slice

Если при следующем изменении возникает соблазн:

- добавить новый пакет ради одного helper-файла;
- добавить новый документ ради одного пакета;
- утащить domain/application policy в `support/*`;
- назвать пакет generic словом и надеяться, что “контекст всё объяснит”;
- оправдать отдельный файл только тем, что “так удобнее читать diff”;

значит, скорее всего, это не новая архитектурная граница, а шум. Его нужно
слить в уже существующую live boundary.
