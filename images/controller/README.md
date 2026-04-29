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
- `cmd/ai-models-node-cache-runtime/*` for the dedicated long-running
  node-cache CSI/runtime entrypoint; the shared publication/materialize runtime
  image no longer carries the node-cache command path used by managed runtime
  Pods;
- `internal/publicationartifact` for controller-owned publication runtime
  result payloads and OCI destination-reference policy; this is no longer the
  old misleading `artifactbackend` seam and no longer keeps a dead request
  contract;
- `internal/ports/modelpack` for replaceable `ModelPack`
  publication/removal/materialization contract;
- `internal/adapters/modelpack/oci` for the ai-models-owned native OCI
  `ModelPack` implementation: controller-side publish/remove over `DMCR`
  direct-upload v2 with late digest completion into backing storage, one-pass
  raw publish for range-capable heavy layers, bounded companion-bundle plus
  raw-layer publish for streamable multi-file object sources, raw publish for
  single-file inputs, archive-source publish for streamable archives, shared
  published-artifact inspection, semantic validation, and standalone runtime
  materialization into a local path from immutable `DMCR` artifacts; when the
  caller supplies direct-upload state, the adapter can continue interrupted
  raw-layer upload from persisted session state while the helper session is
  still alive, instead of depending only on one live worker process;
- `internal/nodecache` for the module-owned SharedDirect node-local cache
  runtime plane: digest-addressed shared store layout, ready marker parsing,
  stable workload-model-link helper contract, single-writer coordination,
  access timestamp handling, desired-artifact prefetch into the shared store
  with per-digest retry/backoff, protected-digest-aware bounded cache
  scan/eviction planning, and runtime maintenance loop;
- `internal/dataplane/nodecachecsi` for the kubelet-facing CSI node service
  used by the node-cache runtime Pod: it exposes Identity/Node RPCs, validates
  immutable artifact attributes, authorizes the requesting Pod through
  `podInfoOnMount`, returns transient `Unavailable` while a digest is still
  being prefetched, and read-only bind-mounts the ready digest store into the
  kubelet target path;
- `internal/adapters/k8s/modeldelivery` for reusable consumer-side
  `PodTemplateSpec` mutation over the workload-facing node-cache
  `SharedDirect` inline CSI contract. It does not render workload-namespace
  materializer init containers, projected registry credentials, explicit cache
  PVC bridges, or emptyDir/ephemeral fallbacks; unsupported storage topology
  fails closed. The stable workload-facing runtime env contract keeps
  legacy primary-model variables (`AI_MODELS_MODEL_PATH` /
  `AI_MODELS_MODEL_DIGEST` / `AI_MODELS_MODEL_FAMILY`) and adds alias-based
  multi-model delivery through `ai.deckhouse.io/model-refs`,
  `/data/modelcache/models/<alias>`, `AI_MODELS_MODELS_DIR`,
  `AI_MODELS_MODELS` with alias/path/digest/family entries, and per-alias
  `AI_MODELS_MODEL_<ALIAS>_{PATH,DIGEST,FAMILY}` variables without leaking raw
  cache-root layout details into consumers; managed `SharedDirect` requires a
  user-declared node-cache CSI volume and leaves node placement to the workload
  owner or scheduler;
- `internal/adapters/k8s/nodecacheruntime` for stable per-node Pod/PVC shaping
  of the node-cache runtime plane, Deckhouse-style CSI registration sidecar and
  hostPath/socket wiring, plus runtime-side extraction of the published
  artifacts required by live `SharedDirect` managed Pods on the same node;
- `internal/adapters/k8s/nodecachesubstrate` for concrete
  `storage.deckhouse.io/v1alpha1` shaping of managed `LVMVolumeGroupSet`,
  ready managed `LVMVolumeGroup` filtering, and ai-models-owned
  `LocalStorageClass` rendering for the node-local cache runtime;
