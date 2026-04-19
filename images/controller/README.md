# images/controller

`images/controller/` is the canonical root for executable controller code of the
`ai-models` module.

Current structure review and pruning rules live in
[STRUCTURE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/STRUCTURE.ru.md).
Controller-level decision/test evidence lives in
[TEST_EVIDENCE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/TEST_EVIDENCE.ru.md).

Rules:
- phase 2 controller code lives in the module rooted here;
- `go.mod` stays on this directory root;
- `werf.inc.yaml` stays a thin controller image definition and must consume the
  module-local `images/distroless` layer instead of pulling `base/distroless`
  directly into final runtime images;
- `cmd/` stays a thin executable shell;
- domain logic stays under `internal/*` until there is a real external consumer;
- shared controller ports live under `internal/ports/*` and must not stay
  buried inside adapter-local packages;
- concrete reconcilers live under `internal/controllers/*`;
- concrete Pod/Service/Secret builders and CRUD adapters live under
  `internal/adapters/k8s/*`;
- shared helper code may live under `internal/support/*` only when it removes
  real duplication across controller/adapters and does not become a second
  business-logic layer;
- public DKP API types still live in top-level `api/`.

Current phase-2 slice implemented here:
- `werf.inc.yaml` now builds controller runtime images without an external
  publisher binary: the publication path is owned by Go code under
  `internal/adapters/modelpack/oci`, while final images still consume the
  module-local `distroless` stage instead of pulling `base/distroless`
  directly;
- `werf.inc.yaml` now builds final controller images from the module-local
  `distroless` stage so the runtime base follows the same DKP pattern as
  `gpu-control-plane` and `virtualization`;
- `cmd/ai-models-controller/*` for thin manager-only shell;
- `cmd/ai-models-artifact-runtime/*` for thin one-shot phase-2 runtime shell:
  `publish-worker`, `upload-session`, `artifact-cleanup`, and
  `materialize-artifact`; the shell now also owns one shared
  component-aware logger setup for all runtime commands instead of letting each
  subcommand fall back to ad-hoc process logging; controller-owned Go runtime
  logs are now JSON-by-default with normalized `level` / `ts` / `msg`
  envelope plus explicit `LOG_FORMAT` / `LOG_LEVEL` wiring for live deployment
  surfaces; long-running publish/materialize flows now emit stable step-boundary
  `info` progress events and optional `debug` detail for source selection,
  mirror/download, pack/push, remote inspect and shared-cache coordination;
- `internal/publicationartifact` for controller-owned publication runtime
  result payloads and OCI destination-reference policy; this is no longer the
  old misleading `artifactbackend` seam and no longer keeps a dead request
  contract;
- `internal/ports/modelpack` for replaceable `ModelPack`
  publication/removal/materialization contract;
- `internal/adapters/modelpack/oci` for the ai-models-owned native OCI
  `ModelPack` implementation: controller-side publish/remove over direct
  heavy-layer upload into `DMCR` backing storage plus shared published-artifact
  inspection, semantic validation, standalone runtime materialization into a
  local path from immutable `DMCR` artifacts, and one canonical internal
  helper-owned transport for heavy layer blobs through the `DMCR` direct-upload
  helper;
- `internal/adapters/k8s/modeldelivery` for reusable consumer-side
  `PodTemplateSpec` mutation over `materialize-artifact`, fixed
  `/data/modelcache` cache-root contract over user-provided storage,
  topology-aware per-pod versus direct shared PVC handling with RWX
  single-writer cache-root coordination, and cross-namespace read-only DMCR
  auth/CA projection into the runtime namespace without runtime env patching;
