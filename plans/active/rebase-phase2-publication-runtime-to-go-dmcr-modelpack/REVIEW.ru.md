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
- controller and phase-2 runtime commands now share the same component-aware
  logger shell, so structured logs no longer diverge between
  `ai-models-controller`, `publish-worker`, `upload-gateway`, and
  `artifact-cleanup`
- append-only publication audit no longer exists only as `Kubernetes Events`:
  the same lifecycle edges are now mirrored into structured controller logs
  with stable object/reason/event-type attrs
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
  - `uploadsession` is no longer a raw byte receiver:
    it now exposes a small multipart session API and persists multipart state
    in the existing upload Secret while staged bytes go directly to object
    storage over presigned part URLs

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
- the observability logging slice additionally passed focused tests for:
  - `internal/cmdsupport`
  - `internal/adapters/k8s/auditevent`
  - `internal/dataplane/uploadsession`
  - `internal/dataplane/publishworker`
  - `internal/controllers/catalogstatus`
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
- `KitOps` still remains an external CLI adapter inside the phase-2 runtime
  image; this is now explicit and isolated, but it is still a process
  boundary and a failure source until a native modelpack publication path
  lands
- upload path still has remaining work, but the debt moved:
  direct multipart staging is landed for object storage; the honest remaining
  gap is a future PVC-specific uploader/staging path and richer resumable
  client protocol if product requirements ever demand it
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

## Дополнительный review по slice 28

Findings:

1. Medium: `uploadsession` now matches the intended boundary much better.
   The Pod is no longer a raw byte ingress plus heavy publication worker in one
   process. It is a session/control-plane runtime over a real multipart
   staging contract, which is materially closer to the `virtualization`
   uploader pattern.
2. Medium: the new `internal/adapters/k8s/uploadsessionstate` package is
   justified. It is not a fake helper bucket; it is the concrete K8s adapter
   for the upload-session state-store port and keeps Secret CRUD out of both
   `cmdsupport` and the dataplane use case.
3. Medium: the remaining honest gap is no longer “direct multipart staging”.

## Дополнительный review по slice 31

Findings:

1. No blocking findings for the landed fail-fast cut. The new admission code is
   bounded, lives in explicit `application/` and `domain/` seams, and does not
   reintroduce per-upload Pod drift or fake helper buckets.
2. Medium: the current concurrency guardrail is still only structural
   per-owner single-session reuse via one session `Secret` per object. A real
   namespace-wide upload quota/budget knob is still absent and remains future
   work if product policy requires it.
3. Medium: upload-session auth still uses the raw session token stored in the
   session `Secret`. This slice improved owner binding and probe admission, but
   it did not yet switch the shared gateway to token-hash-only persistence from
   the target-architecture notes.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH /opt/homebrew/bin/go test ./internal/domain/ingestadmission ./internal/application/sourceadmission ./internal/application/publishplan ./internal/adapters/sourcefetch ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession ./cmd/ai-models-artifact-runtime`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`
   That is landed for object storage. The remaining gap is a future
   PVC-specific uploader/staging path plus consumer-side runtime delivery.

Checks:

- `cd images/controller && go test ./internal/dataplane/uploadsession ./internal/adapters/uploadstaging/s3 ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./cmd/ai-models-artifact-runtime`
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
   the same reason. Encoding `publicationartifact.Result` is publish-worker shell
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

## Дополнительный review по slice 30

Findings:

1. Medium: the main structural drift is actually removed here. Upload
   concurrency now scales through one shared gateway shell plus one session
   `Secret` per upload, instead of one `Pod/Service/Ingress` trio per model.
2. Medium: the session `Secret` is now the only per-upload control-plane state
   seam, which is the correct replacement for the former upload runtime Pod
   lifecycle. This keeps `CRD.status` as the platform truth while removing the
   fake runtime-object source of truth.
