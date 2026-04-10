# REVIEW

## Итог

Phase-2 publication/runtime execution path больше не живёт в backend Python
scripts. Live path теперь controller-owned and Go-first:

- manager binary and phase-2 runtime binary are now split:
  - `cmd/ai-models-controller` for manager
  - `cmd/ai-models-artifact-runtime` for `publish-worker`,
    `upload-session`, and `artifact-cleanup`
- source acquisition moved into Go adapters
- source-agnostic ingest validation now lives in a dedicated Go adapter and
  runs before `ModelPack` packaging for `HuggingFace`, `HTTP`, and `Upload`
- `ModelPack` publish/remove moved behind an explicit Go port with current
  `KitOps` implementation adapter
- ai-inference-oriented resolved metadata now lands in public status with a
  stricter calculation path for current live formats
- dedicated runtime image owns the pinned `KitOps` binary
- controller runtime images now build from a module-local `distroless`
  relocation layer instead of pulling `base/distroless` directly
- `KitOps` delivery now has its own artifact stage instead of being hidden
  inside the Go build stage
- the lone `KitOps` installer script now sits in the controller root next to
  `kitops.lock` instead of creating a fake one-file `tools/` boundary
- backend image no longer carries phase-2 publication/upload/cleanup execution
  entrypoints
- render fixtures now include the dedicated `controllerRuntime` digest key for
  the `controller-runtime` image, so the
  new image path is covered by `helm-template`/`kubeconform` instead of being
  invisible to template validation
- controller registration now uses explicit unique names across
  `catalogcleanup` and `catalogstatus`, so startup no longer dies on duplicate
  controller-runtime controller/metric names
- `controller-kitops-artifact` and `distroless-artifact` no longer render a
  malformed `beforeInstall` command list after `alt packages proxy`; their
  `apt-get install` entries are back on the correct YAML level and no longer
  collapse into one broken `apt-get` invocation during `werf build`
- `controller-kitops-artifact` no longer uses bogus per-file
  `stageDependencies.install` paths that werf expanded into nonexistent
  `images/controller/<file>/<file>` lookups; both imported files are now
  tracked through the standard wildcard dependency pattern
- the same KitOps stage now creates `/root/.local/share/kitops` before
  disabling update notifications, so `kit version --show-update-notifications=false`
  no longer aborts during image build
- root `.helmignore` now follows the same DKP module pattern as
  `gpu-control-plane` / `virtualization`, so Helm ignores generated `hooks`,
  docs, openapi, CRDs and root markdown when loading the chart from the bundle
- this removes the operator-startup drift where the generated
  `hooks/go/ai-models-module-hooks` binary leaked into the Helm chart payload
  and tripped the 5 MiB per-file chart limit
- controller bootstrap now sets the root `slog` logger as the default logger
  for both `controller-runtime` and `klog`, so controller-runtime internals no
  longer fall back to the empty promised logger and emit the 30-second warning
- controller and DMCR metrics no longer scrape directly from plaintext pod
  endpoints:
  - controller metrics now bind to `127.0.0.1` inside the Pod
  - DMCR debug/metrics now bind to `127.0.0.1:5001`
  - both components export metrics only through kube-rbac-proxy sidecars over
    HTTPS, with `ServiceMonitor` resources aligned to the virtualization-style
    protected scrape path
- controller and DMCR deployment shells now follow the DKP production pattern
  more closely:
  - both workloads use `system-cluster-critical` priority class
  - both workloads use HA anti-affinity instead of naked replica scaling
  - both workloads use read-only-rootfs container security helpers
  - controller is pinned to control-plane nodes through module-local fail-safe
    selector/toleration helpers
  - DMCR is pinned to `system` nodes with a control-plane fallback through the
    same module-local fail-safe helper layer
  - both workloads now get VPA objects when
    `vertical-pod-autoscaler-crd` is enabled
  - DMCR readiness/liveness now check the actual HTTPS registry endpoint
    instead of only probing an open TCP socket
- root chart now consumes vendored `deckhouse_lib_helm` through the normal
  DKP dependency path and no longer needs a repo-local helper fork in
  `templates/`