- `internal/controllers/workloaddelivery` for controller-owned adoption of
  annotated `Deployment` / `StatefulSet` / `DaemonSet` / `CronJob`
  workloads: it resolves `Model` or `ClusterModel`, reuses the shared
  `k8s/modeldelivery` service, writes digest rollout annotations onto
  `PodTemplateSpec`, fail-closes when user-provided `/data/modelcache`
  storage topology is invalid instead of inventing a second storage contract,
  and stays controller-driven instead of introducing generic mutating or
  validating admission hooks for foreign workload kinds; watch scope is now
  narrow to opt-in or already-managed workloads plus referenced
  `Model` / `ClusterModel` objects;
- `internal/adapters/sourcefetch` for safe `HuggingFace` source
  acquisition and archive hardening, with one canonical remote ingest entrypoint
  over shared HTTP transport, source mirror transfer, archive inspection and
  remote summary extraction instead of split orchestration in the worker;
  remote `source.url` bytes now support two explicit runtime modes:
  direct remote object-source publication and controller-owned temporary source
  mirror under the shared `raw/` object subtree; both paths avoid a local
  `workspace/model`, provider-card noise such as downloads/likes/tags is not
  retained in the adapter payload without an explicit consumer, and the remote
  adapter now hands worker code explicit seams for source provenance,
  object-source publish inputs and source metadata rather than a flat local
  snapshot contract;
- `internal/adapters/modelformat` for source-agnostic input-format validation
  rules applied before packaging; inspect/validate/select flow now reuses one
  package-local runner over format-specific rule sets, instead of repeating the
  same traversal in both `Safetensors` and `GGUF`;
- `internal/domain/ingestadmission` and
  `internal/application/sourceadmission` for the bounded fail-fast admission
  stage before heavy remote fetch/materialization starts;
- `internal/adapters/modelprofile/safetensors` and
  `internal/adapters/modelprofile/gguf` for ai-inference-oriented metadata
  extraction from normalized model directories, with current live logic based
  on real weight sizes, task-to-endpoint mapping, quantization/precision
  inference, and minimum-launch estimation; endpoint metadata is now projected
  as platform semantic endpoint types (`Chat`, `TextGeneration`, and peers)
  rather than transport names, and runtime compatibility is no longer guessed
  from publication hints or topology terms such as `KubeRay`; `Safetensors`
  task resolution now prefers explicit user intent, then checkpoint
  config/architecture, and only then source hints such as HF `pipeline_tag`;
  `family` no longer falls back to source repo IDs and stays byte-derived only;
  `framework` on the
  `Safetensors` path is now a normalized format-default label
  (`transformers`), not source-derived metadata; source provenance fields such
  as `license` and `sourceRepoID` are now attached after resolution in
  `publishworker`, not treated as resolver inputs, and no longer project into
  public `status.resolved`;
- `internal/adapters/k8s/sourceworker` for controller-owned worker Pods that turn
  accepted remote URLs into internal published artifacts, while reserving
  `Upload` for a dedicated session workflow;
  the package also implements the shared source-worker runtime port directly,
  consumes the shared `publishop.Request` without adapter-local request
  mirrors, and does not keep a second runtime-proxy layer or
  constructor path over the same concrete adapter; it now drives the concrete
  Pod through one direct `CreateOrGet` cycle instead of a separate replay read
  path before the same create/reuse flow, and projected auth supplements now go
  through one direct `CreateOrUpdate` path instead of adapter-local CRUD;
  active publish-worker concurrency is now capped explicitly before Pod
  creation, and the worker Pod now carries only explicit CPU, memory and
  ephemeral-storage requests/limits without a local publication workspace;
- `internal/adapters/k8s/uploadsession` for controller-owned upload session
  supplements:
  one short-lived session `Secret` per upload plus user-facing upload URL
  projection for `spec.source.upload`, while the shared gateway footprint now
  lives in the controller deployment shell instead of per-upload runtime
  objects; the package also implements the shared upload-session runtime port
  directly, consumes the shared `publishop.Request` without local request
  wrappers or a separate request-mapping file, and does not keep a
  second runtime-proxy layer or constructor path over the same concrete
  adapter;