3. Medium: `/probe` is intentionally only a control-API placeholder in this
   slice. That is acceptable only because Slice 31 is explicitly next and the
   docs do not misrepresent probe as a finished security/preflight stage.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession ./internal/controllers/catalogstatus ./cmd/ai-models-controller ./cmd/ai-models-artifact-runtime`
- `make helm-template`
  - scenario renders completed
  - final Python validator step is blocked by local interpreter compatibility
- `make verify`
  - controller-specific gates passed until coverage collection
  - coverage artifact step is blocked by local shell PATH not resolving `go`
- `git diff --check`

## Дополнительный review по slice 34

Findings:

1. Medium: publish runtime semantics are now explicit end-to-end.
   Controller shell, worker Pod builder and shared `workloadpod` helper all
   agree on one bounded work-volume contract plus explicit
   `cpu`/`memory`/`ephemeral-storage` requests and limits, so the old
   “implicit `/tmp` and hope” drift is gone.
2. Medium: upload concurrency and publish concurrency are now materially
   separated. Active publish workers are capped before Pod creation, and the
   controller keeps requeueing the deterministic worker name instead of
   inventing a second queue/state owner.
3. Medium: Slice 34 is honest but intentionally incomplete on the byte path.
   The bounded work volume now contains the full materialized publication
   working set, but current `sourcefetch` + `KitOps` behavior may still spend
   most of that budget inside the same volume until Slice 35 removes the
   remaining unnecessary local full-copy paths.

Checks:

- `cd images/controller && go test ./internal/adapters/k8s/workloadpod ./internal/adapters/k8s/sourceworker ./internal/controllers/catalogstatus ./internal/bootstrap ./internal/dataplane/publishworker ./internal/cmdsupport ./cmd/ai-models-controller`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Дополнительный review по slice 35

Findings:

1. Medium: the byte path is now materially aligned with the target architecture.
   Upload staging and remote `source.url` acquisition both use the
   controller-owned `raw/` subtree before local materialization, and the
   publication backend default is now an explicit `dmcr/` subtree instead of a
   vague generic prefix.
2. Medium: the remaining local copy amplification is narrower and honest.
   Direct single-file inputs now materialize into `checkpoint/` via link-first
   staging when possible, so the former unavoidable second full local copy is
   gone on the normal same-filesystem path.
3. Medium: the residual large-model cost is now mostly a network budget, not a
   node-disk ambiguity. Remote raw-first staging still happens inside the same
   bounded publish worker, so `source.url -> raw -> workdir -> DMCR` pays an
   extra object-storage hop even though the node no longer keeps the extra full
   local copy for direct-file inputs.

Checks:

- `cd images/controller && go test ./...`
- `make helm-template`
- `make verify`
- `git diff --check`

## Дополнительный review по slice 32

Findings:

1. No blocking findings for the landed append-only cut. The new seam is
   internal, keeps public `status` unchanged, and records only small lifecycle
   facts instead of building a second publication state machine.
2. Medium: the concrete implementation is intentionally modest.
   Controller-owned `Kubernetes Events` plus minimal internal raw provenance
   (`RawURI`, raw object count, total raw size) are acceptable precisely
   because they are emitted only after persisted lifecycle edges and are not
   consumed as readiness truth.
3. Medium: this is not async scanner execution. Real post-ingest scanners
   still require a later dedicated runtime boundary between raw ingest
   completion and final `Ready`, especially for remote `source.url` flows.

Checks:

- `cd images/controller && go test ./...`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 36

Findings:

1. No blocking findings for the landed structure refactor. The controller tree
   is now more honest: live docs were synchronized to the actual packages, the
   dead `publishop.OperationContext` wrapper is gone, and the old
   backend-centric `artifactbackend` name no longer survives in a
   controller-owned boundary.
2. Medium: keeping `publishedsnapshot` and `publicationartifact` separate is
   correct. They are close, but they are not duplicates:
   one is the full internal publication snapshot, the other is the narrower
   runtime payload plus OCI reference policy.
3. Medium: the main remaining structural hotspots are still the same real
   ones, not naming accidents:
   `catalogcleanup`, `sourcefetch`, and `modelformat`.
   Future work should keep shrinking them in place instead of inventing new
   generic packages.

Checks:

- `cd images/controller && go test ./...`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 37

Findings:

1. No blocking findings for the landed `sourcefetch` cleanup. The new helper
   stays inside the same package boundary and removes real duplication instead
   of inventing a new generic utility bucket.
2. Medium: the cut improves package honesty, not architecture scope.
   `http.go` and `huggingface.go` now carry more clearly provider-specific
   logic, while shared raw-stage object handoff lives in `rawstage.go`.
3. Medium: the main remaining `sourcefetch` debt is still size and provider
   growth pressure, not the raw-stage flow itself. If new providers land later,
   they should keep reusing the same package-local raw-stage seam instead of
   cloning upload/download glue again.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 38

Findings:

1. No blocking findings for the landed `catalogcleanup` cut. The package keeps
   the same delete-only controller ownership while removing a real metadata
   drift: cleanup job and GC request objects no longer build owner labels
   through separate raw string maps.
2. Medium: refreshing full owner metadata on existing GC request secrets is the
   right semantics. Re-arming only the request label was too narrow because it
   left older or partially populated request secrets with stale controller
   metadata.
3. Medium: this slice intentionally does not split `catalogcleanup` further.
   The main remaining controller debt is still package size and decision flow
   density, not object-metadata duplication.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 39

Findings:

1. No blocking findings for the landed `modelformat` cleanup. The new runner
   stays package-local and removes real duplication instead of inventing a new
   generic support layer outside the adapter boundary.
2. Medium: the cut is correct because ownership stayed explicit.
   `Safetensors` and `GGUF` files still define their own classification and
   required-file semantics; only the repeated traversal and state aggregation
   moved into one local seam.
3. Medium: this does not make `modelformat` "done".
   The remaining future pressure is new format families, not the old duplicate
   inspect/validate/select loops.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/modelformat ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 40

Findings:

1. No blocking findings for the landed `catalogcleanup` apply cleanup. The new
   runtime seam stays inside the same controller package and removes real
   duplicate prerequisite work instead of inventing another controller or
   adapter layer.
2. Medium: reusing the observed cleanup handle for finalizer release is the
   correct semantics. The previous extra annotation parse was not adding new
   information and made the final step depend on a second decode of data that
   had already been observed.
3. Medium: this slice intentionally does not redesign the delete decision
   protocol. The main remaining debt in `catalogcleanup` is still decision-flow
   density, not prerequisite recomputation.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 41

Findings:

1. No blocking findings for the landed `application/deletion` cleanup. The
   refactor stayed inside the same policy seam and made the delete protocol
   more explicit without inventing another controller or adapter boundary.
2. Medium: extracting package-local step helpers is the correct granularity
   here. `cleanupJobProgressDecision` and `garbageCollectionProgressDecision`
   remove real repetition while preserving one canonical `FinalizeDeleteDecision`
   shape for both upload-staging and backend-artifact delete flows.
3. Medium: this slice intentionally does not redesign `catalogcleanup`
   ownership. The remaining delete-path pressure is still in controller-level
   orchestration density, not in the application-layer decision table.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 42

Findings:

1. No blocking findings for the landed `catalogcleanup` flow cleanup. The
   refactor stayed inside the same delete-only controller owner and made the
   observe/decide/apply handoff more explicit instead of spreading partial
   delete state across reconcile calls.
2. Medium: skipping DMCR GC observation for completed upload-staging cleanup is
   the correct semantics. That branch has no registry artifact to garbage
   collect, so reading the GC request secret there was pure controller drift.
3. Medium: this slice intentionally does not split `catalogcleanup` into
   another package. The remaining pressure is still controller size and
   lifecycle density, not missing helper abstractions.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/bootstrap ./cmd/ai-models-controller`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
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

