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
- Hidden backend / DMCR-style artifact plane is reused as backend plumbing and
  must not leak into public/runtime contract.
- External publication registry wiring is not part of the target architecture.
  The controller must publish only into a module-local internal DMCR-style
  registry service.
- Internal publication storage supports only the two deployment modes that
  matter for this module:
  - `S3`-compatible object storage;
  - `PersistentVolumeClaim`.
- Deleting `Model` / `ClusterModel` must delete the internal published artifact
  as part of the same publication lifecycle.
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
- Public `spec` drift is now in the opposite direction:
  - old ADR policy fields were removed entirely;
  - current `Validated=True` path does not read any public policy contract at
    all;
  - the module cannot express `modelType`, allowed endpoint usage, launch
    constraints, or draft-model optimization intent.
- `uploadsession` still accepts bytes and runs heavy publication work in the
  same ephemeral runtime process, which is a poor fit for large-model uploads.
- `KitOps` still lives as a CLI boundary inside the runtime image.
- `sourcefetch` and `modelformat` remain oversized concrete adapter seams.
- Live chart/runtime shell is still external-registry-centric:
  - user-facing `publicationRegistry` config leaks backend plumbing;
  - controller publishes into external `GHCR` instead of an internal module
    backend;
  - there is no module-local registry service with `S3/PVC` storage ownership.

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
  - install shell lives in `images/controller/install-kitops.sh`
- publication worker Pods и cleanup Jobs переведены на runtime image
- legacy phase-2 backend Python runtimes удалены из backend image/runtime tree

Проверки:

- focused package tests
- `go test ./...` in `images/controller`
- `werf build --dev --platform=linux/amd64 controller`
- `make verify`

## Slice 9. Internal DMCR publication backend

Цель:

- заменить external-registry-centric phase-2 wiring на реальный module-local
  internal DMCR publication plane;
- переподключить controller publication workers и cleanup path на internal
  registry service;
- убрать внешний `publicationRegistry` contract из user-facing values и
  заменить его storage-centric module contract.

Артефакты:

- module-local `images/dmcr/*`
- `templates/dmcr/*`
- updated controller/module templates and helpers
- updated `openapi/*`, docs, bundle notes

Проверки:

- `make helm-template`
- `make kubeconform`
- targeted `werf build --dev --platform=linux/amd64 dmcr controller controller-runtime`
- `make verify`

Статус: landed.

Реально сделано:

- module-local internal registry image landed in:
  - `images/dmcr/werf.inc.yaml`
- module-local internal registry binary now has its own repo-owned Go source:
  - `images/dmcr/go.mod`
  - `images/dmcr/cmd/dmcr/*`
- module-local internal registry runtime shell landed in:
  - `templates/dmcr/*`
- shared protected metrics helper landed in:
  - `templates/kube-rbac-proxy/*`
- user-facing external-registry contract removed:
  - `templates/module/publication-registry-secret.yaml` deleted
  - `publicationRegistry` values/schema replaced with `publicationStorage`
- controller publication runtime now always points to the internal registry
  service with module-owned auth/CA wiring
- internal publication storage now supports:
  - `ObjectStorage`
  - `PersistentVolumeClaim`
- docs, repo layout and controller runtime notes were synchronized to the new
  module-local DMCR shape
- build shell no longer clones upstream registry source during image build;
  it now compiles the local `dmcr` binary from the repo-owned wrapper module
  over normal Go module resolution through `GOPROXY`
- controller and DMCR metrics now bind locally inside the Pod and are exposed
  to Prometheus only through kube-rbac-proxy sidecars over HTTPS, closer to
  the virtualization pattern
- top-level docs no longer claim that ai-inference runtime materialization is
  already landed; that workstream remains separate until there is a live
  consumer path

Фактические проверки:

- `make helm-template`
- `make kubeconform`
- `werf config render --dev --loose-giterminism`
- `werf build --dev --platform=linux/amd64 --loose-giterminism dmcr controller controller-runtime`
- direct PVC-mode render via module Helm toolchain:
  - `./.cache/helm/v3.17.2/darwin-arm64/helm template ... --set-string aiModels.publicationStorage.type=PersistentVolumeClaim`
- `make verify`
- `git diff --check`

## Slice 10. Production shell hardening for controller and DMCR

Цель:

- довести live deployment shell `controller` и `dmcr` до closer
  `virtualization` / `gpu-control-plane` production pattern без выдумывания
  несуществующего runtime-delivery слоя;
- закрыть remaining DKP deployment drifts в placement, HA shell, VPA wiring,
  protected scheduling и read-only-rootfs discipline.

Артефакты:

- `templates/controller/deployment.yaml`
- `templates/controller/vpa.yaml`
- `templates/dmcr/deployment.yaml`
- `templates/dmcr/vpa.yaml`
- `templates/kube-rbac-proxy/_helpers.tpl`
- `templates/_helpers.tpl`
- bundle notes/review

Проверки:

- `make helm-template`
- `make kubeconform`
- `werf config render --dev --loose-giterminism`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- `controller` deployment shell теперь использует:
  - HA anti-affinity
  - `system-cluster-critical` priority class
  - control-plane placement
  - fail-safe control-plane tolerations
  - read-only-rootfs container security helper
  - VPA object when `vertical-pod-autoscaler-crd` is enabled
- `dmcr` deployment shell теперь использует:
  - HA anti-affinity
  - `system-cluster-critical` priority class
  - fail-safe `system -> control-plane` node selector fallback
  - fail-safe system/control-plane tolerations
  - read-only-rootfs container security helper
  - HTTPS liveness/readiness probes instead of raw TCP socket checks
  - VPA object when `vertical-pod-autoscaler-crd` is enabled
- module-local helper layer now owns the fail-safe DKP placement logic needed
  for clean fixture renders:
  - `ai-models.controlPlaneNodeSelector`
  - `ai-models.systemNodeSelector`
  - `ai-models.controlPlaneTolerations`
  - `ai-models.systemTolerations`
  - `ai-models.vpaPolicyUpdateMode`
- local `kube-rbac-proxy` helper now also exports VPA container policy so the
  sidecar is covered by the same production shell as the main workloads

## Slice 11. DMCR garbage collection lifecycle

Цель:

- довести destructive publication cleanup до полного module-owned lifecycle,
  а не останавливаться на удалении remote manifest reference;
- добавить virtualization-style coordination между controller cleanup
  finalizer, internal DMCR maintenance mode и physical blob garbage
  collection.

Артефакты:

