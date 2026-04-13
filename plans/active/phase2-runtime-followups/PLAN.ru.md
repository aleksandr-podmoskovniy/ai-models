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
   - bounded shared cache strategy
   - future durable mirror only if product signal justifies it
   - benign alternative export artifacts from HF repos must not fail a valid
     `Safetensors` publication path

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

## Final validation

- `find plans/active -maxdepth 2 -type f | sort`
- `find plans/archive/2026 -maxdepth 2 -type d | sort`
- `rg -n "rebase-phase2-publication-runtime-to-go-dmcr-modelpack" --glob '!plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/**'`
- `git diff --check`