## Дополнительный review по slice 45

Findings:

1. No blocking findings for the landed product-state metrics baseline. The new
   collector reads only public `Model` / `ClusterModel` truth from manager
   cache and does not introduce another persisted or runtime-local source of
   observability truth.
2. Medium: falling back from unresolved `status` to public `spec` for source
   type / format / task is the correct tradeoff for this slice. It keeps new
   objects visible in dashboards without inventing hidden controller-side state
   labels.
3. Medium: including `Deleting` in the phase one-hot set is more correct than
   silently dropping that public phase. The observability contract note was
   synced accordingly.
4. Medium: this slice intentionally stops before alerts and dashboards. The
   remaining observability debt is now in `PrometheusRule` and dashboard
   materialization, not in missing public catalog metrics.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/monitoring/catalogmetrics ./internal/bootstrap`
- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/cmdsupport ./internal/adapters/k8s/auditevent ./internal/dataplane/uploadsession ./internal/dataplane/publishworker ./internal/controllers/catalogstatus ./cmd/ai-models-artifact-runtime ./cmd/ai-models-controller`
- `git diff --check`

## Дополнительный review по slice 46

Findings:

1. No blocking findings for the landed health-rule baseline. The rules stay in
   the normal module monitoring shell and alert only on platform symptoms:
   scrape availability, Pod readiness/running, and DMCR PVC pressure.