- `images/dmcr/cmd/dmcr-cleaner/*`
- `images/dmcr/werf.inc.yaml`
- `images/hooks/pkg/hooks/dmcr_garbage_collection/*`
- `templates/dmcr/*`
- `templates/_helpers.tpl`
- `images/controller/internal/application/deletion/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- updated docs and bundle notes

Проверки:

- `cd images/dmcr && go test ./...`
- `cd images/hooks && go test ./...`
- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `make verify`
- `git diff --check`
- targeted `werf build --dev --platform=linux/amd64 --loose-giterminism dmcr controller controller-runtime`

Статус: landed with targeted build blocked by local Docker daemon availability.

Реально сделано:

- module-local `dmcr-cleaner` binary landed in:
  - `images/dmcr/cmd/dmcr-cleaner/*`
- `dmcr` image now ships both:
  - `dmcr`
  - `dmcr-cleaner`
- hook-driven internal switch now projects DMCR garbage-collection mode into
  module values when cleanup request secrets exist
- DMCR config now enables maintenance/read-only mode only while internal
  garbage collection is active
- DMCR deployment now runs a dedicated `dmcr-garbage-collection` sidecar that:
  - waits idle in normal mode
  - executes registry `garbage-collect` in GC mode
  - marks cleanup requests as completed
- `catalogcleanup` finalizer now waits for three explicit phases:
  - remote artifact delete job
  - DMCR garbage-collection request creation/running
  - request completion before finalizer removal
- controller tests were updated so finalizer removal no longer happens right
  after remote delete completion; it now requires completed DMCR garbage
  collection too

Фактические проверки:

- `cd images/dmcr && go test ./...`
- `cd images/hooks && go test ./...`
- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `make verify`
- `git diff --check`
- targeted `werf build --dev --platform=linux/amd64 --loose-giterminism dmcr controller controller-runtime`
  reached build planning but could not run in this session because Docker
  daemon access was unavailable:
  `Cannot connect to the Docker daemon at unix:///Users/myskat_90/.docker/run/docker.sock`

## Slice 18. Structural cleanup of current runtime hotspots

Цель:

- сделать уже landed runtime tree чище без нового architectural drift;
- убрать два текущих structural hotspots, где код ещё не соответствует своей
  declared boundary:
  - `images/controller/internal/controllers/catalogcleanup`
  - `images/dmcr/cmd/dmcr-cleaner`

Артефакты:

- split `catalogcleanup` package files by real responsibility instead of one
  monolithic `io.go`
- thin `dmcr-cleaner` command shell with runtime logic moved into an explicit
  internal implementation package
- synced structure docs and bundle notes

Проверки:

- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./...`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- `catalogcleanup` больше не держит один generic `io.go`;
  package разрезан по реальным responsibilities:
  - `observe.go`
  - `apply.go`
  - `status.go`
  - `gc_request.go`
- delete-time GC request lifecycle теперь живёт рядом со своей resource/state
  model, а не в общем controller I/O файле
- `dmcr-cleaner/cmd/gc.go` снова стал thin CLI shell:
  in-cluster client bootstrap и registry garbage-collection loop вынесены в
  `images/dmcr/internal/garbagecollection`
- tests переехали вместе с runtime lifecycle logic:
  `shouldRunGarbageCollection` теперь проверяется рядом с internal
  implementation seam, а не в command package

Фактические проверки:

- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./...`
- `make verify`
- `git diff --check`

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
- render fixtures now include a dedicated `controllerRuntime` digest key for
  the `controller-runtime` image, so
  `helm-template` and `kubeconform` validate the new image path instead of
  failing before render
- controller registration now uses explicit unique names across
  `catalogcleanup` and `catalogstatus`, so manager startup does not fail on
  duplicate controller-runtime metric/name validation
- `controller-kitops-artifact` and `distroless-artifact` now keep the
  `apt-get install` line at the same YAML list level as `alt packages proxy`,
  matching `virtualization` / `gpu-control-plane` and preventing malformed
  `beforeInstall` shell commands during `werf build`
- `controller-kitops-artifact` now tracks imported single files through
  `stageDependencies.install: ["**/*"]` instead of fake path names derived
  from the destination path, so werf no longer warns that
  `images/controller/install-kitops.sh/install-kitops.sh` and
  `images/controller/kitops.lock/kitops.lock` do not exist in git
- the same stage now creates `/root/.local/share/kitops` before running
  `kit version --show-update-notifications=false`, so KitOps can persist its
  disable-notifications marker without aborting the build
- full local `werf build --dev --platform=linux/amd64` completed successfully
  after these shell fixes, covering the whole image graph through `bundle`
- root `.helmignore` is now aligned with the DKP module pattern from
  `gpu-control-plane` / `virtualization`, so Helm no longer sees generated
  `hooks/go`, docs, openapi, CRDs and root READMEs as chart files during
  operator startup
- a synthetic local `helm package` check with a 42 MiB
  `hooks/go/ai-models-module-hooks` now excludes that file from the packaged
  chart, removing the exact drift that triggered
  `chart file "ai-models-module-hooks" is larger than the maximum file size`
- controller bootstrap now bridges the chosen `slog` logger into both
  `controller-runtime/pkg/log` and `k8s.io/klog/v2`, matching the live logging
  pattern from `virtualization` and removing the delayed
  `[controller-runtime] log.SetLogger(...) was never called` warning at runtime

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

## Slice 12. Align module build and deploy shell with DKP patterns

Цель:

- убрать platform-shell drift против `gpu-control-plane` и `virtualization`
  для phase-2 runtime path;
- сделать distroless и `helm_lib` usage честными DKP module patterns, а не
  локальной отсебятиной.

Артефакты:

- new `images/distroless/werf.inc.yaml`
- tightened `images/controller/werf.inc.yaml`
- tightened `Chart.yaml`
- new `Chart.lock`
- removed `templates/deckhouse-lib.tpl`
- synced docs / bundle review

Проверки:

- `make helm-template`
- `make kubeconform`
- `werf build --dev --platform=linux/amd64 distroless controller controller-runtime`
- `git diff --check`

Ограничение:

- backend image and its ConfigMap-driven runtime wrappers stay phase-1 shell in
  this slice; no premature backend distroless conversion lands here.

Статус: landed.

Что вошло:

- added module-local `images/distroless/werf.inc.yaml`, so controller runtime
  images now follow the same relocation pattern as `gpu-control-plane` and
  `virtualization` instead of consuming `base/distroless` directly;
- `images/controller/werf.inc.yaml` now builds `controller` and
  `controller-runtime` from the module-local `distroless` image;
- root chart now declares `deckhouse_lib_helm` as a real dependency in
  `Chart.yaml`, keeps the vendored archive in `charts/`, and no longer relies
  on a repo-local helper fork as the primary source of truth;
- `.helmignore` no longer excludes `charts/`, so `helm template` actually sees
  the vendored library chart instead of silently losing it during render;
- local `templates/deckhouse-lib.tpl` was removed after render validation
  proved that the vendored library chart covers the live helper surface;
- repo layout and controller docs now explicitly describe module-local
  distroless and real chart dependency ownership.

## Slice 13. Promote KitOps to a dedicated runtime artifact stage

Цель:

- перестать скачивать и класть `KitOps` как побочный шаг внутри Go build stage;
- сделать `KitOps` отдельным controller-owned runtime artifact seam с явным
  smoke-check в image build.

Артефакты:

- tightened `images/controller/werf.inc.yaml`
- tightened `images/controller/install-kitops.sh`
- synced docs / bundle review

Проверки:

- `werf build --dev --platform=linux/amd64 controller-kitops-artifact controller-runtime`
- `make verify`
- `git diff --check`

Ограничение:

- release asset source for `KitOps` may stay external in this slice, but the
  fetch/install path must become its own stage instead of being hidden inside
  Go compilation.

Статус: landed.

Что вошло:

- `KitOps` install no longer runs inside `controller-build-artifact`; it now
  has a dedicated `controller-kitops-artifact` stage in
  `images/controller/werf.inc.yaml`;
- `controller-runtime` imports the pinned `kit` binary from that dedicated
  artifact stage instead of coupling external tool delivery to Go compilation;
- `install-kitops.sh` became mirror-ready for future hardening by accepting
  optional archive/URL/checksum overrides and by failing closed when the archive
  does not contain the expected `kit` binary;
- the dedicated artifact stage now runs a live `kit version` smoke-check during
  image build so broken release assets fail before runtime.

## Slice 14. Binpack the lone KitOps installer seam

Цель:

- убрать fake boundary `images/controller/tools/`, который держал ровно один
  installer script;
- выровнять controller root с pruning rules: tiny build-only seams живут рядом с
  `werf.inc.yaml` и `kitops.lock`, а не в отдельном каталоге без собственной
  архитектурной роли.

Артефакты:

- moved `images/controller/install-kitops.sh`
- tightened `images/controller/werf.inc.yaml`
- synced controller docs / bundle memory

Проверки:

- `bash -n images/controller/install-kitops.sh`
- `make verify`
- `git diff --check`

Статус: landed.

Что вошло:

- `install-kitops.sh` moved from `images/controller/tools/` to the controller
  root next to `kitops.lock`;
- `controller-kitops-artifact` now imports the installer directly from
  `/images/controller/install-kitops.sh` without a one-file `tools/` directory;
- controller structure docs now explicitly reject a dedicated `tools/` directory
  for a single build-only installer script.

## Slice 15. Audit remaining module-shell drifts vs virtualization patterns

Цель:

- отдельно сверить live `werf` / bundle / image-base shell против
  `virtualization` и `gpu-control-plane`;
- зафиксировать только реальные оставшиеся drifts, чтобы следующий corrective
  cut был bounded и не плодил новую отсебятину.

Артефакты:

- updated bundle review with read-only findings

Проверки:

- manual diff audit against:
  - `virtualization/werf.yaml`
  - `virtualization/.werf/images.yaml`
  - `virtualization/images/distroless/werf.inc.yaml`
  - `gpu-control-plane/werf.yaml`
  - `gpu-control-plane/.werf/images.yaml`
  - `gpu-control-plane/images/distroless/werf.inc.yaml`

