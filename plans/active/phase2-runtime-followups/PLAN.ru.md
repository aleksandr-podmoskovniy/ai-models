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

1. Runtime delivery closure in-repo
   - consumer-side use of `materialize-artifact`
   - read-only DMCR auth projection
   - fixed local cache-root contract for runtimes
   - concrete reusable `PodTemplateSpec` mutation service

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
   - keep `cmd/*` entrypoints thin: env contract, quantity parsing and bootstrap
     composition must not collapse back into one oversized `run.go`

6. Consumer-module adoption outside `ai-models`
   - in-repo runtime delivery seam must stay reusable and concrete
   - future runtime modules should add only thin overlays over
     `k8s/modeldelivery`, not reimplement delivery

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

## Slice 11. Restore custom-CA trust for source-mirror multipart uploads

Цель:

- убрать regression, при котором presigned multipart upload в source mirror
  bypass'ит configured S3 CA trust;
- не использовать `http.DefaultClient` для presigned `UploadPart`, если
  upload-staging adapter уже владеет CA-aware HTTP transport.

Артефакты:

- `internal/adapters/uploadstaging/s3/adapter.go` exposes its CA-aware HTTP client
- `internal/dataplane/publishworker/rawstage.go` propagates that client into
  `sourcefetch.SourceMirrorOptions`
- `internal/adapters/sourcefetch/huggingface_mirror_transport.go` uses the
  propagated client for presigned multipart `PUT`
