# PLAN

## Current phase

Этап 2. Corrective rebase of publication/runtime implementation before a real
production baseline for model catalog and ai-inference integration.

## Orchestration

- mode: `full`
- read-only audits are required by policy, but no subagents are spawned in this
  turn because current execution policy forbids delegation without an explicit
  user request; audits are performed locally.

## Architecture acceptance criteria

- Go-first phase-2 data plane:
  - source publication worker/session/cleanup logic moves toward Go runtime
    code under `images/*`;
  - Python/shell stay only where they are phase-1/backend-adjacent or strictly
    build-time/tool install shell.
- Hexagonal split stays explicit:
  - `domain`
  - `application`
  - `ports`
  - `adapters`
- `ModelPack` is the publication contract.
- `KitOps`, `Modctl`, or a future module-owned implementation remain concrete
  adapters behind ports.
- Hidden backend / DVCR-style artifact plane is reused as backend plumbing and
  must not leak into public/runtime contract.
- ai-inference-oriented resolved metadata becomes a required publication output
  shape, not best-effort optional metadata.
- Public input contract stays simple:
  - `spec.source`
  - `spec.inputFormat`
  - `spec.source` means either `source.url` or `source.upload`
  - `spec.inputFormat` may be omitted when the controller can determine it
    unambiguously
  - fixed internal output stays hidden
  - no dead `spec.publish` noise survives in public API
- Public input format names must be real format names such as `Safetensors`
  and `GGUF`, not source-coupled names.
- No new fat controller growth.
- Controller file budget and quality gates remain active.

## Drift inventory to fix

- `images/backend/scripts/ai-models-backend-source-publish.py` currently mixes:
  - source acquisition;
  - archive handling;
  - `ModelPack` pack/push/inspect;
  - metadata extraction;
  - controller result mapping.
- `images/backend/scripts/ai-models-backend-upload-session.py` currently owns a
  phase-2 upload server/runtime path in Python, unlike virtualization-style
  Go-based uploader/runtime.
- Current `ModelPack` implementation is pinned as a CLI binary in the backend
  image, but not yet abstracted as a real replaceable adapter contract.
- API already contains richer `status.resolved.*` fields, but current publish
  path calculates and projects only a narrow subset.
- Runtime delivery to `ai-inference` is still missing.

## Slice 1. Canonical target architecture

Цель:

- зафиксировать explicit target ownership and replacement map for the current
  Python-centric phase-2 publication path.

Артефакты:

- `TARGET_ARCHITECTURE.ru.md`
- `DRIFT_AND_REPLACEMENTS.ru.md`

Проверки:

- manual consistency check against current live code and agreed chat invariants

## Slice 2. First landed corrective cut

Цель:

- выбрать и реализовать первый bounded code slice, который убирает один
  concrete phase-2 drift seam without breaking the current live path.

Prefer first candidates:

- move phase-2 upload session HTTP serving semantics behind a Go adapter seam;
- or pull publication metadata/result shaping out of Python into Go-side
  contract handling;
- or make current `ModelPack` CLI usage explicit behind a Go adapter boundary.

Артефакты:

- bounded code changes in `images/controller/internal/*` and/or
  `images/backend/scripts/*`
- current bundle notes/review

Статус: landed.

Реально сделано:

- manager binary теперь отдельный:
  - `cmd/ai-models-controller`
- phase-2 runtime binary теперь отдельный:
  - `cmd/ai-models-artifact-runtime`
  - `publish-worker`
  - `upload-session`
  - `artifact-cleanup`
- phase-2 Go dataplane landed в:
  - `internal/dataplane/publishworker`
  - `internal/dataplane/uploadsession`
  - `internal/dataplane/artifactcleanup`
- source acquisition вынесен в `internal/adapters/sourcefetch`
- ai-inference-oriented profile baseline вынесен в
  `internal/adapters/modelprofile/*`