2. Medium: keeping the DMCR storage-risk rule on `kubelet_volume_stats_*`
   instead of inventing a custom capacity metric is the correct
   virtualization-style choice. This preserves one platform source of truth for
   PVC pressure.
3. Medium: backend Pod match uses the concrete deployment pod-name shape
   (`ai-models-<hash>-<suffix>`) to avoid colliding with
   `ai-models-controller-*`. This is acceptable for the current fixed module
   shell, but any future backend deployment rename must update the rule.
4. Medium: this slice intentionally stops before overview dashboards. The main
   remaining observability debt is dashboard materialization over the new
   `d8_ai_models_*` state metrics.

Checks:

- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 47

Findings:

1. No blocking findings for the landed overview dashboard baseline. It reads
   only the already landed public `d8_ai_models_*` catalog metrics and normal
   platform health/storage series; it does not introduce a second
   observability source of truth.
2. Medium: using `max by (namespace, name, uid, ...)` /
   `max by (name, uid, ...)` before `sum`/`count` is the correct dedupe pattern
   here. The controller metrics endpoint may be scraped from more than one pod,
   so naive aggregation would overcount catalog objects.
3. Medium: surfacing `WaitForUpload` directly in overview stats is the correct
   platform signal. It shows catalog objects blocked on user upload without
   turning ordinary user wait states into alerts.
4. Medium: this slice intentionally stays at one module overview dashboard.
   The remaining observability debt is drilldown/dashboard depth only if a real
   operator use case appears later, not a missing baseline overview.

Checks:

- `jq empty monitoring/grafana-dashboards/main/ai-models-overview.json`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 48

Findings:

1. No blocking findings for the landed upload-session token hardening. New
   session secrets no longer persist the raw bearer token, and the gateway now
   authenticates by comparing `hash(request token)` with the stored hash.
2. Medium: in-place legacy secret migration is the correct bounded choice here.
   Existing sessions are upgraded lazily on first reuse instead of breaking
   active uploads during rollout.
3. Medium: recovering the raw token from already persisted `status.upload`
   during later reconciles is acceptable for this slice. It preserves the
   current UX without reintroducing raw-token persistence in the session
   secret.