- regressions for:
  - custom-CA TLS presigned upload endpoint
  - source-mirror wiring from upload-staging client into mirror options

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/uploadstaging/s3 ./internal/dataplane/publishworker`
- `make verify`
- `git diff --check`

## Slice 22. Split catalogmetrics collector internals

Цель:

- убрать ещё один локальный монолит из controller runtime tree;
- не оставлять `internal/monitoring/catalogmetrics/collector.go` местом, где
  одновременно живут descriptor shell, Kubernetes list paths и per-kind metric
  emission.

Артефакты:

- `internal/monitoring/catalogmetrics/collector.go` as thin collector shell
- `internal/monitoring/catalogmetrics/collect.go` for list/read paths
- `internal/monitoring/catalogmetrics/report.go` for metric emission helpers
- updated `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/monitoring/catalogmetrics`
- `make verify`
- `git diff --check`

## Slice 23. Split publishworker runtime internals

Цель:

- убрать следующий локальный монолит из dataplane runtime tree;
- не оставлять `internal/dataplane/publishworker/run.go` местом, где
  одновременно живут worker-level contract shell, HF-specific remote path и
  profile/publish resolution.

Артефакты:

- `internal/dataplane/publishworker/run.go` as thin worker contract shell
- `internal/dataplane/publishworker/huggingface.go` for HF fetch/publish path
- `internal/dataplane/publishworker/profile.go` for profile resolution and
  publish handoff
- updated `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker`
- `make verify`
- `git diff --check`

## Slice 24. Split Safetensors profile resolver internals

Цель:

- убрать следующий локальный монолит из concrete modelprofile tree;
- не оставлять `internal/adapters/modelprofile/safetensors/profile.go` местом,
  где одновременно живут top-level `Resolve`, checkpoint config parsing/value
  helpers и capability inference.

Артефакты:

- `internal/adapters/modelprofile/safetensors/profile.go` as thin orchestration
- `internal/adapters/modelprofile/safetensors/config.go` for config/value
  parsing helpers
- `internal/adapters/modelprofile/safetensors/detect.go` for family/context/
  precision/quantization/parameter inference helpers
- updated `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelprofile/safetensors`
- `make verify`
- `git diff --check`

## Slice 25. Split publishstate policy validation internals

Цель:

- убрать следующий локальный монолит из domain tree;
- не оставлять `internal/domain/publishstate/policy_validation.go` местом,
  где одновременно живут policy evaluation, inferred model capability mapping
  и normalization/intersection helpers.

Артефакты:

- `internal/domain/publishstate/policy_validation.go` as top-level policy
  evaluation shell
- `internal/domain/publishstate/policy_infer.go` for inferred model type and
  endpoint capability mapping
- `internal/domain/publishstate/policy_normalize.go` for normalization and
  set-intersection helpers
- updated `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/domain/publishstate`
- `make verify`
- `git diff --check`

## Slice 12. Split oversized controller entrypoint shell

Цель:

- не оставлять `cmd/ai-models-controller/run.go` местом, где снова смешаны:
  - env contract
  - flag parsing
  - resource parsing
  - bootstrap option shaping;
- вернуть `cmd/` к defendable thin-shell structure.

Артефакты:

- `cmd/ai-models-controller/env.go`
- `cmd/ai-models-controller/config.go`
- `cmd/ai-models-controller/resources.go`
- simplified `cmd/ai-models-controller/run.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller`
- `make verify`
- `git diff --check`

## Slice 13. Split `uploadsession` service monolith

Цель:

- не оставлять `internal/adapters/k8s/uploadsession/service.go` местом, где
  одновременно живут:
  - request orchestration
  - session secret lifecycle
  - stale-secret recreation
  - explicit expiration sync
  - upload handle/token projection;
- сохранить один concrete adapter package без возврата controller-level logic
  к прямой работе с `Secret`.

Артефакты:

- simplified `internal/adapters/k8s/uploadsession/service.go`
- `internal/adapters/k8s/uploadsession/lifecycle.go`
- `internal/adapters/k8s/uploadsession/handle.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession`
- `make verify`
- `git diff --check`

## Slice 14. Split `sourcefetch` archive/materialization monolith

Цель:

- не оставлять `internal/adapters/sourcefetch/archive.go` местом, где
  одновременно живут:
  - archive dispatch
  - tar/zip extraction safety
  - extracted-root normalization
  - single-file materialization
  - GGUF/file IO helpers;
- сохранить acquisition semantics без изменения public/runtime contract.

Артефакты:

- simplified `internal/adapters/sourcefetch/archive.go`
- `internal/adapters/sourcefetch/archive_extract.go`
- `internal/adapters/sourcefetch/materialize.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch`
- `make verify`
- `git diff --check`

## Slice 15. Split `sourcefetch` HuggingFace monolith

Цель:

- не оставлять `internal/adapters/sourcefetch/huggingface.go` местом, где
  одновременно живут:
  - HF info API helpers
  - mirror-or-local snapshot acquisition orchestration
  - snapshot staging/materialization;
- сохранить текущий live HF ingest contract without API/runtime drift.

Артефакты:

- simplified `internal/adapters/sourcefetch/huggingface.go`
- `internal/adapters/sourcefetch/huggingface_info.go`
- `internal/adapters/sourcefetch/huggingface_snapshot.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch`
- `make verify`
- `git diff --check`

## Slice 16. Land reusable runtime delivery wiring

Цель:

- закрыть open workstream, где `materialize-artifact` уже существует, но
  consumer-side wiring отсутствует;
- не изобретать concrete inference consumer inside `ai-models`, а собрать
  reusable K8s adapter над already-landed materializer и OCI auth projection;
- зафиксировать stable local cache-root contract for runtimes.

Артефакты:

- updated `internal/ports/modelpack/*` for stable materialized model path contract
- updated `internal/adapters/modelpack/oci/*` to enforce that contract
- new `internal/adapters/k8s/modeldelivery/*`
- updated `images/controller/STRUCTURE.ru.md`
- updated `images/controller/README.md`
- updated `docs/README.md`
- updated `docs/README.ru.md`
- updated `docs/CONFIGURATION.md`
- updated `docs/CONFIGURATION.ru.md`
- updated `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci ./internal/adapters/k8s/modeldelivery ./cmd/ai-models-artifact-runtime`
- `make verify`
- `git diff --check`

## Slice 17. Split OCI materializer internals after runtime delivery landing

Цель:

- не оставлять `internal/adapters/modelpack/oci/materialize.go` новым локальным
  монолитом сразу после landing runtime delivery contract;
- вынести в отдельные files distinct concerns:
  - stable materialized-path contract normalization
  - destination safe-swap
  - marker-based reuse
- удержать `STRUCTURE.ru.md` в синхроне с реальным package tree.

Артефакты:

- simplified `internal/adapters/modelpack/oci/materialize.go`
- new `internal/adapters/modelpack/oci/materialize_contract.go`
- new `internal/adapters/modelpack/oci/materialize_destination.go`
- new `internal/adapters/modelpack/oci/materialize_reuse.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci ./cmd/ai-models-artifact-runtime`
- `make verify`
- `git diff --check`

## Slice 18. Reuse shared bounded-volume contract for runtime delivery

Цель:

- не держать два параллельных bounded-volume contracts в
  `k8s/workloadpod` и `k8s/modeldelivery`;
- reuse already-live `workloadpod` volume semantics for the new consumer-side
  delivery seam without inventing second mini-framework for PVC/EmptyDir;
- выровнять wording и package map after the merge.

Статус:

- historical intermediate slice; later simplified further by Slice 27,
  which removed `modeldelivery`-owned volume policy entirely in favor of
  user-owned `/data/modelcache`.

Артефакты:

- updated `internal/adapters/k8s/workloadpod/options.go`
- updated `internal/adapters/k8s/workloadpod/render.go`
- updated `internal/adapters/k8s/modeldelivery/options.go`
- updated `internal/adapters/k8s/modeldelivery/render.go`
- updated `internal/adapters/k8s/modeldelivery/render_test.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/workloadpod ./internal/adapters/k8s/modeldelivery`
- `make verify`
- `git diff --check`

## Slice 19. Split sourceworker pod rendering internals

Цель:

- не оставлять `internal/adapters/k8s/sourceworker/build.go` следующей
  oversized pod-rendering точкой после cleanup `cmd/*`, `uploadsession/` и
  `oci/materialize`;
- вынести distinct concerns:
  - top-level build orchestration
  - runtime env/volume/pod shaping
  - source-specific argv shaping for `HuggingFace` and `Upload`;
- сохранить current publish-worker pod semantics without new adapter layers.

Артефакты:

- simplified `internal/adapters/k8s/sourceworker/build.go`
- new `internal/adapters/k8s/sourceworker/build_runtime.go`
- new `internal/adapters/k8s/sourceworker/build_args.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/sourceworker`
- `make verify`
- `git diff --check`

## Slice 20. Split cmdsupport shared process glue

Цель:

- не оставлять `internal/cmdsupport/common.go` shared-process monolith после
  logging-contract hardening;
- вынести separate concerns:
  - env parsing/pass-through helpers
  - structured logging contract and controller-runtime/klog bridge
  - runtime signal/termination helpers;
- сохранить current runtime entrypoint semantics unchanged.

Артефакты:

- simplified `internal/cmdsupport/common.go`
- new `internal/cmdsupport/env.go`
- new `internal/cmdsupport/logging.go`
- new `internal/cmdsupport/runtime.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/cmdsupport ./cmd/ai-models-controller ./cmd/ai-models-artifact-runtime`
- `make verify`
- `git diff --check`

## Slice 21. Split kitops concrete adapter internals

Цель:

- не оставлять `internal/adapters/modelpack/kitops/adapter.go` oversized
  concrete adapter entrypoint;
- вынести separate concerns:
  - publish/remove orchestration
  - command/auth shell
  - Kitfile/context prep
  - OCI reference and runtime env helpers;
- сохранить current `KitOps = pack/push/remove shell` boundary unchanged.

Артефакты:

- simplified `internal/adapters/modelpack/kitops/adapter.go`
- new `internal/adapters/modelpack/kitops/command.go`
- new `internal/adapters/modelpack/kitops/context.go`
- new `internal/adapters/modelpack/kitops/reference.go`
- aligned `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/kitops`
- `make verify`
- `git diff --check`

## Slice 26. Close concrete runtime delivery service

Цель:

- довести `k8s/modeldelivery` от reusable render seam до concrete
  consumer-side K8s service;
- закрыть compile/verify drift вокруг cross-namespace projected DMCR auth/CA
  reuse;
- зафиксировать в docs и bundle, что in-repo runtime delivery boundary теперь
  действительно landed, а внешним runtime modules остаётся только thin
  overlay over this service.

Артефакты:

- fixed `internal/adapters/k8s/ociregistry/projection.go`
- updated `internal/adapters/k8s/ociregistry/projection_test.go`
- updated `internal/adapters/k8s/modeldelivery/service_test.go`
- aligned:
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `plans/active/phase2-runtime-followups/TASK.ru.md`
  - `plans/active/phase2-runtime-followups/REVIEW.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/ociregistry ./internal/adapters/k8s/modeldelivery`
- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/adapters/modelpack/oci`
- `make verify`
- `git diff --check`

## Slice 27. Simplify runtime delivery to user-owned `/data/modelcache`

Цель:

- убрать из landed runtime-delivery surface старый contract
  `emptyDir + runtime env + target container`;
- зафиксировать один delivery hook:
  workload сам предоставляет storage mount at `/data/modelcache`;
- перевести `materialize-artifact` на cache-root semantics:
  `store/<digest>/model` plus `current` symlink.

Артефакты:

- updated `cmd/ai-models-artifact-runtime/materialize_artifact.go`
- updated `cmd/ai-models-artifact-runtime/materialize_artifact_test.go`
- updated `internal/adapters/k8s/modeldelivery/*`
- aligned:
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `plans/active/phase2-runtime-followups/TASK.ru.md`
  - `plans/active/phase2-runtime-followups/REVIEW.ru.md`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/modeldelivery ./internal/adapters/modelpack/oci`
- `make verify`
- `git diff --check`

## Slice 28. Fail-closed runtime delivery topology

Цель:

- закрепить корректную storage topology для reusable runtime delivery seam;
- допустить per-pod storage и StatefulSet claim templates;
- fail-closed reject'ить direct shared PVC на multi-replica workloads до
  отдельного RWX writer/waiter slice.

Артефакты:

- updated `internal/adapters/k8s/modeldelivery/service.go`
- added `internal/adapters/k8s/modeldelivery/topology.go`
- added `internal/adapters/k8s/modeldelivery/workload_hints.go`
- updated:
  - `internal/adapters/k8s/modeldelivery/service_test.go`
  - `internal/adapters/k8s/modeldelivery/workload_hints_test.go`
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `plans/active/phase2-runtime-followups/TASK.ru.md`
  - `plans/active/phase2-runtime-followups/REVIEW.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery`
- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/adapters/modelpack/oci`
- `make verify`
- `git diff --check`

## Slice 29. Shared RWX runtime cache coordination

Цель:

- довести reusable runtime delivery seam до корректного shared RWX path;
- допустить direct shared PVC на multi-replica workloads только при
  `ReadWriteMany`;
- координировать single writer прямо на shared cache root внутри
  `materialize-artifact`, не добавляя новых user-facing knobs и не требуя
  extra RBAC от service account'ов consumer workload'ов.

Артефакты:

- updated `cmd/ai-models-artifact-runtime/materialize_artifact.go`
- added `cmd/ai-models-artifact-runtime/materialize_coordination.go`
- added `cmd/ai-models-artifact-runtime/materialize_coordination_test.go`
- added `internal/adapters/k8s/modeldelivery/coordination.go`
- updated:
  - `internal/adapters/k8s/modeldelivery/render.go`
  - `internal/adapters/k8s/modeldelivery/render_test.go`
  - `internal/adapters/k8s/modeldelivery/service.go`
  - `internal/adapters/k8s/modeldelivery/service_topology_test.go`
  - `internal/adapters/k8s/modeldelivery/topology.go`
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `plans/active/phase2-runtime-followups/TASK.ru.md`
  - `plans/active/phase2-runtime-followups/REVIEW.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./cmd/ai-models-artifact-runtime`
- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `make verify`
- `git diff --check`

## Slice 30. Controller-owned workload delivery adoption

Цель:

- довести runtime delivery до live controller-owned path внутри `ai-models`;
- принять только top-level workload annotations
  `ai-models.deckhouse.io/model` /
  `ai-models.deckhouse.io/clustermodel` без новых knobs;
- мутировать только workload kinds с mutable `PodTemplateSpec`;
- fail-closed чистить stale managed state и reject'ить invalid shared PVC
  topology без leaked projected OCI auth;
- удержать workload delivery вне generic admission webhook surface: controller
  должен watch'ить только opt-in/managed workloads и requeue'ить их по
  referenced `Model` / `ClusterModel`, а не через steady-state polling по
  всему cluster workload set.

Артефакты:

- added `internal/controllers/workloaddelivery/annotations.go`
- added `internal/controllers/workloaddelivery/options.go`
- added `internal/controllers/workloaddelivery/predicate.go`
- added `internal/controllers/workloaddelivery/reconciler.go`
- added `internal/controllers/workloaddelivery/resolve.go`
- added `internal/controllers/workloaddelivery/setup.go`
- added `internal/controllers/workloaddelivery/template.go`
- added `internal/controllers/workloaddelivery/watch.go`
- added:
  - `internal/controllers/workloaddelivery/annotations_test.go`
  - `internal/controllers/workloaddelivery/predicate_test.go`
  - `internal/controllers/workloaddelivery/reconciler_test.go`
  - `cmd/ai-models-controller/config_test.go`
- updated:
  - `internal/bootstrap/bootstrap.go`
  - `cmd/ai-models-controller/config.go`
  - `internal/adapters/k8s/modeldelivery/service.go`
  - `templates/controller/rbac.yaml`
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `plans/active/phase2-runtime-followups/TASK.ru.md`
  - `plans/active/phase2-runtime-followups/REVIEW.ru.md`

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/bootstrap ./cmd/ai-models-controller ./internal/adapters/k8s/modeldelivery`
- `make verify`
- `git diff --check`

## Final validation

- `find plans/active -maxdepth 2 -type f | sort`
- `find plans/archive/2026 -maxdepth 2 -type d | sort`
- `rg -n "rebase-phase2-publication-runtime-to-go-dmcr-modelpack" --glob '!plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/**'`
- `git diff --check`