- external-registry-centric publication wiring is gone from the live module
  contract:
  - a module-local internal DMCR image now builds under `images/dmcr`
  - `dmcr` is now a repo-owned Go binary under `images/dmcr/cmd/dmcr`, not a
    build-time git clone of registry source
  - the wrapper imports only the storage/auth drivers that ai-models actually
    uses (`filesystem`, `s3-aws`, `htpasswd`), so the module no longer drags
    in dead `gcs`/`azure`/`xds` ballast
  - image build follows the normal Go module and `GOPROXY` path instead of a
    checked-in `vendor/` tree
  - the module now deploys its own internal registry service under
    `templates/dmcr`
  - controller publication workers and cleanup jobs now always target that
    internal registry service
  - user-facing storage semantics are expressed as `publicationStorage`
    (`ObjectStorage` or `PersistentVolumeClaim`) instead of leaking registry
    endpoint/credential wiring
  - destructive cleanup now also includes physical DMCR blob reclamation:
    - the controller creates an explicit internal GC request after remote
      artifact deletion
    - hooks switch DMCR into maintenance/read-only mode only while such
      requests exist
    - repo-owned `dmcr-cleaner` runs registry garbage collection and marks the
      request complete before finalizer removal
- current structural hotspots are now cleaner in-place instead of being hidden
  behind new generic packages:
  - `catalogcleanup` no longer keeps one monolithic generic `io.go`; observer,
    apply, status, and GC-request responsibilities are split inside the same
    package boundary
  - `dmcr-cleaner/cmd` is back to a thin CLI shell, while the actual garbage
    collection loop moved into `images/dmcr/internal/garbagecollection`

## Проверки

- `cd images/controller && go test ./...`
- `make verify`
- `make helm-template`
- `make kubeconform`
- `werf config render --dev --loose-giterminism`
- `werf build --dev --platform=linux/amd64 --loose-giterminism dmcr controller controller-runtime`
- direct PVC-mode render via the same repo Helm toolchain to prove the second
  storage branch of `publicationStorage`
- `git diff --check`

Validation note:

- repo-level checks passed, including `make verify` and `git diff --check`;
- chart/schema validation passed via `make helm-template`, `make kubeconform`,
  and `werf config render --dev --loose-giterminism`;
- the current DMCR garbage-collection slice also passed focused Go tests in
  `images/dmcr`, `images/hooks`, and controller deletion/cleanup packages;
- the structural cleanup slice additionally passed focused tests for
  `catalogcleanup` and the new `images/dmcr/internal/garbagecollection`
  package;
- a rerun of targeted end-to-end image build for
  `dmcr/controller/controller-runtime` could not be completed in this session
  because local Docker daemon access was unavailable
  (`Cannot connect to the Docker daemon at unix:///Users/myskat_90/.docker/run/docker.sock`);
- the `PersistentVolumeClaim` branch of the new internal publication storage
  was also rendered directly with the repo Helm toolchain and produced the
  expected `dmcr` PVC and filesystem-backed `rootdirectory` path.

## Архитектурный эффект

- virtualization-style ownership improved:
  - controller/runtime data plane in Go
  - build/install shell only for tool installation
  - controller runtime base image is now module-owned at the werf layer too,
    not only at the Go code layer
  - external `KitOps` packaging is now an explicit runtime artifact seam,
    not hidden inside controller compilation
  - hidden backend artifact plane is now a real module-local internal DMCR
    service behind the OCI/ModelPack contract
- public input contract is now simpler:
  - users provide `spec.source`
  - users provide `spec.inputFormat`
  - `spec.source` is either `source.url` or `source.upload`
  - `spec.inputFormat` can stay empty when the format is clear from contents
  - fixed internal `ModelPack` output stays implicit
- current live input formats are `Safetensors` and `GGUF`
- top-level docs no longer falsely claim that runtime materialization into a
  local path is already wired; the live module still stops at publication into
  internal `ModelPack`/OCI and keeps ai-inference delivery as a separate
  future slice
- direct single-file `GGUF` now works on generic `HTTP` and `Upload` paths;
  archive uploads also keep their original filename through the upload session