- `internal/controllers/workloaddelivery` for controller-owned adoption of
  annotated `Deployment` / `StatefulSet` / `DaemonSet` / `CronJob`
  workloads: its bounded admission hook adds an ai-models scheduling gate to
  opt-in `Deployment` / `StatefulSet` / `DaemonSet` pod templates before the
  first rollout, then the controller
  resolves `Model` or `ClusterModel`, reuses the shared `k8s/modeldelivery`
  service, writes resolved artifact plus delivery mode/reason annotations onto
  `PodTemplateSpec`, stamps artifact attributes on the user-declared
  node-cache CSI volume, removes the scheduling gate only after delivery
  preflight passes, leaves workload placement to the workload owner or
  scheduler,
  fail-closes when user-provided `/data/modelcache` storage topology is invalid
  instead of inventing a second storage contract,
  and performs delete-only cleanup for legacy projected registry Secrets without
  creating workload-namespace credentials; watch scope is now narrow to opt-in
  or already-managed workloads plus referenced
  `Model` / `ClusterModel` objects;
- `internal/controllers/nodecachesubstrate` for ai-models-owned managed local
  storage substrate: it keeps one `LVMVolumeGroupSet` plus one
  `LocalStorageClass` for node-local cache preparation and removes the need to
  hand-create local-storage resources for the node-cache runtime plane; the
  controller stays limited to substrate ownership and does not mutate
  workload templates directly;
- `internal/controllers/nodecacheruntime` for controller-owned stable per-node
  runtime Pod/PVC ownership: it reconciles the node-cache agent only for the
  selected nodes and keeps runtime shaping separate from workload mutation and
  storage substrate ownership;
- module render now keeps the first real node-local cache runtime plane as a
  controller-owned stable per-node Pod plus stable PVC over the ai-models-
  managed `LocalStorageClass`; that shared-store volume is sized by
  `nodeCache.size`, reads the published artifacts required by live
  `SharedDirect` managed Pods on the current node, prefetches immutable
  artifacts into the shared node-local digest store through the dedicated
  `nodeCacheRuntime` distroless image, exposes the
  `node-cache.ai-models.deckhouse.io` CSI socket through `node-driver-registrar`
  and bind-mounts ready digest stores into workloads; module render also
  publishes the workload-facing inline CSI driver identity;
- `internal/adapters/sourcefetch` for safe `HuggingFace` remote source fetch
  and archive hardening, with one canonical remote ingest entrypoint
  over shared HTTP transport, source mirror transfer, archive inspection and
  remote summary extraction instead of split orchestration in the worker;
  remote `source.url` bytes use a module-owned policy over direct remote
  object-source publication and controller-owned temporary source objects
  where the adapter needs durable resume semantics; both paths avoid a local
  `workspace/model`, provider-card noise such as downloads/likes/tags is not
  retained in the adapter payload without an explicit consumer, and the remote
  adapter now hands worker code explicit seams for source provenance,
  object-source publish inputs and source metadata rather than a flat local
  snapshot contract;
- `internal/adapters/modelformat` for source-agnostic input-format validation
  rules applied before packaging; inspect/validate/select flow now reuses one
  package-local runner over format-specific rule sets, instead of repeating the
  same traversal in both `Safetensors` and `GGUF`;
- `internal/domain/ingestadmission` for source-agnostic fail-fast invariants
  before heavy remote fetch/materialization starts; sourceworker keeps the
  small owner-binding preflight local to its runtime request path;
- `internal/adapters/modelprofile/safetensors` and
  `internal/adapters/modelprofile/gguf` for ai-inference-oriented metadata
  extraction from normalized model directories, with current live logic based
  on real weight sizes, confidence-tagged source capability/family/quantization
  extraction and endpoint projection only from reliable capability evidence; runtime
  compatibility, launch sizing and topology terms such as `KubeRay`, `MIG` or
  `MPS` are not guessed in catalog metadata; `Safetensors` task resolution now
  prefers explicit metadata and checkpoint config/architecture before weaker
  source hints such as HF `pipeline_tag`; `GGUF` filename-derived family and
  quantization stay internal hints rather than public facts; source provenance
  fields such as `license` and `sourceRepoID` are now attached after resolution
  in `publishworker`, not treated as resolver inputs, and no longer project into
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
  ephemeral-storage requests/limits without a local publication workspace; the
  adapter also keeps one owner-scoped direct-upload state `Secret`, recreates a
  failed worker Pod only when that state still says `Running`, and exposes both
  bounded running progress percent and message derived from the persisted
  checkpoint instead of forcing status code to infer progress from logs or
  human-readable condition text;