- `ModelPack` contract вынесен в `internal/ports/modelpack`
- текущий `KitOps` path вынесен в concrete adapter
  `internal/adapters/modelpack/kitops`
- промежуточный runtime-side write path, который использовался в раннем
  промежуточном slice, позже полностью убран в Slice 6 вместе со служебным
  `ConfigMap`
- dedicated runtime image стал self-contained phase-2 runtime image:
  - pinned `KitOps` lives in `images/controller/kitops.lock`
  - install shell lives in `images/controller/tools/install-kitops.sh`
- publication worker Pods и cleanup Jobs переведены на runtime image
- legacy phase-2 backend Python runtimes удалены из backend image/runtime tree

Проверки:

- focused package tests
- `go test ./...` in `images/controller`
- `werf build --dev --platform=linux/amd64 controller`
- `make verify`

## Slice 3. Inference metadata hardening

Цель:

- define and land the minimum required ai-inference-oriented resolved metadata
  set that publication must calculate and project.

Артефакты:

- bundle decisions/update
- code around publication result shaping and status projection

Проверки:

- package-local tests
- controller quality gates

Статус: landed and hardened for current live formats.

Что вошло:

- publication now calculates and projects:
  - `parameterCount`
  - `quantization`
  - `supportedEndpointTypes`
  - `compatibleRuntimes`
  - `compatibleAcceleratorVendors`
  - `compatiblePrecisions`
  - `minimumLaunch`

Детали текущего расчёта:

- `Safetensors`
  - читает `config.json`
  - считает `contextWindowTokens` по известным ключам окна контекста
  - считает `parameterCount` сначала из явных полей config, затем по размерам
    `.safetensors` shard files
  - определяет `quantization` по `quantization_config`
  - определяет `compatiblePrecisions`
  - строит `supportedEndpointTypes` по `task`
  - строит `minimumLaunch` из реального веса model shards
- `GGUF`
  - читает имя и размер `.gguf` файла
  - выделяет family и quantization из имени файла
  - оценивает `parameterCount` по имени файла, а при отсутствии — по размеру
    файла
  - строит `supportedEndpointTypes` по `task`
  - строит GPU baseline `minimumLaunch` по реальному размеру `.gguf` файла и
    quantization

Оставшийся scope:

- runtime delivery to `ai-inference` remains a separate future slice;
- richer format-specific parsers may still appear later when new input formats
  are added.

## Slice 4. Source-agnostic input-format validation

Цель:

- трактовать `spec.inputFormat` как source-agnostic validation contract для
  состава model project, а не как upload-only branch flag;
- применять одинаковую fail-closed content validation к `HuggingFace`, `HTTP`
  и `Upload` до `ModelPack` packaging.

Артефакты:

- `internal/adapters/modelformat`
- tightened `internal/dataplane/publishworker`
- tightened `internal/adapters/sourcefetch/huggingface`
- synced docs / structure / evidence

Проверки:

- focused package tests for `modelformat`, `sourcefetch`, `publishworker`
- `go test ./...` in `images/controller`
- `make verify`

Статус: landed as the first strict validation baseline.

Что вошло:

- `spec.inputFormat` became the public input contract;
- fixed internal `ModelPack` output stayed hidden from public `spec`;
- `spec.source` was simplified to:
  - `source.url`
  - `source.upload`
- remote source type is now resolved internally from the URL;
- live input formats are currently:
  - `Safetensors`
  - `GGUF`
- if `spec.inputFormat` is empty, the controller now tries to determine it
  automatically from remote file lists or unpacked local contents
- current live input matrix is:
  - `HuggingFace URL -> Safetensors`
  - `HTTP URL -> Safetensors archive or GGUF file/archive`
  - `Upload -> Safetensors archive or GGUF file/archive`

## Slice 5. CRD vs ADR audit

Цель:

- отдельно сверить текущий CRD contract и фактическое заполнение `status`
  против текущего ADR из `internal-docs`;
