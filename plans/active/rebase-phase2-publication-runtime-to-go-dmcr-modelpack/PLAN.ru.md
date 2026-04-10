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
  `artifactbackend.Result` как shared helper;
- termination result encoding для publish-worker теперь живёт в самом runtime
  command path, а shared `cmdsupport` снова ближе к process-level glue.

Фактические проверки:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/cmdsupport ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup`

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