- `internal/adapters/k8s/uploadsession` for controller-owned upload session
  supplements:
  one short-lived hash-only session `Secret` per upload plus a separate
  owner-scoped token handoff `Secret` kept as internal recovery state for
  `spec.source.upload`; `status.upload` projects time-bounded secret upload
  URLs, matching the direct upload UX in virtualization, while the shared
  gateway footprint now lives in the controller deployment shell instead of
  per-upload runtime objects; the package also
  implements the shared upload-session runtime port directly,
  consumes the shared `publishop.Request` without local request wrappers or a
  separate request-mapping file, and does not keep a second runtime-proxy
  layer or constructor path over the same concrete adapter; upload-session
  handle shaping now also computes one virtualization-style public progress
  value for local uploads from persisted `expectedSizeBytes` and multipart
  uploaded-part sizes instead of scraping text messages, and the controller now
  refreshes multipart uploaded-part state from staging directly before
  projecting `status.progress`, so public progress does not depend on a
  separate client poll against the gateway;
- `internal/adapters/k8s/uploadsessionstate` for the secret-backed multipart
  session and lifecycle store used by the shared upload gateway; this keeps
  K8s Secret CRUD out of the dataplane use case and out of shared
  `cmdsupport`; the store now owns hash-only upload auth, explicit
  `issued/probing/uploading/uploaded/failed/aborted/expired` phases, and the
  persisted multipart part manifest instead of keeping resumability only in
  `uploadID`;
- `internal/adapters/k8s/directuploadstate` for the secret-backed direct-upload
  checkpoint store reused by `sourceworker` and `publish-worker`:
  owner-generation reset, current-layer session token plus digest-state
  persistence, committed-layer journal, and terminal `completed/failed`
  handoff without leaking K8s Secret CRUD into the OCI adapter or dataplane;
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
  message; canonical `HuggingFace`, staged-object and direct file paths now
  publish streamable inputs as raw layers, archive inputs stay on the
  archive-source path, and the live worker shell no longer keeps a successful
  `checkpointDir` fallback;
- `internal/dataplane/uploadsession` for the controller-owned upload session
  runtime; it now serves the shared `/v1/upload/<sessionID>/<token>` simple
  `PUT` upload path plus multipart session/control API, persists session state in the
  upload Secret, and marks the staged upload result back into that Secret after
  direct or multipart completion, after which controller requeues the object
  into the normal publish-worker path; the runtime now also syncs the
  server-side multipart part manifest from object storage for
  resumability/state inspection and persists explicit `probing` / `expired`
  edges instead of keeping them only implicit in probe data or expiry
  timestamps; once controller takes ownership after upload handoff, the gateway
  now also treats `publishing/completed` as closed session phases and rejects
  any late multipart mutation attempts instead of letting the preserved
  manifest imply a still-open upload; user-facing upload auth is the projected
  secret URL, header/query token variants are not part of the contract, and
  token handoff Secrets remain internal controller/runtime state;
- `internal/dataplane/artifactcleanup` for the controller-owned published
  artifact removal runtime;
- `internal/publishedsnapshot` for immutable published-artifact snapshots used
  as controller handoff between publish, cleanup, and delete steps;
- `internal/ports/publishop` for shared publication operation runtime
  contracts and worker/session handles reused across adapters; the live handoff
  is now one direct `publishop.Request`, not a wrapper over the same request,
  and source-worker handles now also carry one bounded running progress
  percent plus message instead of forcing status code to scrape raw Pod logs or
  parse human-readable condition text;
- `internal/domain/publishstate` for publication lifecycle state, condition and
  observation decisions; upload-driven objects and sourceworker-driven
  publication now project one top-level `status.progress` percent string from
  machine-readable runtime progress instead of forcing operators to infer
  progress from `conditions[*].message`;