- зафиксировать, что именно совпадает, что разошлось, и какие поля в самом CRD
  уже висят без полноценной живой семантики.

Артефакты:

- `ADR_AUDIT.ru.md`
- review update with current drift summary

Проверки:

- read-only code/ADR audit
- `git diff --check`

Статус: landed as an audit slice.

Что зафиксировано:

- текущий `status` в целом уже живой и внутренне согласованный;
- текущий `spec` заметно расходится с ADR;
- в live CRD до этого среза оставался мёртвый public knob `spec.access`.

## Slice 7. Remove dead public publish knob

Цель:

- убрать `spec.publish.repositoryClass` из CRD как мёртвый public knob без
  живой controller semantics;
- выровнять public API под уже достигнутый договор:
  `spec.source` + `spec.inputFormat` + `spec.runtimeHints`.

Артефакты:

- `api/core/v1alpha1/types.go`
- generated deepcopy and CRD artifacts
- synced active bundle audit/review notes

Проверки:

- `bash api/scripts/verify-crdgen.sh`
- `go test ./...` in `images/controller`
- `make verify`

Статус: landed.

Что вошло:

- `spec.publish` removed from `ModelSpec`
- `ModelPublishSpec` removed from public API
- generated deepcopy and CRD artifacts regenerated

## Slice 8. Remove dead public access knob

Цель:

- убрать `spec.access` из CRD как мёртвый public knob без live semantics;
- убрать cluster-scoped special rules вокруг `spec.access`, которые не
  подкреплены реальным controller/runtime path.

Артефакты:

- `api/core/v1alpha1/types.go`
- `api/core/v1alpha1/clustermodel.go`
- generated deepcopy and CRD artifacts
- synced active bundle audit/review notes

Проверки:

- `bash api/scripts/verify-crdgen.sh`
- `cd api && go test ./...`
- `cd images/controller && go test ./...`
- `make verify`

Статус: landed.

Что вошло:

- `spec.access` removed from `ModelSpec`
- `ModelAccessPolicy` removed from public API
- `ServiceAccountReference` removed from public API
- ClusterModel no longer carries dead access-specific validations
- generated deepcopy and CRD artifacts regenerated

## Slice 9. Collapse repeated remote-fetch and source-worker shell

Цель:

- убрать повторяющийся HTTP GET / error / file-write boilerplate между
  `sourcefetch/http` и `sourcefetch/huggingface`;
- убрать дублирующийся handle construction path внутри `sourceworker/service`.

Артефакты:

- `images/controller/internal/adapters/sourcefetch/transport.go`
- tightened `images/controller/internal/adapters/sourcefetch/http.go`
- tightened `images/controller/internal/adapters/sourcefetch/huggingface.go`
- tightened `images/controller/internal/adapters/k8s/sourceworker/service.go`
- synced `README` and `STRUCTURE`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/k8s/sourceworker ./...`
- `make verify`

Статус: landed.

Что вошло:

- remote source adapters now share one minimal transport helper for GET,
  response error decoding, JSON decode and response-body file write
- `HuggingFace` and generic `HTTP` no longer keep their own duplicated network
  shell
- `sourceworker/service` now keeps one local source-worker handle path instead
  of open-coding the same `NewSourceWorkerHandle(...)` block twice

## Slice 10. Split manager and phase-2 runtime binaries

Цель:

- убрать смешение controller manager и phase-2 one-shot runtime execution в
  одном binary/image entrypoint;
- приблизить build/runtime layout к virtualization-style split.

Артефакты:

- new `internal/cmdsupport`
- `cmd/ai-models-controller` becomes manager-only
- new `cmd/ai-models-artifact-runtime`
- `images/controller/werf.inc.yaml`
- `templates/controller/deployment.yaml`
- `fixtures/module-values.yaml`
- synced docs / structure / review

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller ./cmd/ai-models-artifact-runtime ./internal/cmdsupport ./...`
- `make verify`
- `werf build --dev --platform=linux/amd64 controller controller-runtime`