- benign extras are stripped before packaging
- active/ambiguous files are rejected fail-closed
- remote source acquisition now has one canonical remote ingest entrypoint over
  shared provider-specific fetch helpers, so `publishworker` no longer repeats
  `HuggingFace` vs `HTTP` download/orchestration shell
- `sourceworker` and `uploadsession` no longer keep a separate replay-read path
  before the same `CreateOrGet` ensure cycle
- projected auth secret handling in `sourceworker` no longer keeps adapter-local
  `Get/Create/Update`; it now uses one direct reconcile path too
- `uploadsession` no longer keeps a separate request-mapping file; the only
  remaining mapping helper lives locally next to the pod builder
- public contract stays:
  - `ModelPack` in OCI
  - runtime input only `OCI from the internal DMCR registry`
- backend phase-1 runtime remains untouched for MLflow-oriented concerns
- chart render path is now honest:
  - `deckhouse_lib_helm` comes from the vendored library chart in `charts/`
  - `.helmignore` no longer drops the library dependency during `helm template`
  - helper ownership matches `gpu-control-plane` / `virtualization` patterns
- live module publication shell is now also honest:
  - no external `publicationRegistry` values contract survives
  - controller runtime wiring always resolves to the internal registry service
  - module storage semantics are modeled as `publicationStorage`, not as an
    exposed registry URL/credential bundle
- destructive cleanup now also matches the internal artifact-plane ownership
  more closely:
  - delete finalizer no longer stops after manifest/reference removal
  - physical blob cleanup is coordinated through explicit internal GC request
    state and a repo-owned `dmcr-cleaner` lifecycle
- current package boundaries are also closer to their declared ownership:
  - `catalogcleanup` keeps one package boundary but no longer hides four
    different responsibilities behind one generic file
  - `dmcr-cleaner` no longer mixes CLI parsing with kube polling and registry
    process execution in the same command file
- remaining `KitOps` debt is now supply-chain provenance, not controller/runtime
  ownership: release asset fetching is still external, but no longer mixed into
  the Go build path

## Остаточный долг

- runtime delivery to `ai-inference` is still not wired
- upload path is still a real architectural debt:
  `uploadsession` keeps `receive bytes -> publish` in one synchronous runtime
  critical section, so large uploads still occupy one ephemeral pod for the
  whole ingest+publish path
- `KitOps` still remains an external CLI adapter inside the phase-2 runtime
  image; this is now explicit and isolated, but it is still a process
  boundary and a failure source until a native modelpack publication path
  lands
- restored public policy fields already have live semantics, but
  `optimization.speculativeDecoding.draftModelRefs` is intentionally narrow:
  the current slice only validates local profile compatibility and does not yet
  resolve referenced draft models cross-object

## Дополнительный review по slice 19 и 22

Findings:

1. Medium: public policy contract is now honest, but only because the scope was
   kept narrow. `modelType`, `usagePolicy`, `launchPolicy` and
   `optimization` are acceptable in `spec` precisely because they now affect
   `Validated` / `Ready` through live controller semantics. The same discipline
   must be kept for any future policy growth.
2. Medium: `sourcefetch` and `modelformat` are cleaner than before, but not
   magically small. The corrective split improved ownership and stopped file
   monolith growth; it did not remove the need for the next deeper slices on
   upload/session redesign and modelpack publication.

Checks:

