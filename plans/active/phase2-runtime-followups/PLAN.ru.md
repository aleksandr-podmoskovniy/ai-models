# PLAN

## Current phase

Этап 2. Follow-up maintenance of the phase-2 model-catalog runtime after the
large corrective rebase bundle was completed and archived.

## Orchestration

- mode: `solo`
- reason:
  - задача structural and procedural;
  - архитектурный baseline уже зафиксирован;
  - delegation here would add latency, not signal.

## Current baseline

Что считаем уже landed и не пересобираем в этом bundle:

- controller-owned Go-first phase-2 runtime;
- internal `DMCR` as publication backend;
- immutable OCI `ModelPack` as canonical published artifact;
- active public source contract:
  - `source.url` for `HuggingFace`
  - `source.upload`
- current controller structure and test methodology fixed in:
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`

Архив engineering history:

- `plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/`

## Open workstreams carried forward

Следующие bounded slices должны жить уже в этом bundle, а не возобновлять
старый giant log.

Текущие живые направления:

1. Runtime delivery wiring
   - consumer-side use of `materialize-artifact`
   - read-only DMCR auth projection
   - local model path contract for runtimes

2. `HuggingFace` ingest hardening
   - native downloader/cache semantics
   - current local-first transient download path must be replaced by a durable
     source-mirror seam
   - bounded shared cache remains optional only as a later optimization
   - benign alternative export artifacts from HF repos must not fail a valid
     `Safetensors` publication path
   - the same seam should stay reusable for future non-HF sources such as
     Ollama-like registries

3. Ongoing structural hygiene
   - no return of generic HTTP source
   - no new controller or test monoliths
   - no drift between docs, bundle and live tree

4. Live validation hardening
   - second HF smoke against a repo different from the original phi test
   - controller/runtime bugs found by cluster smoke must become bounded
     corrective slices with focused regressions
   - current public HF source contract must be checked on a small official
     `Gemma 4` checkpoint before any API redesign around `repoID + revision`
   - live `Gemma 4` smoke must also confirm that published `ModelPack`
     contains real model bytes in `DMCR`, not only manifest/config shells

5. Structural package-map hygiene
   - remove misleading package names that collide with already existing live
     boundaries
   - keep `STRUCTURE.ru.md` as a live package map, not as a historical refactor
     diary
   - keep support/adapters package inventory aligned with the current tree

## Slice 1. Archive giant active bundle

Цель:

- убрать historical corrective-rebase bundle из `plans/active` и перенести его
  в `plans/archive/2026`.

Артефакты:

- archived predecessor bundle under:
  - `plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/`

Проверки:

- `find plans/active -maxdepth 2 -type f | sort`
- `find plans/archive/2026 -maxdepth 2 -type d | sort`

## Slice 2. Create compact canonical continuation bundle

Цель:

- создать новый короткий active bundle, который несёт только current baseline,
  open workstreams и validation expectations.

Артефакты:

- `plans/active/phase2-runtime-followups/TASK.ru.md`
- `plans/active/phase2-runtime-followups/PLAN.ru.md`

Проверки:

- manual sanity check that the new bundle does not re-copy old giant history

## Slice 3. Fix planning hygiene docs

Цель:

- закрепить правило: oversized active bundle архивируется и заменяется новым
  compact continuation bundle.

Артефакты:

- `plans/README.md`

Проверки:

- `sed -n '1,240p' plans/README.md`

## Slice 4. Validate second HF smoke and harden benign side-artifact handling

Цель:

- прогнать живой smoke publish для другого небольшого HF checkpoint;
- убедиться, что `Safetensors` ingest не ломается на benign alternative export
  artifacts вроде `onnx/`;
- если такой bug найден, исправить file-selection/validation path и покрыть
  его регрессией.

Артефакты:

- updated `images/controller/internal/adapters/modelformat/*`
- updated tests around remote selection / local validation
- updated live evidence if the cluster run exposes a real defect

Проверки:

- focused `go test` for `internal/adapters/modelformat`
- `make verify`
- live cluster smoke result recorded in the workstream

## Rollback point

Если новая planning surface окажется неудачной:

1. удалить `plans/active/phase2-runtime-followups/`;
2. переместить archived predecessor обратно в `plans/active/`;
3. откатить изменение `plans/README.md`.

## Slice 5. Validate current HF source contract on official Gemma 4

Цель:

- проверить live cluster на current user-facing manifest shape для
  `HuggingFace`;
- прогнать smoke publish для official small `Gemma 4` checkpoint;
- по факту зафиксировать working manifest или bounded defect.

Артефакты:

- updated active bundle notes for the live smoke result
- optional live evidence update if the cluster run exposes a real defect

Проверки:

- `kubectl get module ai-models`
- `kubectl -n d8-ai-models get deploy ai-models ai-models-controller dmcr`
- live `Model` apply/watch result in `ai-models-smoke`

## Slice 6. Fix empty-layer `KitOps` publication defect

Цель:

- исправить live defect, при котором publish path доходит до `Ready`, но
  публикует в `DMCR` пустой weight-layer вместо реального model filesystem;
- сохранить bounded byte path без дополнительной полной копии модели.

Артефакты:

- updated `images/controller/internal/adapters/modelpack/kitops/adapter.go`
- focused regression in `images/controller/internal/adapters/modelpack/kitops/adapter_test.go`
- updated live evidence for the empty-layer defect

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/kitops`
- `cd images/controller && go test ./internal/adapters/modelpack/... ./cmd/ai-models-artifact-runtime/...`
- `git diff --check`

## Slice 7. Introduce durable source-mirror seam

Цель:

- зафиксировать target architecture для resumable source ingest;
- ввести явный port/adapter split для source mirror manifest/state storage;
- не размазывать будущий resume logic по `sourcefetch` и `publishworker`.

Артефакты:

- active bundle decision note for durable source mirror
- new `internal/ports/sourcemirror/*`
- first object-storage-backed adapter for manifest/state persistence
- first `HuggingFace` integration that persists mirror manifest/state and hands
  mirror prefix ownership to the existing backend cleanup path
- structure/evidence docs updated for the new seam

Проверки:

- focused `go test` for the new `sourcemirror` packages
- `make verify`
- `git diff --check`

## Slice 8. Land resumable mirror-byte transport

Цель:

- перестать использовать pod-local snapshot как единственную truth для remote
  bytes;
- mirror `HuggingFace` files into object storage before local materialization;
- продолжать download после restart через multipart upload state плюс HTTP
  `Range` resume.

Артефакты:

- `sourcefetch` resumable mirror transport split into state/transport files
- `publishworker` raw provenance switched to durable source mirror for remote
  sources
- focused regressions for:
  - source-mirror-backed `FetchRemoteModel`
  - resumed multipart upload from a persisted byte offset

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`
- `make verify`
- `git diff --check`

## Slice 9. Remove naming collisions from the controller package map

Цель:

- убрать live naming collision between K8s pod-projection glue and real
  object-storage adapters;
- выровнять `STRUCTURE.ru.md` под текущее дерево без выпавших support
  packages и без лишнего historical noise.

Артефакты:

- `internal/adapters/k8s/storageprojection/*` replacing
  `internal/adapters/k8s/objectstorage/*`
- imports and runtime options updated to the renamed package
- `images/controller/STRUCTURE.ru.md` simplified and aligned with the live tree

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/workloadpod ./internal/controllers/catalogstatus ./internal/controllers/catalogcleanup ./internal/bootstrap`
- `make verify`
- `git diff --check`

## Slice 10. Remove dead shared-result collision from `publishop`

Цель:

- убрать мёртвый `publishop.Result`, который больше не используется в live
  runtime path;
- не оставлять в shared port package третью `Result`-структуру рядом с живыми
  `publishedsnapshot.Result` и `publicationartifact.Result`.

Артефакты:

- simplified `internal/ports/publishop/operation_contract.go`
- aligned `internal/ports/publishop/operation_contract_test.go`
- updated `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/ports/publishop ./internal/application/publishplan ./internal/controllers/catalogstatus ./internal/adapters/k8s/sourceworker`
- `make verify`
- `git diff --check`

## Final validation

- `find plans/active -maxdepth 2 -type f | sort`
- `find plans/archive/2026 -maxdepth 2 -type d | sort`
- `rg -n "rebase-phase2-publication-runtime-to-go-dmcr-modelpack" --glob '!plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/**'`
- `git diff --check`