Статус: landed.

Что вошло:

- manager shell no longer keeps runtime subcommand dispatch
- phase-2 one-shot execution moved into dedicated runtime binary
- pinned `KitOps` stays only in runtime image, not in manager image
- deployment now passes `controller-runtime` image into cleanup and publication
  worker options
- render fixtures now include a dedicated digest for `controller-runtime`, so
  `helm-template` and `kubeconform` validate the new image path instead of
  failing before render

## Slice 11. Collapse remote ingest and direct runtime ensure flow

Цель:

- убрать split orchestration между `publishworker` и `sourcefetch` для remote
  sources;
- убрать отдельный replay-read shell в `sourceworker` и `uploadsession`, когда
  тот же adapter всё равно затем шёл в `CreateOrGet`.

Артефакты:

- new `images/controller/internal/adapters/sourcefetch/remote.go`
- new `images/controller/internal/adapters/sourcefetch/remote_test.go`
- tightened `images/controller/internal/dataplane/publishworker/run.go`
- tightened `images/controller/internal/dataplane/publishworker/support.go`
- tightened `images/controller/internal/adapters/k8s/sourceworker/service.go`
- tightened `images/controller/internal/adapters/k8s/sourceworker/build.go`
- tightened `images/controller/internal/adapters/k8s/uploadsession/service.go`
- tightened `images/controller/internal/adapters/k8s/uploadsession/resources.go`
- tightened `images/controller/internal/adapters/k8s/uploadsession/pod.go`
- tightened `images/controller/internal/adapters/k8s/uploadsession/status.go`
- synced `README`, `STRUCTURE`, `REVIEW`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/dataplane/publishworker`
- `cd images/controller && go test ./...`
- `make verify`

Статус: landed.

Что вошло:

- `sourcefetch` теперь даёт один canonical remote entrypoint:
  `FetchRemoteModel(...)`
- `publishworker` больше не держит отдельную ручную оркестрацию для
  `HuggingFace` и generic `HTTP` download/select/unpack/validate steps
- `sourceworker/service` больше не делает отдельный предварительный `Get` Pod
  перед тем же `CreateOrGet` cycle
- projected auth secret в `sourceworker` больше не держит свой ручной
  `Get/Create/Update`; он тоже идёт через один direct reconcile path
- `uploadsession/service` больше не держит отдельную pre-read ветку для
  `Pod` / `Service` / `Secret`; replay теперь идёт через тот же ensure path,
  что и создание
- derived upload status теперь строится прямо из ensured resources, без
  промежуточного replay-only helper path
- `uploadsession` больше не держит отдельный `request.go`; shared request
  mapping теперь живёт локально рядом с единственным live pod builder

## Slice 6. Remove publication-operation ConfigMap protocol

Цель:

- убрать служебный `ConfigMap` как внутренний протокол между controller и
  worker/session runtime;
- привести flow к virtualization-style path, где основной объект сам несёт
  public status, а controller напрямую наблюдает owned runtime resources.

Артефакты:

- `catalogstatus` becomes the single live publication controller
- `publishrunner` is removed
- `operationstate` adapter is removed
- worker/session result is delivered through pod termination message instead of
  a dedicated service `ConfigMap`
- owned temporary resources are attached directly to the published object
  instead of an intermediate namespaced operation object

Проверки:

- focused controller package tests
- `go test ./...` in `images/controller`
- `make verify`

Риск:

- это жёсткий architecture cut, поэтому важно не заменить ConfigMap на другой
  service object with the same smell

## Rollback point

After Slice 1. Bundle and target architecture are written, but no unsafe code
replacement has landed yet.

## Final validation

- package-local `go test`
- controller quality gates
- `go test ./...` in `images/controller`
- `make verify`
- `werf build --dev --platform=linux/amd64 controller controller-runtime`
- `git diff --check`