- `cd api && bash scripts/update-codegen.sh`
- `cd api && go test ./...`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./internal/domain/publishstate ./internal/application/publishobserve ./internal/controllers/catalogstatus`
- `cd images/controller && go test ./internal/adapters/modelformat ./internal/adapters/sourcefetch`
- `cd images/controller && go test ./...`
- `make verify`
- `git diff --check`

## Дополнительный review по slice 23

Findings:

1. Medium: previous target architecture text was too short and therefore hid
   critical truth about the remaining upload/data-plane work. The rewritten
   version is better because it explicitly states that current
   `port-forward`-based synchronous upload is not the target architecture.
2. Medium: auth/trust/authorization around internal `DMCR` were previously
   under-specified. The new target text now makes write-vs-read credential
   split, session-scoped projection, CA-bundle distribution and cleanup
   lifecycle explicit, closer to the real `virtualization` pattern.
3. Medium: the new target text still does not claim that consumer delivery is
   landed. This is correct. Any future doc that describes `ai-inference`
   materialization as live before the code exists should be treated as a real
   regression.

Checks:

- manual diff review against `virtualization` upload/auth docs
- `make verify`
- `git diff --check`

## Дополнительный review по slice 24

Findings:

1. Medium: live upload edge is now materially closer to `virtualization`
   because `source.upload` projects session URLs instead of a
   `kubectl port-forward` helper command. This is a real improvement in the
   control-plane contract and in chart/runtime ownership.
2. Medium: the slice intentionally does not claim the upload architecture is
   finished. The same runtime still performs `receive bytes -> publish`, so
   the main remaining drift is now the synchronous critical path itself, not
   the user-facing edge.
3. Medium: keeping `status.upload.command` in the type as an empty legacy field
   is acceptable only as a compatibility bridge. A future slice should either
   remove it from the public contract or mark it explicitly deprecated in docs
   once external consumers are confirmed.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/application/publishobserve ./internal/domain/publishstate ./internal/controllers/catalogstatus ./internal/bootstrap ./internal/ports/publishop ./internal/dataplane/uploadsession`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Дополнительный review по slice 25

Findings:

1. Medium: publication-side `DMCR` auth/trust is now materially closer to the
   real `virtualization` pattern because worker/session/cleanup runtimes no
   longer read the module root credential secret directly.
2. Medium: splitting server, write, and read secrets removes a real ownership
   bug. The registry server now consumes only htpasswd material, while
   controller-owned runtimes consume projected client creds.
3. Medium: the slice intentionally does not claim consumer-side auth/trust is
   finished. Read-only projection into a future materializer / `ai-inference`
   runtime is still absent, and that is now the main remaining drift.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/ociregistry ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/controllers/catalogcleanup ./internal/bootstrap ./internal/support/resourcenames`
- `make helm-template`

## Дополнительный review по slice 26

Findings:

1. Medium: upload path now actually matches the staged async architecture it
   previously only described. The upload runtime no longer mixes byte receive
   with publication, and controller-owned requeue between upload session and
   publish worker is the right control-plane boundary.
2. Medium: removing `status.upload.command` is correct. Keeping an empty legacy
   field would have preserved a fake contract after the user-facing upload edge
   had already moved to session URLs.
3. Medium: the main honest upload drift is now narrower and more concrete:
   the system still lacks direct multipart/presigned staging for very large
   uploads, but the monolithic `receive -> publish` runtime smell is gone.

Checks:

- `cd images/controller && go test ./internal/... ./cmd/ai-models-artifact-runtime/...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Дополнительный review по slice 27

Findings:

1. Medium: keeping upload-staging env parsing in `internal/cmdsupport` was a
   real boundary leak because shared process glue knew a concrete S3 adapter.
   Moving it back under `cmd/ai-models-artifact-runtime` is the correct
   binpack.
2. Medium: removing `WriteTerminationResult` from `cmdsupport` is correct for
   the same reason. Encoding `artifactbackend.Result` is publish-worker shell
   logic, not shared command support.
3. Medium: this slice intentionally does not claim that all runtime-shell drift
   is gone. It only removes the concrete fake seam that had already appeared
   after the staged upload refactor.

Checks:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/cmdsupport ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup`
- `make verify`
- `git diff --check`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Оставшиеся drifts против virtualization / gpu-control-plane

- backend Python build/runtime stages still use raw external `python:` bases;
  unlike the former raw `node:` UI stages, this part still has no mapped
  module-owned replacement in the current repo base-image set and remains the
  main honest shell debt.

## Текущая сверка с ADR

Отдельный audit зафиксирован в
[ADR_AUDIT.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/plans/active/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/ADR_AUDIT.ru.md).

Короткий вывод:

- текущий `status` уже живой и в основном лучше структурирован, чем в ADR;
- текущий `spec` заметно ушёл от ADR;
- сам ADR сейчас нельзя считать точным описанием текущего public contract;
- в самом CRD главный remaining drift уже не в dead knobs, а в общем
  расхождении текущего `spec` с историческим ADR.