Статус: landed as review-only slice.

Что зафиксировано:

- current `bundle` stage still omits `Chart.yaml` and vendored `charts/`, so the
  release payload does not yet match the now-correct helm render path;
- backend build still bypasses the Deckhouse base-image map with raw `node:` and
  `python:` `from:` images instead of a module-disciplined image source path;
- root `werf` shell still lacks the common mirror/proxy discipline
  (`SOURCE_REPO_GIT`, distro packages proxy helpers) used by `virtualization`
  and `gpu-control-plane`.

## Slice 16. Restore module-shell parity for bundle and mirrors

Цель:

- привести release bundle к честному chart payload после перехода на vendored
  `deckhouse_lib_helm`;
- поднять в root `werf` общий mirror/proxy context и reusable helper templates,
  чтобы package installs и git source fetches перестали жить как ad-hoc shell.

Артефакты:

- added `.werf/stages/helpers.yaml`
- tightened `werf.yaml`
- tightened `werf-giterminism.yaml`
- tightened `.werf/stages/bundle.yaml`
- tightened `images/distroless/werf.inc.yaml`
- tightened `images/controller/werf.inc.yaml`
- tightened `images/backend/werf.inc.yaml`
- tightened backend fetch scripts
- synced `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- `bash -n images/backend/scripts/fetch-source.sh`
- `bash -n images/backend/scripts/fetch-oidc-auth-source.sh`
- `make verify`
- `werf build --dev --platform=linux/amd64 backend-source-artifact backend-ui-build backend-oidc-auth-ui-build bundle`
- `git diff --check`

Статус: landed with one environment-limited validation gap.

Что вошло:

- root `werf.yaml` now exports `SOURCE_REPO_GIT` and `DistroPackagesProxy`, and
  shared package-manager helpers now live in `.werf/stages/helpers.yaml`;
- live stage files now consume shared helper templates instead of inline
  distro-specific proxy shell for alt/debian/alpine package installs;
- backend source fetch scripts now rewrite GitHub/GitLab repository URLs through
  `SOURCE_REPO_GIT` when a mirror is configured;
- release `bundle` now includes `Chart.yaml`, `Chart.lock`, and vendored
  `charts/`, so the release payload matches the live helm dependency path;
- repo layout docs now explicitly require bundle/chart parity and root-level
  mirror/proxy discipline.

Validation note:

- `make verify`, shell syntax checks, and `git diff --check` passed;
- the targeted `werf build` was rendered and planned successfully, but the local
  Docker API failed again with `_ping` `500 Internal Server Error` before the
  first base image could complete.

## Slice 17. Remove raw node bases from backend UI build path

Цель:

- убрать оставшиеся raw `node:` image refs из backend UI build stages там, где в
  Deckhouse base-image map уже есть module-disciplined replacement.

Артефакты:

- tightened `images/backend/werf.inc.yaml`

Проверки:

- `make verify`
- `werf build --dev --platform=linux/amd64 backend-ui-build backend-oidc-auth-ui-build`
- `git diff --check`

Статус: landed with environment-limited build confirmation.

Что вошло:

- `backend-ui-build` and `backend-oidc-auth-ui-build` now build from
  `builder/node-alpine` instead of raw external `node:` images;
- the matching install shell was rewritten from Debian `apt` to Alpine `apk`
  while preserving the same build responsibilities and source fetch path.

## Slice 19. Restore live public model policy contract

Цель:

- вернуть `modelType`, `usagePolicy`, `launchPolicy` и `optimization` только в
  той форме, в которой controller уже может реально валидировать их against
  calculated profile;
- перестать ставить `Validated=True` без чтения public policy.

Артефакты:

- `api/core/v1alpha1/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/application/publishobserve/*`
- focused docs and bundle notes

Проверки:

- `cd api && bash scripts/update-codegen.sh && go test ./...`
- `cd images/controller && go test ./internal/domain/publishstate ./internal/application/publishobserve ./...`
- `bash api/scripts/verify-crdgen.sh`

Статус: landed.

Реально сделано:

- public policy contract restored only with live semantics:
  - `spec.modelType`
  - `spec.usagePolicy`
  - `spec.launchPolicy`
  - `spec.optimization`
- API immutability and CRD schema were regenerated from codegen
- `Validated` / `Ready` no longer become `True` blindly after a successful
  publication worker run
- controller now validates the declared public policy against the resolved
  publication profile before projecting the final status
- condition reasons were extended to explain policy mismatches:
  - `ModelTypeMismatch`
  - `EndpointTypeNotSupported`
  - `RuntimeNotSupported`
  - `AcceleratorPolicyConflict`
  - `OptimizationNotSupported`
- speculative decoding remains intentionally narrow:
  today it is accepted only for resolved LLM/chat-or-generation profiles and
  does not yet resolve or cross-validate referenced draft models

Фактические проверки:

- `cd api && bash scripts/update-codegen.sh`
- `cd api && go test ./...`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./internal/domain/publishstate ./internal/application/publishobserve ./internal/controllers/catalogstatus`
- `cd images/controller && go test ./...`
- `make verify`
- `git diff --check`

## Slice 20. Remove the KitOps CLI runtime dependency

Цель:

- заменить current `KitOps` CLI adapter на native Go OCI publication/remove
  path, если это можно сделать без слома internal publication contract;
- убрать pinned binary/install shell from the runtime image.

Артефакты:

- `images/controller/internal/ports/modelpack/*`
- current concrete adapter under `images/controller/internal/adapters/modelpack/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/werf.inc.yaml`
- `images/controller/README.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/... ./internal/dataplane/publishworker ./cmd/ai-models-artifact-runtime`
- `make verify`
- targeted `werf build --dev --platform=linux/amd64 controller controller-runtime`

## Slice 21. Break the synchronous upload-session critical path

Цель:

- уйти от live smell, где upload pod сам же синхронно делает publication after
  the final byte;
- ввести explicit intermediate ownership seam for large uploads instead of
  `receive + publish` in one request lifecycle.

Артефакты:

- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/internal/application/publishobserve/*`
- `templates/controller/*` only if upload ingress/service shell changes

Проверки:

- focused controller/runtime tests around upload path
- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./internal/application/publishobserve`
- `make verify`

## Slice 22. Shrink sourcefetch and modelformat hotspots in place

Цель:

- уменьшить concrete adapter hotspots без новых generic buckets;
- сделать per-format/per-transport ownership explicit.

Артефакты:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/modelformat/*`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- focused package tests
- `make verify`

Статус: landed.

Реально сделано:

- `internal/adapters/modelformat/` was split by explicit format ownership
  instead of keeping all detect/validate logic in one growing file:
  - `format_common.go`
  - `safetensors.go`
  - `gguf.go`
  - thin top-level `detect.go`
  - thin top-level `validation.go`
- `internal/adapters/sourcefetch/` was split by explicit remote/provider
  ownership:
  - `remote.go` keeps only the canonical remote ingest entrypoint
  - provider-specific heavy fetch logic now lives in `http.go` and
    `huggingface.go`
- the corrective cut stayed in place:
  no new generic helper buckets or fake support packages were introduced
- `images/controller/STRUCTURE.ru.md` was synchronized to the new hotspot shape

Фактические проверки:

- `cd images/controller && go test ./internal/adapters/modelformat ./internal/adapters/sourcefetch`
- `cd images/controller && go test ./...`
- `make verify`
- `git diff --check`

## Slice 23. Rewrite the target architecture around honest DMCR patterns

Цель:

- перестать держать target architecture в режиме wish-list и явно
  зафиксировать реальный hexagonal target around `DMCR`, upload session,
  auth/trust projection and future materialization;
- описать reuse from `virtualization` without falsely claiming already landed
  upload/session/materializer capabilities.

Артефакты:

- `plans/active/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/TARGET_ARCHITECTURE.ru.md`
- `plans/active/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/DRIFT_AND_REPLACEMENTS.ru.md`
- `plans/active/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/REVIEW.ru.md`

Проверки:

- docs sanity review against live code and `virtualization` references
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- target architecture was rewritten from a short generic note into an explicit
  contract for:
  - public API / control plane
  - hidden data plane
  - hidden storage plane
  - upload staging-first flow
  - auth/trust/authorization boundaries
  - DMCR GC lifecycle
  - future materialization contract
- the document now states explicitly that:
  - `port-forward` is not the target upload UX
  - shard/torrent upload into registry is not the target
  - parallel upload belongs to object-storage staging, not to OCI registry
  - `virtualization` patterns are reused at the boundary level, not copied
    blindly together with CDI-specific flows
- drift companion notes now explicitly enumerate the remaining gaps:
  - synchronous upload path
  - CLI modelpack adapter
  - simplified DMCR auth/trust lifecycle
  - missing consumer materialization

Фактические проверки:

- manual bundle/docs review against `virtualization/docs/internal/data_source_details.md`
- manual bundle/docs review against `virtualization/docs/internal/dvcr_auth.md`
- `make verify`
- `git diff --check`

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

## Slice 24. Upload session URLs and ingress-backed edge

Цель:

- убрать `port-forward`-centric upload UX из live phase-2 controller path;
- перевести `source.upload` на controller-owned session URLs по паттерну
  `virtualization`, не притворяясь при этом, что staging-first async publish
  уже landed.

Артефакты:

- `api/core/v1alpha1/types.go`
- `crds/ai-models.deckhouse.io_{models,clustermodels}.yaml`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/dataplane/uploadsession/run.go`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/publishstate/*`
- `templates/controller/deployment.yaml`
- `templates/controller/rbac.yaml`
- `templates/_helpers.tpl`
- runtime/docs/bundle notes

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/application/publishobserve ./internal/domain/publishstate ./internal/controllers/catalogstatus ./internal/bootstrap ./internal/ports/publishop ./internal/dataplane/uploadsession`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- `ModelStatus.upload` now projects URL capability instead of a
  `kubectl port-forward` helper command:
  - `status.upload.inClusterURL`
  - `status.upload.externalURL`
- controller upload session adapter now owns:
  - `Pod`
  - `Service`
  - short-lived auth `Secret`
  - optional session `Ingress`
- controller bootstrap/runtime shell now passes:
  - `--upload-public-host`
  - `--upload-ingress-class`
  - `--upload-ingress-tls-secret-name`
- upload runtime now accepts both:
  - `/upload`
  - `/upload/<token>`
  so the new URL-based edge works without breaking the old auth-header path
- controller/publication projection tests, runtime observation tests and CRD
  schema were synchronized to the new upload surface

Фактические проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/application/publishobserve ./internal/domain/publishstate ./internal/controllers/catalogstatus ./internal/bootstrap ./internal/ports/publishop ./internal/dataplane/uploadsession`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `make helm-template`
- `make kubeconform`
- `make verify`

## Slice 25. DMCR auth/trust projection for publication and cleanup runtimes

Цель:

- довести publication-side `DMCR` auth/trust до честного
  `virtualization`-style projection lifecycle;
- убрать прямое потребление root write secret из one-shot runtime pods/jobs;
- разделить server auth secret, write client secret и read client secret без
  выдумывания нового public contract.

Артефакты:

- `templates/_helpers.tpl`
- `templates/dmcr/secret.yaml`
- `templates/controller/deployment.yaml`
- `images/controller/internal/adapters/k8s/ociregistry/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- `images/controller/internal/support/{resourcenames,testkit}/*`
- bundle/docs notes

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/ociregistry ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/controllers/catalogcleanup ./internal/bootstrap ./internal/support/resourcenames`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- `DMCR` auth was split into:
  - server-side htpasswd secret
  - write client secret
  - read client secret
- controller/runtime wiring now uses the write client secret instead of the
  server auth secret;
- `sourceworker`, `uploadsession`, and delete-time cleanup jobs now receive
  controller-owned projected OCI auth/CA copies derived per owner UID, rather
  than reading the module root secret directly;
- derived auth/trust objects are now explicitly deleted together with runtime
  completion / delete finalization;
- the main remaining auth/trust drift moved to the future consumer side:
  read-only projection into materializer / `ai-inference` runtime still does
  not exist because that runtime is not landed yet.

Фактические проверки:

- `cd images/controller && go test ./internal/adapters/k8s/ociregistry ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/controllers/catalogcleanup ./internal/bootstrap ./internal/support/resourcenames`

## Slice 26. Staging-first async upload publication and public status cleanup

Цель:

- убрать live `receive bytes -> publish` synchronous critical path из upload
  runtime;
- выровнять upload flow под `virtualization` pattern:
  upload edge only stages bytes, controller then runs separate publish worker;
- удалить legacy public `status.upload.command`, который уже не несёт live
  semantics.

Артефакты:

- `api/core/v1alpha1/types.go`
- `crds/ai-models.deckhouse.io_{models,clustermodels}.yaml`
- `images/controller/internal/support/cleanuphandle/*`
- `images/controller/internal/support/modelobject/*`
- `images/controller/internal/ports/publishop/*`
- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/adapters/k8s/{objectstorage,sourceworker,uploadsession,workloadpod}/*`
- `images/controller/internal/adapters/uploadstaging/s3/*`
- `images/controller/internal/dataplane/{uploadsession,publishworker,artifactcleanup}/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/cmd/ai-models-controller/run.go`
- `templates/controller/deployment.yaml`
- docs/bundle notes

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/objectstorage ./internal/adapters/uploadstaging/s3 ./internal/application/publishplan ./internal/application/publishobserve ./internal/domain/publishstate ./internal/controllers/catalogstatus ./internal/controllers/catalogcleanup ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/support/cleanuphandle ./internal/support/modelobject ./internal/ports/publishop`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- `source.upload` больше не держит live `receive bytes -> publish` critical
  path:
  upload session runtime теперь только принимает байты и пишет их в
  controller-owned object-storage staging;
- termination message upload runtime теперь несёт `upload staging` cleanup
  handle, а controller сохраняет его, удаляет upload runtime и requeue'ит
  объект в normal publish-worker path;
- staged upload source затем продолжается через `sourceworker`, который
  скачивает staged object, валидирует/профилирует модель, публикует `ModelPack`
  в `DMCR` и при успехе удаляет staging object;
- delete-time cleanup path теперь понимает и backend artifact handles, и
  upload staging handles;
- public contract очищен:
  `status.upload.command` удалён из API/CRD, live upload status теперь
  ограничен URL capability и expiry/repository metadata.

Фактические проверки:

- `cd images/controller && go test ./internal/... ./cmd/ai-models-artifact-runtime/...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`

## Slice 27. Runtime shell binpack and `cmdsupport` boundary cleanup

Цель:

- убрать fake seam, где shared `internal/cmdsupport` знает concrete
  upload-staging adapter;
- вернуть environment/result wiring в ту binary boundary, которой он реально
  принадлежит;
- не оставлять в controller tree shared glue, который тащит concrete
  data-plane details только ради удобства.

Артефакты:

- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/cmdsupport/*`
- `images/controller/STRUCTURE.ru.md`
- bundle notes

Проверки:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/cmdsupport ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- staging S3 env wiring больше не живёт в `internal/cmdsupport`;
- общий helper для upload-staging config теперь binpack'нут в
  `cmd/ai-models-artifact-runtime`, где он и используется тремя subcommands;
- `cmdsupport` больше не знает `uploadstaging/s3` adapter и больше не кодирует
  `publicationartifact.Result` как shared helper;
- termination result encoding для publish-worker теперь живёт в самом runtime
  command path, а shared `cmdsupport` снова ближе к process-level glue.

Фактические проверки:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/cmdsupport ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup`

## Slice 28. Direct multipart staging for `source.upload`

Цель:

- довести object-storage upload path до честного direct multipart staging;
- оставить `uploadsession` только session/control-plane runtime instead of a
  raw byte receiver;
- сохранить staging-first async publication flow без возврата к synchronous
  upload worker.

Артефакты:

- `images/controller/internal/ports/uploadstaging/*`
- `images/controller/internal/adapters/uploadstaging/s3/*`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/cmd/ai-models-artifact-runtime/upload_session.go`
- docs/bundle notes

Проверки:

- `cd images/controller && go test ./internal/dataplane/uploadsession ./internal/adapters/uploadstaging/s3 ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./cmd/ai-models-artifact-runtime`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Статус: landed.

Реально сделано:

- `uploadstaging` port stopped pretending to be a simple body uploader and now
  owns explicit multipart primitives:
  - start multipart upload
  - presign upload parts
  - complete multipart upload
  - abort multipart upload
  - stat staged object
- S3 staging adapter now implements those multipart operations directly over
  AWS SDK v2 presign/create/complete/abort/head paths;
- `uploadsession` dataplane no longer receives final model bytes into the Pod;
  it now exposes a small session API over the existing session URL:
  - `GET <sessionURL>`
  - `POST <sessionURL>/init`
  - `POST <sessionURL>/parts`
  - `POST <sessionURL>/complete`
  - `POST <sessionURL>/abort`
- upload Pod now keeps only upload session token validation and multipart
  control-plane duties; data bytes go directly from client to object storage
  staging through presigned part URLs;
- multipart session state is now persisted in the existing upload Secret
  through a dedicated K8s adapter `internal/adapters/k8s/uploadsessionstate`
  instead of living only in Pod memory or leaking into `cmdsupport`;
- upload Ingress path switched from `Exact` to `Prefix`, because session URLs
  now own subpaths under the same capability root;
- controller/runtime public surface stayed stable:
  `status.upload.{inClusterURL,externalURL}` still points to the base session
  URL, while the new multipart API lives behind that same capability.

Фактические проверки:

- `cd images/controller && go test ./internal/dataplane/uploadsession ./internal/adapters/uploadstaging/s3 ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./cmd/ai-models-artifact-runtime`

## Slice 29. Remove MLflow from the phase-2 publication target

Цель:

- зафиксировать, что phase-2 publish path не требует `MLflow` lifecycle для
  normal operation;
- не смешивать raw-ingest storage, audit hooks и final OCI publication backend
  в один монолит;
- вернуть `CRD + DMCR` как единственную pair of truths для platform state.

Артефакты:

- `plans/active/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/TARGET_ARCHITECTURE.ru.md`

Проверки:

- manual consistency check against:
  - `docs/CONFIGURATION.ru.md`
  - current `uploadsession` / `publishworker` live flow
  - phase rules from `docs/development/PHASES.ru.md`

Статус: landed.

Реально зафиксировано:

- phase-2 publish path stays independent from internal `MLflow` backend;
- controller owns raw object URI allocation and deterministic storage layout;
- raw bytes go directly into controlled object storage, not through backend
  proxy and not through `DMCR`;
- `CRD.status` remains the only platform truth for publication state;
- `DMCR` remains the only storage truth for published OCI artifact;
- future audit/log export may exist only as append-only, non-authoritative
  hooks.

## Slice 30. Introduce the shared upload gateway target

Цель:

- заменить per-upload runtime thinking на один shared upload edge for
  object-storage mode;
- гарантировать, что десятки параллельных uploads не раздувают cluster
  footprint линейно по Pods/Services/Ingresses;
- отделить upload concurrency от publish concurrency.

Артефакты:

- `images/controller/internal/ports/*`
- `images/controller/internal/application/*`
- `images/controller/internal/adapters/k8s/*`
- bundle/docs notes

Проверки:

- focused `go test` on upload/session/gateway packages
- `make verify`
- `git diff --check`

Ожидаемый результат:

- one shared upload gateway Deployment + Service + optional Ingress;
- one short-lived session `Secret` per upload instead of one uploader Pod per
  upload;
- fixed control API:
  - `GET /v1/upload/<sessionID>`
  - `POST /probe`
  - `POST /init`
  - `POST /parts`
  - `POST /complete`
  - `POST /abort`;
- direct multipart upload into controller-owned raw object URIs;
- bounded publish workers continue later as a separate queue.

Статус: landed.

Реально сделано:

- upload path больше не создаёт per-upload runtime objects:
  - upload `Pod`
  - upload `Service`
  - upload `Ingress`
- controller deployment теперь несёт shared `upload-gateway` sidecar на том же
  deployment shell;
- controller Service теперь общий:
  - protected metrics port
  - shared upload control port
- public upload edge теперь optional shared controller `Ingress` c fixed
  `/v1/upload` prefix instead of session-specific ingress objects;
- upload session adapter теперь создаёт только один short-lived session
  `Secret` per upload и читает runtime lifecycle напрямую из него;
- upload gateway dataplane теперь обслуживает fixed control API:
  - `GET /v1/upload/<sessionID>`
  - `POST /v1/upload/<sessionID>/probe`
  - `POST /v1/upload/<sessionID>/init`
  - `POST /v1/upload/<sessionID>/parts`
  - `POST /v1/upload/<sessionID>/complete`
  - `POST /v1/upload/<sessionID>/abort`
- multipart/raw staging lifecycle теперь живёт в
  `internal/adapters/k8s/uploadsessionstate` as the only per-upload session
  state seam;
- successful multipart completion больше не идёт через upload runtime Pod
  termination message; staged result фиксируется в session `Secret`, после чего
  controller requeues upload into the normal publish-worker path;
- manager shell теперь явно публикует shared upload service name into runtime
  status projection;
- controller docs/config notes synced to shared gateway semantics.

Фактические проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession ./internal/controllers/catalogstatus ./cmd/ai-models-controller ./cmd/ai-models-artifact-runtime`
- `make helm-template`
  - render stage completed and produced updated scenario YAMLs under
    `tools/kubeconform/renders/`
  - final `validate-renders.py` step is currently blocked by local Python
    runtime lacking support for the `list[...] | None` type syntax used by the
    validator script
- `make verify`
  - controller-specific gates passed through file size / thin reconciler
  - later coverage step is currently blocked by `tools/collect-controller-coverage.sh`
    invoking `go` without a PATH that resolves the local Go toolchain
- `git diff --check`

## Slice 31. Add primary preflight validation before full raw ingest

Цель:

- fail fast before large transfers when the request is obviously invalid;
- keep these checks small, cheap and objective instead of pretending to be a
  full security scanner;
- define exactly which checks happen before bytes land in raw storage.

Артефакты:

- `images/controller/internal/application/*`
- `images/controller/internal/domain/*`
- upload/fetch admission docs and bundle notes
- docs/bundle notes

Проверки:

- focused `go test` on upload admission / fetch admission packages
- `make verify`
- `git diff --check`

Статус: landed.

Что вошло:

- landed dedicated fail-fast admission seams:
  - `images/controller/internal/domain/ingestadmission`
  - `images/controller/internal/application/sourceadmission`
- upload session issuance now validates owner binding and declared
  `spec.inputFormat` before the shared gateway session is created;
- upload session `Secret` state now persists cheap preflight metadata needed by
  the shared gateway:
  - owner identity binding;
  - owner generation;
  - declared input format;
  - successful probe result;
- `POST /v1/upload/<sessionID>/probe` is no longer a placeholder:
  - it accepts only a bounded initial chunk;
  - validates filename/path hygiene;
  - checks direct `GGUF` magic and basic archive signature sanity;
  - rejects direct single-file `Safetensors`;
  - persists the successful probe decision and blocks `/init` until probe
    succeeds;
- source-worker runtime now performs a cheap remote preflight before Pod
  creation:
  - owner binding / declared-format allowlist are checked first;
  - `HTTP` sources run `HEAD`, with `Range: bytes=0-0` fallback when `HEAD`
    is unsupported;
  - obvious non-artifact responses such as `text/html` fail closed;
- controller test evidence was extended for the new admission packages, and
  dead shared-gateway leftovers from the old per-upload shape were removed
  so repo quality gates remain green.

Фактические проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH /opt/homebrew/bin/go test ./internal/domain/ingestadmission ./internal/application/sourceadmission ./internal/application/publishplan ./internal/adapters/sourcefetch ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession ./cmd/ai-models-artifact-runtime`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

Ожидаемый результат:

- first-release preflight checks are explicit:
  - authz / owner binding;
  - namespace quota / concurrency guardrails;
  - declared size and input-format allowlist;
  - filename / extension / basic metadata sanity;
  - remote-source `HEAD` / lightweight probe when supported;
  - upload-side bounded `probe` chunk validation before multipart bulk ingest;
- deep malware/content scanning is explicitly not claimed by preflight and
  remains a later async stage.

## Slice 32. Add append-only audit / provenance hooks without a second lifecycle engine

Цель:

- спроектировать future audit trail так, чтобы он не стал вторым source of
  truth;
- зафиксировать, где и какие small records допустимы;
- не дублировать large raw blobs ради истории.

Артефакты:

- controller lifecycle notes
- optional provenance record contract
- docs/bundle notes

Проверки:

- docs consistency check
- `make verify`
- `git diff --check`

Ожидаемый результат:

- future audit/provenance remains append-only and non-authoritative;
- acceptable records are limited to:
  - who/when started upload or remote ingest;
  - canonical raw URI and object metadata;
- validation/profile summary;
- immutable OCI ref/digest and outcome;
- deleting CR still deletes published OCI artifact and raw staging according to
  policy without waiting on any external history system.

Статус: landed.

Реально сделано:

- slice intentionally landed after slices `34` and `35`, once the live byte
  path and raw/publication storage separation were explicit enough to add
  provenance hooks without inventing a second lifecycle engine;
- controller now has an explicit append-only audit sink seam:
  - `internal/ports/auditsink`
  - `internal/adapters/k8s/auditevent`
  - `internal/application/publishaudit`
- the concrete sink is controller-owned `Kubernetes Events`, not a new audit
  CRD or external history service;
- events are emitted only on persisted lifecycle edges, not on speculative
  in-memory transitions:
  - `UploadSessionIssued`
  - `RemoteIngestStarted`
  - `RawStaged`
  - `PublicationSucceeded`
  - `PublicationFailed`
- publish runtime result now carries the minimal internal raw provenance needed
  for append-only history:
  - `RawURI`
  - `RawObjectCount`
  - `RawSizeBytes`
- this provenance lives only in internal runtime/result structures and does not
  expand public `ModelStatus` / `ClusterModelStatus`;
- the landed seam remains non-authoritative:
  `CRD.status` stays the only platform truth for publication state, while
  `DMCR` stays the only storage truth for the published artifact;
- honest residual note remains explicit:
  this slice is not a real async scanner runtime. Future post-ingest scanners
  still need their own dedicated runtime boundary before final readiness,
  especially for `source.url`.

Фактические проверки:

- `cd images/controller && go test ./...`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 33. Add post-ingest scanner hooks and operator audit UX later

Цель:

- предусмотреть future malware/content scanners and audit browsing without
  complicating first release;
- keep security scanning and operator history outside the synchronous upload
  critical path.

Артефакты:

- docs and future integration notes
- optional hook/event sink design only where it remains internal

Проверки:

- docs consistency check
- `make verify`
- `git diff --check`

Ожидаемый результат:

- future scanners run after raw ingest completion and before final readiness;
- operator audit UX reads append-only records, not a second lifecycle engine;
- public `Model` / `ClusterModel` по-прежнему не требуют знания internal audit
  entity model.

## Slice 34. Make large-model publication resource semantics explicit

Цель:

- убрать ложную неопределённость вокруг 1+ TB publication path;
- зафиксировать bounded work-volume semantics для publish worker;
- не оставлять reviewer'у гадать, streaming у нас path или materialized.

Артефакты:

- `images/controller/internal/adapters/k8s/workloadpod/*`
- runtime pod templates/builders
- docs/bundle notes

Проверки:

- focused `go test` on workloadpod/sourceworker/uploadsession builders
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Ожидаемый результат:

- work volume type/size/lifecycle у publish runtime explicit;
- resource requests/limits для memory/cpu/storage заданы явно;
- bundle/docs честно фиксируют worst-case storage amplification until a native
  `ModelPack` encoder replaces the current CLI path.

## Slice 35. Enforce the multi-terabyte publication target path

Цель:

- довести live implementation до одного правильного large-model scenario;
- убрать лишние full-copy paths там, где это возможно без слома semantics;
- зафиксировать raw/audit/dmcr storage separation contract.

Артефакты:

- publishworker/sourcefetch paths
- workload volume/runtime builders
- values/docs notes for bounded work-volume sizing

Проверки:

- focused `go test` on publishworker/sourcefetch/workloadpod
- `make verify`
- `git diff --check`

Ожидаемый результат:

- raw source goes only to controlled object storage first;
- publish worker uses one bounded work volume;
- single-file inputs avoid unnecessary second local full copy when possible;
- same S3 backend may be reused, but raw/audit/dmcr logical separation is
  explicit via prefixes or equivalent backend boundaries.

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

## Slice 34 landed

- `workloadpod` now carries the explicit publish-runtime contract:
  - bounded work volume type (`EmptyDir` or `PersistentVolumeClaim`);
  - bounded work-volume size or claim name;
  - explicit `cpu`, `memory`, `ephemeral-storage` requests/limits.
- `sourceworker` now always passes a fixed bounded `--snapshot-dir`, sets
  `TMPDIR` to that work mount, and materializes the publish container with the
  explicit resource requirements instead of the former implicit `/tmp` path.
- publish concurrency is now an explicit controller knob:
  `catalogstatus` wires `MaxConcurrentWorkers`, and `sourceworker.Service`
  refuses to spawn a new worker Pod when the active publication Pod count has
  already reached the configured cap.
- controller chart now exposes the runtime shell explicitly through internal
  values and args:
  - `aiModels.internal.publicationRuntime.maxConcurrentWorkers`;
  - `aiModels.internal.publicationRuntime.workVolume.*`;
  - `aiModels.internal.publicationRuntime.resources.*`.
- chart also renders an optional module-owned
  `templates/controller/publication-work-pvc.yaml` when
  `workVolume.type=PersistentVolumeClaim`; render validation now covers that
  scenario through `fixtures/render/publication-work-pvc.yaml`.
- `publishworker.ensureWorkspace` no longer treats a provided snapshot root as
  one long-lived directory; it now allocates a per-run subdirectory under that
  bounded root and removes it on exit.
- honest near-term copy budget is now explicit in code/docs:
  - one durable raw copy in object storage;
  - one bounded working set inside the publish worker work volume;
  - one durable published copy in `DMCR`.
- honesty note remains explicit for Slice 35:
  current `sourcefetch` + `KitOps` path is still materialized and may consume
  most of that bounded work volume during archive expansion / pack preparation;
  this slice makes the resource contract explicit, but does not yet remove the
  remaining extra local full-copy paths.

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/workloadpod ./internal/adapters/k8s/sourceworker ./internal/controllers/catalogstatus ./internal/bootstrap ./internal/dataplane/publishworker ./internal/cmdsupport ./cmd/ai-models-controller`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Slice 35 landed

- canonical raw staging is now aligned in live code and tests:
  controller-owned staged bytes use the `raw/` subtree instead of the former
  stale `uploaded-model-staging/` prefix.
- remote `source.url` path now lands bytes into controller-owned raw storage
  first:
  - `HTTP` sources stream the response body into `raw/<owner>/source-url/...`
    and only then materialize a local working copy;
  - `HuggingFace` selected files now follow the same raw-first path file by
    file under the same controller-owned raw subtree.
- direct single-file inputs now avoid an unnecessary second full local byte
  copy when source and checkpoint live on the same filesystem:
  checkpoint preparation is link-first and only falls back to copy when the
  filesystem cannot link.
- raw/publication separation is now explicit in both code and config/docs:
  - raw ingest stays under `raw/`;
  - future audit/provenance stays documented as optional `audit/`;
  - publication object storage now defaults to the isolated `dmcr/` subtree
    instead of the old generic `published-models`.
- raw objects staged for remote publication are deleted after successful
  publish using the same controller-owned object-storage contract as upload
  staging cleanup.
- honest residual note remains explicit:
  remote raw-first staging now happens inside the same bounded publish worker,
  so `source.url -> raw -> workdir -> DMCR` still pays an extra network hop
  through object storage even though it no longer keeps a second full local
  copy for direct-file paths.

Проверки:

- `cd images/controller && go test ./...`
- `make helm-template`
- `make verify`
- `git diff --check`

## Slice 36 landed

- `images/controller/STRUCTURE.ru.md` was rewritten around the live package map
  instead of stale pre-slice names; the document now explicitly covers:
  - `domain/ingestadmission`
  - `application/sourceadmission`
  - `application/publishaudit`
  - `ports/auditsink`
  - `adapters/k8s/auditevent`
  - `publicationartifact`
- shared publication runtime contract is now one direct `publishop.Request`;
  the empty `publishop.OperationContext` wrapper was removed from ports,
  application wiring, concrete runtime adapters, and tests.
- legacy backend-centric package naming was removed from the live controller
  tree:
  - `internal/artifactbackend` was renamed to
    `internal/publicationartifact`
  - the dead `Request` type inside that package was deleted
  - the package now keeps only:
    - publication runtime result payload
    - payload validation/encoding
    - OCI artifact reference policy
- controller docs and evidence were synchronized to the new shape:
  - `README.md`
  - `STRUCTURE.ru.md`
  - `TEST_EVIDENCE.ru.md`
- the refactor intentionally stayed structural and bounded:
  no new fake packages were introduced, and runtime ownership did not move.

Проверки:

- `cd images/controller && go test ./...`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 37 landed

- `sourcefetch` raw-stage orchestration was tightened in place instead of
  spawning another helper package:
  shared response-to-raw-stage upload logic now lives in
  `internal/adapters/sourcefetch/rawstage.go`.
- `http.go` and `huggingface.go` now keep mostly provider-specific concerns:
  URL resolution, auth/headers, metadata, and file-selection flow;
  duplicated object-storage upload handle construction was removed from both.
- new focused coverage was added in `rawstage_test.go`, while existing
  `remote_test.go` and publish-worker tests continue to cover the end-to-end
  raw-stage path.
- the cut intentionally stayed within the same adapter boundary:
  no new package, no new port, no new fake abstraction.

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 38 landed

- `catalogcleanup` owner metadata and DMCR GC request lifecycle were tightened
  in place instead of keeping a second local label-rendering scheme:
  cleanup `Job` and GC request `Secret` now reuse one package-local metadata
  seam over shared `resourcenames` policy.
- GC request reuse no longer restores only the single request label.
  Existing request secrets now get:
  - shared owner labels refreshed;
  - controller app label restored;
  - stale `dmcr-gc-done` cleared;
  - a new GC switch timestamp written.
- the cut stayed within the same delete-only controller owner:
  no new controller package, no new public contract, no new storage truth.
- focused evidence was added in:
  - `internal/controllers/catalogcleanup/gc_request_test.go`
  - expanded `internal/controllers/catalogcleanup/job_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 39 landed

- `modelformat` no longer repeats the same inspect/validate/select flow across
  `Safetensors` and `GGUF`.
  One package-local runner now owns shared traversal, benign-drop handling,
  hidden-directory rejection, and required-file enforcement.
- Format-specific files still keep only format-specific rules:
  - `safetensors.go` keeps safetensors classification plus config/asset
    requirements;
  - `gguf.go` keeps gguf classification plus asset requirement.
- top-level adapter entrypoints became thinner without creating a new generic
  bucket outside the package:
  - `DetectDirFormat`
  - `DetectRemoteFormat`
  - `ValidateDir`
  - `SelectRemoteFiles`
  now all route through the same local rules seam.
- focused detect coverage was added in:
  - `internal/adapters/modelformat/detect_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/modelformat ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 40 landed

- `catalogcleanup` apply path no longer recomputes delete prerequisites on each
  step. A package-local runtime now captures the observed cleanup handle and,
  when needed, the resolved cleanup owner once per finalize apply cycle.
- finalizer release no longer reparses cleanup annotation from the object.
  It now reuses the already observed handle, which avoids coupling the final
  delete step to a second annotation decode over the same object.
- the cut stayed inside the existing controller owner and did not introduce a
  new package boundary:
  the new runtime seam is package-local to `catalogcleanup`.
- focused coverage was added in:
  - `internal/controllers/catalogcleanup/apply_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 41 landed

- `internal/application/deletion` no longer assembles delete policy through a
  long series of ad-hoc `FinalizeDeleteDecision{...}` literals.
  Cleanup-job progress and registry garbage-collection progress now each route
  through explicit package-local step helpers.
- upload-staging and backend-artifact delete flows still stay inside the same
  application seam, but they now compose from the same small decision helpers:
  create-job, pending, GC-request, remove-finalizer, and failure paths.
- the cut stayed inside the existing package boundary:
  no new controller owner, no new adapter, no new public contract.
- focused evidence was added in:
  - `internal/application/deletion/finalize_delete_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 42 landed

- `catalogcleanup` delete owner now carries one package-local finalize flow
  from observation to apply instead of passing raw handle/observation/decision
  pieces separately through reconcile methods.
- the controller no longer performs irrelevant DMCR garbage-collection
  observation for upload-staging cleanup once the cleanup job is complete.
  GC observation is now explicit only for backend-artifact delete flow.
- the cut stayed inside the same delete-only controller owner:
  no new package, no new public contract, no new storage truth.
- focused coverage was added in:
  - `internal/controllers/catalogcleanup/observe_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 43. Virtualization-derived observability contract

Цель:

- поднять реальные observability patterns из sibling `virtualization` module
  без folklore и без blind-copy;
- зафиксировать для `ai-models` минимальный module-owned contract по logging,
  product metrics, alerts и platform integration;
- явно отделить обязательный signal от later/debug noise.

Исследованные reference areas:

- `../virtualization/templates/*service-monitor*.yaml`
- `../virtualization/monitoring/prometheus-rules/*`
- `../virtualization/monitoring/grafana-dashboards/*`
- `../virtualization/images/virtualization-artifact/pkg/logger/*`
- `../virtualization/images/virtualization-artifact/pkg/monitoring/metrics/*`
- `../virtualization/images/virtualization-artifact/pkg/eventrecord/eventrecorderlogger.go`

Выводы, которые становятся contract для `ai-models`:

- platform scrape shell already matches the right baseline:
  protected `ServiceMonitor` + `kube-rbac-proxy` + localhost-only metrics
  bind are the right pattern and must stay;
- alerts must stay symptom-first:
  target down/absent, pod readiness/running, and module-owned storage pressure;
  not controller-internal debug counters;
- product observability must anchor in public object state truth:
  `Model` / `ClusterModel` phase, readiness/validation booleans, small info
  metrics, and artifact size;
- structured logs must be unified across controller and runtime binaries with
  stable context keys and lifecycle-edge logging;
- runtime-local progress/debug metrics may exist for upload/publish internals,
  but are not the primary platform contract.

Артефакты:

- `plans/active/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/OBSERVABILITY_CONTRACT.ru.md`
- current bundle notes in `TASK.ru.md` / `PLAN.ru.md`

Проверки:

- manual consistency check against live `ai-models` deployment shell and
  controller/runtime code

## Slice 44 landed

- first observability implementation cut now follows the previously fixed
  contract instead of inventing metrics first:
  the shared logging shell was aligned across controller and runtime commands.
- controller bootstrap now uses the same component-aware logger factory as the
  phase-2 runtime commands, so both sides keep stable `component` context in
  structured logs.
- `cmd/ai-models-artifact-runtime` now configures one shared root logger per
  selected runtime subcommand (`publish-worker`, `upload-gateway`,
  `artifact-cleanup`) instead of falling back to ad-hoc default stderr logger
  behavior.
- runtime lifecycle-edge logging landed in the live byte path:
  - `publish-worker` logs start/failure/completion with source and artifact
    context;
  - `upload-gateway` logs process start/stop plus successful probe/init/
    complete/abort edges and size-mismatch rejection;
  - `artifact-cleanup` logs cleanup start/failure/completion with handle kind.
- append-only audit sink no longer writes only `Kubernetes Events`:
  `internal/adapters/k8s/auditevent` now mirrors the same audit lifecycle edge
  into structured controller logs with object kind/name/namespace, reason, and
  event type.
- the cut intentionally stayed bounded:
  no new metrics, no new alerts, no dashboard yet.
  Those remain later observability slices after the logging baseline.
- focused evidence was added in:
  - `internal/cmdsupport/common_test.go`
  - `internal/adapters/k8s/auditevent/recorder_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/cmdsupport ./internal/adapters/k8s/auditevent ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/controllers/catalogstatus ./cmd/ai-models-artifact-runtime ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH git diff --check`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`

## Slice 45 landed

- first real product-state metrics now landed on the existing protected
  controller metrics shell instead of inventing another scrape/runtime path.
- `internal/monitoring/catalogmetrics` registers one module-owned collector
  against manager cache and exports only public catalog truth:
  - `Model` / `ClusterModel` phase one-hot gauges;
  - explicit ready/validated booleans;
  - one small info metric with source/format/task/framework/artifact-kind;
  - published artifact size bytes.
- the collector intentionally does not export debug/runtime-local internals:
  no upload session IDs, no artifact URIs, no digest labels, no raw staging
  counters, no reconcile error totals.
- field fallback is explicit and bounded:
  when `status` has not resolved source/format/task yet, metrics fall back to
  public `spec` so new objects still appear in dashboards without waiting for
  terminal publication.
- this cut stayed inside one new observability boundary and one bootstrap
  wiring point:
  no alert rules, no dashboards, no new public API, no second source of truth.
- focused evidence was added in:
  - `internal/monitoring/catalogmetrics/collector_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/monitoring/catalogmetrics ./internal/bootstrap`
- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/cmdsupport ./internal/adapters/k8s/auditevent ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/controllers/catalogstatus ./cmd/ai-models-artifact-runtime ./cmd/ai-models-controller`
- `git diff --check`

## Slice 46 landed

- first module-owned health alerts now landed in the normal DKP monitoring
  shell under `monitoring/prometheus-rules/`, rendered through the required
  `templates/monitoring.yaml` entrypoint instead of being reimplemented as
  controller-local synthetic metrics.
- controller, backend, and DMCR each now have the same bounded health set:
  - scrape target down;
  - scrape target absent;
  - Pod not ready;
  - Pod not running.
- DMCR additionally gets one storage-risk rule for PVC mode using
  `kubelet_volume_stats_*`; the rule stays inert in object-storage mode
  because there is no `dmcr` PVC metric series to evaluate.
- the cut intentionally stayed symptom-first and did not introduce noisy
  runtime/debug alerts on reconcile errors, upload failures, or user-created
  model failures.
- no dashboard JSON in this slice yet; overview/dashboard materialization
  remains the next observability step over the already landed state metrics.

Проверки:

- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 47 landed

- one module overview dashboard now landed under
  `monitoring/grafana-dashboards/main/ai-models-overview.json`, rendered by the
  already landed `templates/monitoring.yaml` shell instead of adding another
  monitoring entrypoint.
- the dashboard is built only from the already landed public catalog metrics
  and platform health/storage signals:
  - controller / backend / DMCR metrics reachability;
  - DMCR PVC free space and usage;
  - `Model` / `ClusterModel` totals;
  - explicit ready / failed / wait-for-upload split;
  - phase distribution;
  - source-type distribution;
  - resolved-format distribution;
  - top published artifact sizes.
- PromQL deduplication is explicit everywhere object counts matter:
  the dashboard uses `max by (namespace, name, uid, ...)` or
  `max by (name, uid, ...)` over the one-hot/object gauges so replicated
  controller scrapes do not double-count the same `Model` /
  `ClusterModel`.
- the cut intentionally stays minimal and platform-shaped:
  no second dashboard, no per-upload traffic views, no per-worker drilldown,
  no runtime-local debug counters.

Проверки:

- `jq empty monitoring/grafana-dashboards/main/ai-models-overview.json`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 48 landed

- upload session auth storage now moved from raw bearer token persistence to
  hash-only persistence in session `Secret` data.
- `internal/adapters/k8s/uploadsessionstate` now writes only `tokenHash`
  for new sessions, accepts old `token` only as a migration input, and rewrites
  legacy session secrets in place to remove the raw token key.
- shared upload gateway no longer compares request bearer/query token against a
  raw secret value; it hashes the presented token and compares it to the stored
  hash before serving `probe/init/parts/complete/abort`.
- controller-side upload-session reconcile keeps the existing user UX:
  - on new session issue, controller still returns one raw tokenized URL set in
    `status.upload`;
  - on later reconciles, controller recovers the raw token from the already
    persisted `status.upload` URL when it needs to rebuild status;
  - if persisted upload status is missing or unusable while the session is
    still active, controller rotates the token, rewrites the secret hash, and
    republishes fresh upload URLs.
- the cut stayed bounded:
  no API contract redesign, no new session object, no scanner changes, no
  upload gateway protocol changes beyond hash-based verification.
- focused evidence was added in:
  - `internal/adapters/k8s/uploadsessionstate/secret_test.go`
  - `internal/adapters/k8s/uploadsession/service_test.go`
  - `internal/dataplane/uploadsession/run_test.go`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./cmd/ai-models-artifact-runtime`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 49 landed

- upload session state is now less fake and less implicit:
  - the session `Secret` persists a server-side multipart part manifest;
  - explicit session phases now cover at least `issued`, `probing`,
    `uploading`, `uploaded`, `failed`, `aborted`, `expired`;
  - `GET /v1/upload/<sessionID>` now exposes the richer session state,
    including current phase, failure message, and persisted multipart parts.
- the shared gateway no longer keeps resumability only in `uploadID`:
  it now queries object storage for uploaded parts, persists that manifest back
  into the session `Secret`, and uses it again before `/complete` to validate
  the final part list against the server-side multipart state.
- expiry is no longer only implicit in `expiresAt`:
  gateway requests and controller-side session reuse now both persist explicit
  `expired` state when a live session is already past TTL.
- the upload-staging port now includes `ListMultipartUploadParts`, and the S3
  adapter implements that read path so upload session resumability/state sync
  does not depend on client memory alone.
- the cut stayed bounded and did not lie about later lifecycle ownership:
  - no CRD/status expansion;
  - no new upload API endpoint;
  - no scanner/runtime-delivery work;
  - no fake `publishing/completed` state while the controller still deletes
    the upload session after raw staging handoff.
- repo memory was synced in:
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/sourcefetch ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./internal/adapters/uploadstaging/s3 ./cmd/ai-models-artifact-runtime`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 50 landed

- upload-session lifecycle is now honest across the controller handoff:
  - `publishing/completed` are no longer fake future labels;
  - controller keeps the successful runtime alive through the cleanup-handle
    requeue and only deletes it after the post-status reconcile that persists
    final `Publishing` / `Ready` / `Failed`;
  - upload-source reconciles now sync the session `Secret` lifecycle through
    `publishing`, `completed`, and publish-time `failed` from the controller
    side instead of deleting the session right after raw staging.
- `DecideCatalogStatusReconcile` now keeps upload objects on the
  `SourceWorker` path while the same-generation object is still in
  `Publishing` with a persisted cleanup handle, so upload sources do not drift
  back into `UploadSession` after successful source-worker result handoff.
- shared upload gateway now treats `uploaded/publishing/completed` as closed
  mutation phases:
  late `probe/init/parts/complete/abort` attempts fail closed even though the
  persisted multipart manifest is still available for inspection.
- the cut stayed bounded:
  - no CRD/status contract change;
  - no new session object or janitor;
  - no new persisted bus between controller, upload session, and publish
    worker.
- repo memory was synced in:
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/publishobserve ./internal/controllers/catalogstatus ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Slice 51 landed

- user-facing storage contract is now honest and simpler:
  - phase-1 MLflow backend remains in place and still owns the PostgreSQL
    requirement for metadata/auth storage;
  - shared S3-compatible `artifacts` storage is the only supported byte path
    for MLflow artifacts, controller-owned raw ingest, and internal DMCR
    publication bytes;
  - inline `artifacts.accessKey` / `artifacts.secretKey` are removed from the
    public contract in favor of required `artifacts.credentialsSecretName`;
  - user-facing `publicationStorage` and the PVC-backed DMCR branch are
    removed from schema, values, templates, fixtures, and docs.
- DMCR render/runtime shell is now S3-only:
  - `templates/dmcr/configmap.yaml` always renders the `s3` driver with a
    fixed `/dmcr` rootdirectory;
  - `templates/dmcr/deployment.yaml` no longer switches between filesystem and
    object storage, and HA rollout logic is now independent from a fake PVC
    branch;
  - `templates/dmcr/pvc.yaml` is deleted.
- artifact credentials are now reference-only:
  - `templates/module/artifacts-secret.yaml` is deleted;
  - `artifactsResolvedSecretName` resolves directly to the referenced Secret;
  - validation now fails fast when `artifacts.credentialsSecretName` is empty.
- observability contract was tightened to match the new storage reality:
  - the PVC-only DMCR capacity alert was removed;
  - the module overview dashboard no longer shows fake DMCR PVC capacity
    panels.

Проверки:

- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make helm-template`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`