- `internal/adapters/k8s/uploadsessionstate` for the secret-backed multipart
  session and lifecycle store used by the shared upload gateway; this keeps
  K8s Secret CRUD out of the dataplane use case and out of shared
  `cmdsupport`; the store now owns hash-only upload auth, explicit
  `issued/probing/uploading/uploaded/failed/aborted/expired` phases, and the
  persisted multipart part manifest instead of keeping resumability only in
  `uploadID`;
- `internal/adapters/k8s/ociregistry` for shared OCI registry auth/CA env,
  volume rendering, and controller-owned write-auth / CA projection lifecycle
  used by worker/session/cleanup paths against the module-local DMCR-style
  publication backend;
- `internal/adapters/k8s/ownedresource` for the single canonical
  owned-resource lifecycle shell reused by controlled worker/session
  supplements: create/reuse plus ignore-not-found delete;
- `internal/ports/auditsink` plus `internal/adapters/k8s/auditevent` for the
  append-only internal audit sink over `Kubernetes Events`, without creating a
  second lifecycle truth; the concrete sink now mirrors the same lifecycle
  edges into structured controller logs so operator audit trail no longer
  lives only in Event resources;
- `internal/adapters/k8s/sourceworker` for the concrete publication Pod shell:
  publish-worker arg/env shaping, projected HuggingFace auth, OCI registry CA
  mounts, object-storage CA mounts, and concurrency-safe source-worker pod
  lifecycle without a local publication workspace contract;
- `internal/dataplane/publishworker` for the controller-owned publication
  runtime that fetches sources, computes resolved metadata, publishes a
  `ModelPack`, and writes the final result into the worker Pod termination
  message; canonical `HuggingFace`, direct-upload and streamable archive paths
  now publish via object-source or archive-source streaming semantics and no
  longer keep a successful `checkpointDir` fallback in the live worker shell;
- `internal/dataplane/uploadsession` for the controller-owned upload session
  runtime; it now serves the shared `/v1/upload/<sessionID>` multipart
  session/control API, persists session state in the upload Secret, and marks
  the staged upload result back into that Secret after multipart completion,
  after which controller requeues the object into the normal publish-worker
  path; the runtime now also syncs the server-side multipart part manifest
  from object storage for resumability/state inspection and persists explicit
  `probing` / `expired` edges instead of keeping them only implicit in probe
  data or expiry timestamps; once controller takes ownership after upload
  handoff, the gateway now also treats `publishing/completed` as closed
  session phases and rejects any late multipart mutation attempts instead of
  letting the preserved manifest imply a still-open upload;
- `internal/dataplane/artifactcleanup` for the controller-owned published
  artifact removal runtime;
- `internal/publishedsnapshot` for immutable published-artifact snapshots used
  as controller handoff between publish, cleanup, and delete steps;
- `internal/ports/publishop` for shared publication operation runtime
  contracts and worker/session handles reused across adapters; the live handoff
  is now one direct `publishop.Request`, not a wrapper over the same request;
- `internal/domain/publishstate` for publication lifecycle state, condition and
  observation decisions;
- `internal/application/publishplan` for source-worker and upload-session
  planning use cases;
- `internal/application/publishaudit` for append-only internal audit/event
  planning: one-time lifecycle edge detection and message shaping for upload
  session issue, source fetch start, direct-or-mirrored `HuggingFace`
  publication, and final publication outcome without introducing a second
  lifecycle engine;
- `internal/application/deletion` for delete-time finalizer policy and
  package-local step decisions over cleanup-job progress and registry
  garbage-collection progress instead of hand-assembling the same
  `FinalizeDeleteDecision` payload shape in multiple branches;
- `internal/monitoring/catalogmetrics` for module-owned Prometheus collectors
  over public `Model` / `ClusterModel` state: phase one-hot gauges,
  ready/validated booleans, small info metrics, and artifact size from
  public `spec/status` instead of runtime-local counters or log parsing;