- `internal/application/publishobserve` for runtime observation mapping and
  public status mutation planning; source-worker Pod plan shaping is adapter
  code in `internal/adapters/k8s/sourceworker`, and runtime-mode selection is
  controller-local orchestration in `internal/controllers/catalogstatus`;
- `internal/application/publishaudit` for append-only internal audit/event
  planning: one-time lifecycle edge detection and message shaping for upload
  session issue, source fetch start, direct-or-mirrored `HuggingFace`
  publication, and final publication outcome without introducing a second
  lifecycle engine;
- `internal/application/deletion` for delete-time finalizer policy and
  package-local step decisions over cleanup operation progress and registry
  garbage-collection progress instead of hand-assembling the same
  `FinalizeDeleteDecision` payload shape in multiple branches;
- `internal/monitoring/catalogmetrics` for module-owned Prometheus collectors
  over public `Model` / `ClusterModel` state: phase one-hot gauges, ready
  booleans, condition status/reason gauges, small info metrics, and artifact
  size from public `spec/status` instead of runtime-local counters or log
  parsing;
- `internal/monitoring/runtimehealth` for module-owned Prometheus collectors
  over the managed node-cache runtime plane: stable per-node agent `Pod`
  phase/readiness, selector-scoped desired-vs-ready summary gauges, shared-
  cache `PVC` bind/requested-size signals, and aggregated top-level workload
  delivery mode/reason counts from ai-models-owned runtime state instead of
  ad-hoc `kubectl` inspection;
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
  controller path for `Model` / `ClusterModel`; it now owns cleanup operation
  execution and DMCR garbage-collection request lifecycle directly because
  there is no second cleanup adapter and per-delete Kubernetes Jobs do not
  match the module runtime pattern;
  cleanup state and GC request metadata now also reuse one package-local
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
  persisting planned status plus internal cleanup state without an intermediate
  persisted bus; successful runtime cleanup-handle handoff now also keeps the
  runtime object alive until the post-status reconcile that projects final
  state, and upload-source reconciles sync the session lifecycle through
  `publishing/completed/failed` without inventing a second persisted state
  machine; persisted pre-cut non-HF `source.url` objects still terminate as
  `Failed` with `UnsupportedSource` instead of looping forever on reconcile
  errors;
- `internal/controllers/workloaddelivery` for direct workload mutation over
  top-level annotations `ai.deckhouse.io/model`,
  `ai.deckhouse.io/clustermodel`, and alias-based
  `ai.deckhouse.io/model-refs`; this controller intentionally stays
  on mutable workload templates (`Deployment`, `StatefulSet`, `DaemonSet`,
  `CronJob`), uses admission only for the scheduling gate, and does not pretend
  that direct `Job` mutation is safe after creation;
- `internal/bootstrap` for manager/bootstrap wiring.

Naming rule:
- do not keep four different packages named `publication` across
  `application/`, `domain/`, `ports/` and `internal/`; role-based names such
  as `publishobserve`, `publishstate`, `publishop`, `publishedsnapshot`, and
  `publicationartifact` are required so the tree stays explicit and closer to
  virtualization-style ownership.

Current controller scope excludes:
- publication paths beyond the current live input matrix:
  - `HuggingFace URL -> supported Hugging Face checkpoint layouts`
  - `Upload -> Safetensors/Diffusers archive or GGUF file/archive`
  into internal `ModelPack/OCI` served by the module-local DMCR-style backend
  through the current Go dataplane and
  implementation adapter;
- richer input formats beyond the current fail-closed `Safetensors`,
  `Diffusers` and `GGUF` rules shared across `HuggingFace` and `Upload`
  sources;
- richer source auth flows beyond the current minimal projection contract:
  `HuggingFace` supports a projected token secret, but broader source
  integrations and richer auth/session handoff stay out of scope;
- live runtime integration with `ai-inference`; reusable SharedDirect
  consumer-side wiring now exists, but concrete `ai-inference` integration
  remains a separate adapter-specific step;
- richer publication hardening beyond the current implementation adapter:
  implementation switching and stronger trust/promotion semantics.