4. Medium: this slice intentionally does not redesign the public upload URL
   contract. The remaining honest security debt is that the bearer-equivalent
   token still lives in `status.upload` URL query parameters until a separate
   API/UX change replaces that contract.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./cmd/ai-models-artifact-runtime`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 49

Findings:

1. No blocking findings for the landed upload-session state hardening. The
   session `Secret` now keeps a real server-side multipart manifest and
   explicit raw-ingest lifecycle phases instead of relying only on
   `multipartUploadID` plus implicit timestamps.
2. Medium: pulling uploaded-part state from object storage through
   `ListMultipartUploadParts` is the correct bounded fix here. It gives the
   gateway/controller one server-owned resumability view without inventing a
   second custom upload database or an extra API endpoint.
3. Medium: persisting explicit `expired` state both from gateway requests and
   from controller-side session reuse is the right fail-closed choice. Expiry
   is no longer only inferred from `ExpiresAt` at observation time.
4. Medium: this slice honestly stops at raw-ingest stage ownership. The
   remaining lifecycle gap is still `publishing/completed`: as long as the
   controller deletes the upload session right after raw staging handoff, those
   phases cannot be claimed as a real persisted contract.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/adapters/sourcefetch ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession ./internal/dataplane/uploadsession ./internal/adapters/uploadstaging/s3 ./cmd/ai-models-artifact-runtime`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`

## Дополнительный review по slice 50

Findings:

1. No blocking findings for the landed upload-session handoff fix. The
   controller no longer deletes successful runtime state before final status
   projection, so upload sources can now carry session lifecycle truth through
   `publishing/completed` without reopening the upload path.
2. Medium: keeping the source worker alive through the cleanup-handle requeue
   is the correct fix. Deleting it earlier risks either recreating runtime on
   the next reconcile or dropping the successful upload-source handoff before
   `Ready` is persisted.
3. Medium: the new controller-owned upload-session phase sync stays bounded.
   It updates only the existing session `Secret` via the upload-session runtime
   seam and does not introduce a second public status engine or another
   persisted bus.
4. Medium: preserving the multipart manifest while treating
   `uploaded/publishing/completed` as closed mutation phases is the correct
   balance here. Operators keep server-owned inspection state, but late client
   writes are now fail-closed after controller ownership begins.
5. Residual risk: completed upload session secrets still stay with the owner
   object lifecycle. There is still no separate session-retention janitor, so
   this slice closes lifecycle correctness, not retention policy.

Checks:

- `cd images/controller && PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH go test ./internal/application/publishobserve ./internal/controllers/catalogstatus ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/dataplane/uploadsession`
- `PATH=/opt/homebrew/bin:/usr/local/go/bin:$PATH make verify`
- `git diff --check`
## Slice 51 review notes

- The new contract is stricter but more honest: users now configure one shared
  S3-compatible artifact backend and one credential Secret reference instead of
  a fake choice between inline credentials and a separate publication-storage
  branch.
- The phase split is explicit again:
  - MLflow remains a phase-1 backend concern and keeps PostgreSQL in the
    module contract;
  - phase-2 raw ingest and DMCR publication reuse the same S3 backend without
    pretending that PVC-backed DMCR remains a supported product path.
- Observability no longer overpromises PVC capacity monitoring for a storage
  mode that the module no longer supports.

## Slice 52 review notes

- No blocking findings for the new artifact-secret delivery pattern. Moving the
  user-facing source Secret to `d8-system` and copying only the required keys
  into module-owned Secrets in `d8-ai-models` is materially safer than asking
  users to hand-manage credentials inside the service namespace.
- Medium: the hook keeps the blast radius bounded. It does not use global
  `secret-copier` distribution, so the S3 credentials are not sprayed into
  unrelated namespaces.
- Medium: the runtime shell now depends only on fixed module-owned Secret names,
  which removes install-time ambiguity around "where exactly should the Secret
  live" and matches the service-namespace ownership model better.
- Residual risk: the source Secret still has to exist in `d8-system` before a
  successful enable, and the hook currently watches all Secrets in `d8-system`
  because the source names are user-configured. That is acceptable for this
  bounded slice, but it is still broader than a static name selector.

## Slice 53 review notes

- No blocking findings for the DMCR metrics-shell fix. Giving
  `kube-rbac-proxy` the existing projected `dmcr-kube-api-access` volume is
  the correct narrow repair for the live crash.
- Medium: keeping `automountServiceAccountToken: false` at the pod level is
  better than flipping it on globally. The registry container still runs
  without a Kubernetes API token, and only the containers that need cluster API
  access mount the projected token volume.
- Medium: extending the local `kube-rbac-proxy` helper with optional
  `volumeMounts` is acceptable here. The helper already owns the ai-models
  sidecar render contract, and the new field is bounded and opt-in.
- Residual risk: the repeated `TLS handshake error ... EOF` lines from the main
  `dmcr` container are not the same failure. They are secondary probe/client
  noise while the sidecar is unhealthy; this slice targets the actual startup
  blocker in `kube-rbac-proxy`.

## Slice 54 review notes

- No blocking findings against the landed repo fixes. The live e2e run exposed
  real product bugs rather than operator mistakes, and all landed fixes stay
  bounded and aligned with the current phase-2 publication scope.
- High: the CRD CEL fixes are necessary, not optional cleanup. Optional
  immutable fields and upload-only `spec.source` paths must not fail on absent
  keys during normal create/status-update flows.
- High: reducing default publication-worker scratch/resources from `2Ti` to
  `50Gi` is the correct baseline correction. A default that cannot schedule on
  ordinary worker nodes is not an honest bounded runtime shell.
- High: controller-created runtime pods/jobs must inherit the module registry
  pull secret. Without that, tiny-model e2e will just move from `Pending` to
  `ErrImagePull`, which is still a broken default runtime path.
- Medium: the live cluster still cannot finish tiny-model e2e until the new
  module bundle is rolled out. DKP forbids direct mutation of module-owned CRDs
  and deployments, so the remaining blocker is deployment freshness, not an
  unhandled repo defect in this slice.

## Slice 55 review notes

- No blocking findings against the new `HuggingFace` acquisition path.
  Replacing the ad-hoc per-file `HTTP GET` loop with source-native snapshot
  download is the correct architectural cut and matches the earlier canonical
  note.
- High: keeping the rest of the publish pipeline stable is the right boundary.
  Raw staging, checkpoint normalization, format validation, profile resolution
  and `DMCR` publication remain controller-owned contracts rather than leaking
  HF-specific semantics upward.
- Medium: link-first materialization from the snapshot tree into canonical
  `checkpoint/` avoids reintroducing the extra full local copy that earlier
  byte-path slices removed.
- High: the earlier bundled `hf` CLI/binpack implementation was the wrong
  long-term shell for this module. The corrective slice that removes it and
  keeps the same source-native behavior in Go is the right follow-up.
- Residual risk: the Go snapshot adapter is unit-covered, but the final proof
  still requires a fresh module bundle rollout and a real `HuggingFace`
  publish run in the cluster.

## Slice 56 review notes

- No blocking findings against removing the HF CLI/binpack shell. This reduces
  build/runtime drift without changing the public CRD or the controller-owned
  publication contract.
- High: keeping `HuggingFace` acquisition inside `sourcefetch` in Go is the
  right corrective boundary. It simplifies the runtime image and keeps the
  complexity where it belongs: in a normal adapter seam, not in build-time
  Python packaging.
- Medium: the corrected path still preserves the important byte-path behavior:
  exact resolved revision, selected-file download, optional raw-first staging,
  and one normalized local checkpoint tree before profile/publication.
- Residual risk: this slice deliberately optimizes for simpler module shell and
  clearer ownership, not for full HF cache/resume parity. If later required,
  that should be added inside the same Go adapter rather than by reintroducing
  a bundled external CLI.

## Slice 57 review notes

- No blocking findings against trimming the retained HF metadata subset.
  Removing provider-card noise with no live consumer is the right direction
  for both code size and contract discipline.
- High: keeping only `repo/revision/library/pipeline/license/files` inside the
  adapter matches the earlier rule that source-specific metadata must not
  quietly become a second, accidental contract.
- Medium: replacing generic `cardData map[string]any` with a typed minimal
  shape is a real maintenance improvement, not cosmetic cleanup.
- Residual risk: if future provenance/audit work really needs additional HF
  fields, that should be introduced explicitly with a declared consumer, not by
  silently widening the adapter payload again.

## Slice 58 review notes

- No blocking findings against moving `Safetensors` task resolution closer to
  the real checkpoint bytes. This is the right planner-facing correction.
- High: explicit task still wins, so the slice does not weaken the public
  contract or hide user intent behind implicit magic.
- High: using HF `pipeline_tag` only as fallback is materially better than
  treating source metadata as the primary truth for `ResolvedProfile.Task`.
- Residual risk: `GGUF` still needs explicit task. That is acceptable for now
  because there is no equally honest byte-level task inference path there yet.

## Slice 59 review notes

- No blocking findings against removing `SourceRepoID` as a hidden fallback for
  `ResolvedProfile.Family`. Provenance should not silently turn into a profile
  classifier.
- High: switching endpoint compatibility to the final resolved task is a real
  correctness fix. Without it, inferred or fallback task resolution could still
  leave `SupportedEndpointTypes` stale or empty.
- Residual risk: `framework` still allows a source hint path on `Safetensors`.
  That is now the clearest remaining field to either derive from bytes or
  reduce to a format-default value in a later slice.

## Slice 60 review notes

- No blocking findings against removing source-derived `framework` from the
  `Safetensors` publication path. This closes the last obvious case where
  source metadata could still silently shape planner-facing resolved profile.
- High: deleting `framework` from `RemoteResult` and `publishworker` inputs is
  the right boundary. It prevents the same drift from coming back through a new
  remote source adapter as an implicit pass-through field.
- High: keeping public `status.resolved.framework` while normalizing it to the
  format-default value is the right compatibility balance. External readers
  still get a stable label, but it no longer mirrors `HF` provider metadata.
- Residual risk: this narrows the observable meaning of `framework` for
  `Safetensors`. External dashboards that previously differentiated Hugging Face
  `library_name` values from this field will now only see `transformers`, which
  is intentional.

## Slice 61 review notes

- No blocking findings against separating remote acquisition provenance from
  profile hints. This makes the `sourcefetch -> publishworker` boundary more
  explicit without changing any public API.
- High: `HTTP` no longer pretends to be a partially populated Hugging Face
  result. Empty `ProfileHints` on `HTTP` were clearer than carrying unrelated
  zero-value fields in one flat struct; this boundary was later refined in
  `Slice 63` into `Fallbacks` plus `Metadata`.
- Medium: this slice improves maintainability more than behavior, but that is
  exactly the right kind of cleanup here. It reduces the chance of another
  source adapter reintroducing accidental planner/profile inputs by copying a
  flat result shape.
- Residual risk at that point: `ProfileHints` still contained `TaskHint` for
  `HuggingFace` fallback and provenance-derived `license` / `sourceRepoID`.
  That residual was then closed in `Slice 63`.

## Slice 62 review notes

- No blocking findings against moving `license` and `sourceRepoID` out of the
  format resolvers. This restores the right ownership line: resolvers derive
  profile, worker code attaches provenance.
- High: removing provenance-only inputs from both `Safetensors` and `GGUF`
  makes the modelprofile packages more honest and reduces the chance of future
  resolver logic quietly depending on source metadata again.
- High: keeping `TaskHint` as the only remaining `HuggingFace` fallback is the
  right balance. It is still a real consumer-backed hint, unlike `license` and
  `sourceRepoID`, which were only copied through.
- Residual risk: `ResolvedProfile` still mixes planner-facing fields and a
  small provenance subset in one public struct. That is acceptable for now
  because the public API already exposes them and they have declared consumers.

## Slice 63 review notes

- No blocking findings against separating remote fallback and metadata into
  distinct seams. This removes the last mixed bucket in the remote acquisition
  handoff without changing behavior or public API.
- High: `TaskHint` is now explicitly the only source-side fallback contract.
  That is the right boundary and makes future source adapter work less likely
  to smuggle extra semantics through a generic `hints` struct.
- Medium: the extra `Metadata` seam is justified here because the previous
  structure was semantically wrong, not merely imperfectly named.
- Residual risk: the public `ResolvedProfile` still keeps a small metadata
  subset (`license`, `sourceRepoID`). The internal handoff is now clean; the
  remaining question is public API philosophy, not an implementation drift in
  the runtime path.

## Slice 64 review notes

- No blocking findings against removing `license` and `sourceRepoID` from the
  public resolved status. At this point they were pure provenance fields with
  no live planner, policy, metrics, or condition consumer inside the repo.
- High: keeping them in internal runtime provenance while removing them from
  public `status.resolved` is the right boundary. It avoids leaking source
  metadata into the scheduler-facing API without breaking the internal
  publication flow.
- Medium: this is an intentional public compatibility break for status readers.
  That is acceptable in the current alpha stage, but it is important that the
  bundle now states it explicitly instead of letting it happen implicitly.
- Residual risk: external scripts or dashboards outside this repo that read
  `.status.resolved.license` or `.status.resolved.sourceRepoID` will stop
  seeing those fields after the next status rewrite in-cluster.

## Slice 65 review notes

- No blocking findings against removing generic `HTTP` as a source kind. The
  remaining supported remote path, `HuggingFace`, has a clear platform value
  and a much better-defined security and metadata model than arbitrary remote
  HTTP fetch.
- High: deleting `source.caBundle`, HTTP auth projection, HTTP probe code and
  HTTP worker flags in one slice is the right boundary. A half-removed source
  path would leave dead config surface and hidden maintenance debt behind.
- High: the shipped backend image must not quietly keep a second HTTP import
  CLI surface after the controller/API cut. Removing the packaged
  `ai-models-backend-source-import` alias and narrowing
  `ai-models-backend-hf-import` back to HF-only semantics is part of the same
  correctness boundary, not an optional cleanup.
- High: keeping `spec.source.url` while narrowing it to Hugging Face URLs is
  the right API shape. User intent stays simple, while the live support matrix
  becomes explicit and fail-closed.
- Medium: at this stage the live remote matrix is intentionally narrower than
  the compatibility input shape. That is acceptable only because unsupported
  remote URLs deterministically converge to `Failed/UnsupportedSource`.

## Slice 66 review notes

- No blocking findings against handling persisted legacy non-HF `source.url`
  objects at the `catalogstatus` owner boundary. This is the right place to
  converge old immutable specs to a terminal public state without reopening
  deleted runtime branches.
- High: using `Failed` with reason `UnsupportedSource` is materially better
  than surfacing a permanent reconcile error or reintroducing a fake `HTTP`
  `resolvedType`.
- High: omitting `status.source.resolvedType` for this terminal migration path
  is the correct API behavior. Inventing `HTTP` or `Unknown` would create a
  dead public contract surface after the source-kind cut.
- Medium: the slice intentionally fixed persisted objects only; this remained a
  temporary migration gap until the next compatibility cut.

## Slice 68 review notes

- No blocking findings against reintroducing `source.caBundle` and generic
  `http(s)` URL schema only as deprecated compatibility input. The live remote
  matrix is still enforced by controller resolution, not by the CRD pattern.
- High: this is the right way to remove manual manifest migration without
  reviving the deleted generic `HTTP` runtime path.
- High: the compatibility bridge stays one-way and explicit:
  old manifests apply, unsupported remote URLs become
  `Failed/UnsupportedSource`, and no fake `HTTP` source kind returns into the
  public status contract.

## Slice 67 review notes

- No blocking findings against treating this as a build-shell correctness
  slice, not a feature slice. The failure was in module packaging layout, so
  fixing it before more controller work is the right priority.
- High: the missing YAML document separator in `images/controller/werf.inc.yaml`
  was the direct CI/root build breaker and had to be fixed explicitly.
- High: reverting `beforeInstall` back to helper-compatible YAML list items is
  correct for this repo. `.werf/stages/helpers.yaml` emits list items, so a
  block-script wrapper would keep breaking `werf` shell generation.
- Medium: local `werf build --dev --env dev --platform=linux/amd64 controller
  controller-runtime` is now the relevant proof, because it exercises the same
  controller image path that the CI failure stopped before.

## Slice 69 review notes

- No blocking findings against disabling default trace export in `dmcr`. The
  live registry pod was healthy; the repeated `localhost:4318` errors were
  pure upstream auto-telemetry noise because this module does not run an OTLP
  collector beside `dmcr`.
- High: setting `OTEL_TRACES_EXPORTER=none` in the module-owned deployment is
  the correct fix boundary. It is explicit, fail-closed, and does not pretend
  that a local collector exists.
- High: leaving metrics intact while disabling traces is the right platform
  default. `dmcr` observability in this module is currently Prometheus-first,
  not tracing-first.
- Medium: if the module later gets a real tracing integration, this default
  should be revisited deliberately instead of being inferred from generic
  upstream defaults.

## Slice 70 review notes

- No blocking findings against treating the `dmcr` auth mismatch as a module
  template bug, not a runtime/user misconfiguration. Live `HF` smoke reached
  `dmcr`, and the failure reduced to `401 unauthorized` with reproducible
  credential drift between `ai-models-dmcr-auth-write/read` and
  `ai-models-dmcr-auth`.
- High: persisting `write.password` / `read.password` in the server auth secret
  is the right repair boundary. Without persisted plaintext source-of-truth,
  Helm keeps preserving mismatched `htpasswd` entries forever.
- High: self-healing `htpasswd` regeneration only when the stored plaintext no
  longer matches the desired password is the right no-surprises behavior. It
  preserves stable credentials after the first repaired rollout and still fixes
  existing drift automatically.
- Medium: live patching remained intentionally impossible because DKP forbids
  updating `heritage: deckhouse` objects by hand. That is acceptable here: the
  template fix is correct, and rollout is the supported repair path.

## Slice 71 review notes

- No blocking findings against removing `kit inspect --remote` from the
  controller-owned publication success path. Live cluster evidence showed that
  `pack` and `push` already succeeded and the published manifest was present in
  `DMCR`; only the CLI inspection step was failing.
- High: direct OCI registry inspection is the right boundary here. The module
  already owns the target registry and already has the exact write credentials
  and CA material, so digest/manifest lookup should not depend on opaque CLI
  stdout parsing.
- High: keeping `KitOps` only for `pack/push/remove` makes the adapter
  narrower and easier to reason about. This reduces external-tool coupling
  instead of deepening it.
- Medium: live validation still needs one more module rollout because the
  current cluster was tested against the bundle that fixed `DMCR` auth but did
  not yet include this new post-push inspect fix.

## Slice 72 review notes

- No blocking findings against hardening the registry-inspect path with real
  `ModelPack` semantics. The previous transport fix was necessary but not
  sufficient: digest existence alone would have weakened the role of `KitOps`
  to “something that pushed some OCI object”.
- High: validating `artifactType`, config descriptor media type, weight layer
  media types/annotations, and config blob `modelfs` is the right contract
  boundary. These belong to the published `ModelPack`, not to `KitOps`
  implementation trivia.
- High: not requiring `ml.kitops.modelkit.kitfile` is also correct. That
  annotation is `KitOps` provenance, not the durable internal artifact
  contract, so keeping it optional preserves the replaceable packer seam.
- Medium: the next live cluster check still requires another rollout, but the
  code now matches the stronger interpretation of `ModelPack` semantics rather
  than a transport-only approximation.