- `internal/support/cleanuphandle` for controller-owned backend-specific delete
  state
  that must not leak into public status;
- `internal/support/modelobject` for shared `Model` / `ClusterModel`
  publication-request, kind and status helpers;
- `internal/support/resourcenames` for the single canonical owner-based
  resource naming policy plus owner-label rendering/extraction and label
  normalization across K8s adapters;
- `internal/support/testkit` for shared controller test scheme/object/fake-client
  fixtures; package-local `test_helpers_test.go` should only keep adapter-local
  option/resource builders, not duplicate the same scheme and model fixtures in
  every controller package;
- `internal/controllers/catalogcleanup` for minimal delete-only finalizer
  controller path for `Model` / `ClusterModel`; it now owns cleanup Job
  materialization and DMCR garbage-collection request lifecycle directly
  because there is no second cleanup adapter and the old
  `adapters/k8s/cleanupjob` package was only an unnecessary extra boundary;
  cleanup job and GC request metadata now also reuse one package-local
  owner-label seam over shared `resourcenames` policy instead of duplicating
  raw label maps; delete apply prerequisites are now precomputed once per
  reconcile step, and finalizer release reuses the observed cleanup handle
  instead of reparsing annotations; delete reconcile itself now carries one
  package-local finalize flow object from observation to apply, and
  upload-staging cleanup completion no longer performs irrelevant DMCR GC
  observation;
- `internal/application/publishobserve` for publication reconcile gating,
  runtime port orchestration, worker/session observation mapping, and runtime
  result decoding plus status-mutation planning behind an application seam
  instead of inside reconciler files;
- `internal/controllers/catalogstatus` for thin `Model` / `ClusterModel`
  publication lifecycle ownership: calling application use cases and
  persisting planned status / cleanup-handle mutations without an intermediate
  persisted bus; successful runtime cleanup-handle handoff now also keeps the
  runtime object alive until the post-status reconcile that projects final
  state, and upload-source reconciles sync the session lifecycle through
  `publishing/completed/failed` without inventing a second persisted state
  machine; persisted pre-cut non-HF `source.url` objects still terminate as
  `Failed` with `UnsupportedSource` instead of looping forever on reconcile
  errors;
- `internal/controllers/workloaddelivery` for direct workload mutation over
  top-level annotations `ai.deckhouse.io/model` and
  `ai.deckhouse.io/clustermodel`; this controller intentionally stays
  on mutable workload templates (`Deployment`, `StatefulSet`, `DaemonSet`,
  `CronJob`) and does not pretend that direct `Job` mutation is safe after
  creation;
- `internal/bootstrap` for manager/bootstrap wiring.

Naming rule:
- do not keep four different packages named `publication` across
  `application/`, `domain/`, `ports/` and `internal/`; role-based names such
  as `publishplan`, `publishstate`, `publishop`, `publishedsnapshot`, and
  `publicationartifact` are required so the tree stays explicit and closer to
  virtualization-style ownership.

Current controller scope excludes:
- publication paths beyond the current live input matrix:
  - `HuggingFace URL -> supported Hugging Face checkpoint layouts`
  - `Upload -> Safetensors archive or GGUF file/archive`
  into internal `ModelPack/OCI` served by the module-local DMCR-style backend
  through the current Go dataplane and
  implementation adapter;
- richer input formats beyond the current fail-closed `Safetensors` and `GGUF`
  rules shared across `HuggingFace` and `Upload` sources;
- richer source auth flows beyond the current minimal projection contract:
  `HuggingFace` supports a projected token secret, but broader source
  integrations and richer auth/session handoff stay out of scope;
- live runtime integration with `ai-inference` and concrete init-container
  pod mutation/runtime injection; reusable consumer-side wiring now exists,
  but concrete `ai-inference` integration remains a separate adapter-specific
  step;
- richer publication hardening beyond the current implementation adapter:
  implementation switching and stronger trust/promotion semantics.
