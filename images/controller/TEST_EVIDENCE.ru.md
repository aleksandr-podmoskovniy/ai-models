# Test Evidence

Этот файл — единственный канонический inventory decision-coverage для
`images/controller`.

Мы больше не держим локальные `BRANCH_MATRIX.ru.md` в части пакетов и не
размазываем одинаковый паттерн по дереву. Evidence остаётся одной точкой
правды рядом с controller runtime.

Дополнительная discipline для live tree:

- `_test.go` files подчиняются тому же structural budget, что и production
  code: `< 350` строк на file без allowlist-first мышления;
- tests режутся по decision surface, а не по случайному helper reuse;
- shared helper file допустим только если он реально обслуживает несколько
  маленьких decision-specific test files внутри одного package.

## `internal/domain/publishstate`

- Decision surface:
  - publication phases и terminal semantics
  - upload status equality
  - worker/session observation decisions
  - status/condition projection
  - explicit terminal `UnsupportedSource` projection for persisted
    non-HF remote objects after the HTTP-source removal cut
- Primary evidence:
  - `operation_test.go`
  - `runtime_decisions_test.go`
  - `status_test.go`
  - `status_success_test.go`
- Residual gaps:
  - replay/retry и malformed runtime result остаются на
    `controllers/catalogstatus`, а не в domain

## `internal/application/publishplan`

- Decision surface:
  - execution-mode selection
  - source-worker planning
  - upload-session issuance policy
- Primary evidence:
  - `start_publication_test.go`
  - `plan_source_worker_test.go`
  - `issue_upload_session_test.go`
- Residual gaps:
- source acceptance и reconcile short-circuit rules остаются adapter-local в
  `controllers/catalogstatus`
- concrete replay/IO races остаются в adapter package tests

## `internal/application/publishobserve`

- Decision surface:
  - translation of worker/session runtime handles into domain observations
  - fail-closed handling for malformed terminal payloads
  - upload-session expiry handling before public status projection
  - upload-source reconcile gating when cleanup-handle handoff is already
    persisted before the final ready status pass
  - unsupported non-HF remote-source rejection staying at controller-owner
    boundary instead of leaking deeper into runtime orchestration
- Primary evidence:
  - `observe_runtime_test.go`
  - `observe_source_worker_test.go`
  - `observe_upload_session_test.go`
  - `ensure_runtime_test.go`
  - `reconcile_gate_test.go`
- Residual gaps:
  - final status persistence and reconcile replay still belong to
    `controllers/catalogstatus` tests

## `internal/application/sourceadmission`

- Decision surface:
  - fail-fast preflight boundary for `source.url`
  - owner binding and declared-format allowlist before remote fetch starts
  - `HuggingFace` host allowlist and obvious remote-source rejection before
    byte download starts
- Primary evidence:
  - `preflight_test.go`
- Residual gaps:
  - concrete remote snapshot/download behavior stays covered in
    `adapters/sourcefetch`
  - full remote download and unpack semantics remain adapter/runtime evidence

## `internal/application/publishaudit`

- Decision surface:
  - append-only internal audit/event planning without a second lifecycle truth
  - one-time lifecycle edge detection for upload session issue, remote ingest
    start, raw staging, and final publication outcome
  - audit message shaping from internal raw provenance and final OCI outcome
- Primary evidence:
  - `plan_test.go`
- Residual gaps:
  - operator-facing browsing/aggregation remains a later UX slice
  - async scanner execution still requires a separate runtime split and is not
    claimed by this event-planning seam

## `internal/adapters/k8s/uploadsessionstate`

- Decision surface:
  - secret-backed upload session lifecycle store
  - hash-only upload token persistence
  - explicit `issued/probing/uploading/uploaded/publishing/completed/failed/
    aborted/expired` phase transitions
  - persisted multipart part manifest encoding/decoding
- Primary evidence:
  - `secret_test.go`
- Residual gaps:
  - no background garbage collection for completed session secrets; retention
    still follows owner lifecycle rather than a separate session janitor

## `internal/adapters/k8s/uploadsession`

- Decision surface:
  - controller-owned upload session issuance and replay over Secret-backed
    session state
  - shared-gateway URL projection and owner-bound namespace semantics
  - legacy/stale session secret invalidation before token-hash-based reuse
  - explicit controller phase sync for `publishing/completed/failed`
- Primary evidence:
  - `service_test.go`
  - `service_lifecycle_test.go`
  - `service_phase_sync_test.go`
- Residual gaps:
  - concrete Pod rendering and kube object shaping stay outside this adapter
    and remain covered by `dataplane/uploadsession` plus controller-runtime
    tests

## `internal/dataplane/uploadsession`

- Decision surface:
  - shared upload gateway control API
  - bounded probe/init/parts/complete/abort request validation
  - server-side multipart manifest sync before info/complete
  - fail-closed expiry handling with explicit persisted `expired` state
  - closed-session rejection after controller handoff to
    `uploaded/publishing/completed`
- Primary evidence:
  - `run_test.go`
  - `run_session_api_test.go`
  - `run_multipart_api_test.go`
  - `run_session_expiry_test.go`
- Residual gaps:
  - upload gateway still exposes one tokenized session URL contract; bearer
    removal from the public upload URL is a separate API/UX slice

## `internal/adapters/k8s/auditevent`

- Decision surface:
  - append-only audit sink over `Kubernetes Events`
  - structured log mirroring for the same audit lifecycle edges
  - stable object/reason/event-type attrs without a second lifecycle engine
- Primary evidence:
  - `recorder_test.go`
- Residual gaps:
  - operator-facing audit browsing remains outside this sink
  - long-term aggregation still belongs to monitoring/logging platform layers,
    not to the sink itself

## `internal/cmdsupport`

- Decision surface:
  - one shared logger shell for controller and runtime commands
  - bridge from root `slog` logger into `controller-runtime` and `klog`
  - component-aware process error logging instead of ad-hoc stderr-only errors
- Primary evidence:
  - `common_test.go`
- Residual gaps:
  - command-level lifecycle logging stays covered in runtime/controller package
    tests and live shell checks, not here

## `internal/monitoring/catalogmetrics`

- Decision surface:
  - module-owned product metrics over public `Model` / `ClusterModel` truth
  - one-hot phase projection plus explicit ready/validated gauges
  - fallback to public `spec` when resolved status fields are not populated yet
  - minimal info/artifact-size exposure without leaking hidden runtime handles
- Primary evidence:
  - `collector_test.go`
- Residual gaps:
  - `PrometheusRule` and dashboards remain later observability slices
  - component health alerts still rely on platform kube-state metrics rather
    than custom collector logic

## `internal/publicationartifact`

- Decision surface:
  - publication runtime result payload validation and JSON round-trip
  - controller-owned OCI destination reference policy
- Primary evidence:
  - `contract_test.go`
  - `location_test.go`
- Residual gaps:
  - worker/controller integration around this payload remains covered in
    `publishobserve` and `catalogstatus` tests, not here

## `internal/domain/ingestadmission`

- Decision surface:
  - owner binding invariants for upload and remote admission
  - declared input-format allowlist
  - filename normalization and obvious direct-file rejection policy
  - bounded upload probe classification for archive vs direct `GGUF`
- Primary evidence:
  - `common_test.go`
  - `upload_test.go`
- Residual gaps:
  - deep content validation and malware scanning are intentionally out of scope
    for this fail-fast stage
  - remote transport quirks remain adapter evidence

## `internal/adapters/modelformat`

- Decision surface:
  - source-agnostic input-format validation policy
  - automatic format detection when `spec.inputFormat` is empty
  - file keep / benign-drop / hard-reject policy
  - benign-extra stripping before `ModelPack` packaging
  - required file and required asset enforcement
  - single-file `GGUF` acceptance alongside archive-based inputs
- Primary evidence:
  - `detect_test.go`
  - `validation_test.go`
- Residual gaps:
  - future extra formats beyond `Safetensors` and `GGUF` will need their own
    rule families and tests

## `internal/adapters/modelprofile`

- Decision surface:
  - endpoint type mapping from `task`
  - precision and quantization inference
  - `parameterCount` estimation
  - `minimumLaunch` estimation
  - format-specific profile extraction for `Safetensors` and `GGUF`
- Primary evidence:
  - `common/profile_test.go`
  - `safetensors/profile_test.go`
  - `gguf/profile_test.go`
  - `domain/publishstate/status_test.go`
- Residual gaps:
  - future richer formats will need their own profile adapters

## `internal/adapters/k8s/workloadpod`

- Decision surface:
  - bounded publish work-volume rendering for `EmptyDir` vs PVC
  - explicit runtime resource contract for CPU, memory and
    `ephemeral-storage`
- Primary evidence:
  - `options_test.go`
  - `render_test.go`
- Residual gaps:
  - no scheduler/integration replay here; cluster-level placement remains
    covered by render and controller runtime tests

## `internal/adapters/k8s/sourceworker`

- Decision surface:
  - explicit publish-worker Pod resources and bounded snapshot-dir wiring
  - shared publish concurrency gate before Pod creation
  - controller-owned raw-stage args for remote `source.url` publication
  - projected auth/registry supplement lifecycle around the worker Pod
- Primary evidence:
  - `build_test.go`
  - `service_test.go`
  - `service_roundtrip_test.go`
  - `validation_test.go`
- Residual gaps:
  - exact concurrent create races across independent reconciles remain an
    adapter/runtime integration concern, not a unit-testable pure decision
    table

## `internal/adapters/k8s/modeldelivery`

- Decision surface:
  - concrete consumer-side `PodTemplateSpec` mutation over
    `materialize-artifact`
  - reuse existing workload-mounted `/data/modelcache` storage instead of
    inventing a second delivery volume contract
  - stable local runtime path contract:
    `/data/modelcache/current`
  - topology rules:
    per-pod mounts and StatefulSet claim templates pass, shared direct PVC on
    multi-replica workloads requires RWX and cache-root single writer
  - cross-namespace reuse of projected OCI auth/CA without a second
    delivery-specific secret model
- Primary evidence:
  - `service_test.go`
  - `service_topology_test.go`
  - `render_test.go`
  - `workload_hints_test.go`
  - `materialize_artifact_test.go`
  - `materialize_coordination_test.go`
  - `internal/adapters/k8s/ociregistry/projection_test.go`
  - `internal/adapters/modelpack/oci/materialize_test.go`
- Residual gaps:
  - concrete runtime wiring is landed as a reusable K8s service, but an
    external consumer module still needs its own thin overlay to call it

## `internal/controllers/workloaddelivery`

- Decision surface:
  - top-level workload annotation contract:
    `ai-models.deckhouse.io/model` /
    `ai-models.deckhouse.io/clustermodel`
  - controller-owned mutation only for workloads with mutable
    `PodTemplateSpec`
  - generic workload delivery stays out of admission webhook surface and
    keeps narrow opt-in / managed watch scope plus reverse reconcile from
    referenced `Model` / `ClusterModel`
  - stale delivery cleanup when annotation disappears or referenced model is
    not `Ready`
  - fail-closed shared direct PVC topology without leaked projected OCI auth
- Primary evidence:
  - `internal/controllers/workloaddelivery/annotations_test.go`
  - `internal/controllers/workloaddelivery/options_test.go`
  - `internal/controllers/workloaddelivery/predicate_test.go`
  - `internal/controllers/workloaddelivery/reconciler_test.go`
  - `cmd/ai-models-controller/config_test.go`
  - `internal/bootstrap/bootstrap_test.go`
  - `internal/adapters/k8s/modeldelivery/service_topology_test.go`
- Residual gaps:
  - live cluster proof for shared `RWX` writer/waiter coordination through the
    workload delivery controller is still pending rollout

## `internal/adapters/sourcefetch`

- Decision surface:
  - remote `HuggingFace` source mirror into controller-owned object storage
    before local checkpoint preparation
  - persisted mirror manifest/state handoff around `HuggingFace` snapshot
    acquisition
  - resumable mirror-byte transport via object-storage multipart upload plus
    HTTP `Range` resume from the already confirmed byte offset
  - `HuggingFace` source-native snapshot acquisition through a package-local
    Go downloader instead of the removed ad-hoc per-file download loop
  - direct single-file checkpoint materialization via link-first staging when
    source and checkpoint share the same filesystem
  - safe archive unpacking and direct `GGUF` normalization
- Primary evidence:
  - `archive_test.go`
  - `rawstage_test.go`
  - `huggingface_test.go`
  - `hfsnapshot_test.go`
  - `huggingface_fetch_test.go`
- Residual gaps:
  - dedicated live-cluster replay for `HuggingFace` snapshot acquisition is
    still pending a fresh module rollout; current evidence is unit-level plus
    the shared publishworker path
  - file-level and intra-file parallelism are still future slices; current
    landed transport is resumable but intentionally sequential

## `internal/ports/sourcemirror`

- Decision surface:
  - durable source snapshot locator contract
  - immutable mirror manifest validation
  - persisted mirror phase/file state validation
  - stable object-storage prefix derivation for mirror ownership and cleanup
- Primary evidence:
  - `contract_test.go`
- Residual gaps:
  - no provider-specific policy lives here; `HuggingFace` and future `Ollama`
    mapping remain adapter-owned

## `internal/adapters/sourcemirror/objectstore`

- Decision surface:
  - object-storage persistence for mirror manifest/state JSON
  - `not found` mapping into shared mirror contract
  - whole-snapshot prefix deletion contract for later owner cleanup
- Primary evidence:
  - `adapter_test.go`
- Residual gaps:
  - multipart byte transport intentionally remains outside this adapter and
    stays owned by `sourcefetch`

## `internal/dataplane/publishworker`

- Decision surface:
  - bounded workspace allocation under controller-provided snapshot root
  - cleanup semantics for per-run work directories
  - source-mirror raw provenance over durable mirrored files instead of
    transient raw-stage copies for remote sources
  - source-mirror prefix handoff into backend cleanup ownership
  - direct upload / direct `GGUF` acceptance and archive validation on the
    publish path
- Primary evidence:
  - `run_test.go`
- Residual gaps:
  - remote raw-first staging still pays an extra object-storage hop inside the
    same bounded publish worker until a future native encoder/runtime cut

## `internal/dataplane/artifactcleanup`

- Decision surface:
  - backend artifact delete versus upload-staging delete dispatch
  - repository metadata prefix pruning after backend remove
  - source-mirror prefix pruning as part of the same backend-owned cleanup path
- Primary evidence:
  - `run_test.go`
  - `backend_prefix_test.go`
- Residual gaps:
  - registry-side asynchronous blob GC remains outside this runtime and stays
    owned by the existing `DMCR` GC path

## `internal/support/cleanuphandle`

- Decision surface:
  - cleanup-handle serialization contract across controller and cleanup runtime
  - backend artifact ownership fields, including repository metadata and source
    mirror prefixes
  - upload staging handle validation
- Primary evidence:
  - `handle_test.go`
- Residual gaps:
  - handle schema versioning is still unnecessary while the runtime remains
    alpha and module-owned

## `internal/application/deletion`

- Decision surface:
  - cleanup finalizer policy
  - delete-time cleanup decision table
  - package-local step decisions for cleanup-job progress and DMCR GC progress
- Primary evidence:
  - `ensure_cleanup_finalizer_test.go`
  - `finalize_delete_test.go`
  - `finalize_delete_progress_test.go`
- Residual gaps:
  - adapter-level create-race/status replay остаются в
    `controllers/catalogcleanup` tests

## `internal/controllers/catalogstatus`

- Decision surface:
  - owner-level reconcile gating and runtime selection for `Model` /
    `ClusterModel`
  - status projection replay across upload handoff and source-worker progress
  - upload-session phase sync on `publishing/completed/failed`
  - explicit `UnsupportedSource` terminal projection without starting runtime
  - publication audit emission on upload issue and terminal success
- Primary evidence:
  - `reconciler_test.go`
  - `reconciler_upload_test.go`
  - `test_helpers_test.go`
- Residual gaps:
  - envtest-level controller-runtime race replay still remains outside the
    current fake-client evidence shape

## `internal/controllers/catalogcleanup`

- Decision surface:
  - delete/finalizer owner flow over cleanup job, DMCR GC request and finalizer
    release
  - shared owner metadata projection for cleanup job and GC request objects
  - request-secret reuse semantics: refresh owner labels, clear stale `done`
    mark and re-arm the GC switch
  - package-local finalize flow over observe/decide/apply on the delete path
- Primary evidence:
  - `apply_test.go`
  - `observe_test.go`
  - `reconciler_test.go`
  - `job_test.go`
  - `gc_request_test.go`
- Residual gaps:
  - no standalone envtest replay here; controller-runtime fake client coverage
    remains the primary evidence for this delete-only owner

## Live `HF` smoke against cluster

- Scenario:
  - `Model` in namespace `ai-models-smoke`
  - `source.url=https://huggingface.co/hf-internal-testing/tiny-random-PhiForCausalLM`
  - `inputFormat=Safetensors`
- Result:
  - `RemoteIngestStarted` reached the publish worker successfully;
  - publish then failed on `dmcr /v2/` auth with
    `401 unauthorized: authentication required`.
- Root-cause proof:
  - direct curl with current `ai-models-dmcr-auth-write` credentials also
    returned `401`;
  - direct bcrypt comparison proved
    `ai-models-dmcr-auth-write.password` did not match
    `ai-models-dmcr-auth.write.htpasswd`.
- Outcome:
  - this smoke validated the need for Slice 70 template reconciliation of
    `dmcr` server and client credentials;
  - live repair requires module rollout because DKP forbids manual mutation of
    `heritage: deckhouse` secrets.

## Live `HF` smoke after `DMCR` auth rollout

- Scenario:
  - `Model` in namespace `ai-models-smoke`
  - `source.url=https://huggingface.co/hf-internal-testing/tiny-random-PhiForCausalLM`
  - `inputFormat=Safetensors`
- Result:
  - `DMCR` auth now succeeds:
    - projected write/read passwords match server-side `htpasswd`;
    - direct `/v2/` call with live write credentials returns `200`;
  - publish still fails, but now later in the path with
    `kitops inspect returned an empty payload`.
- Root-cause proof:
  - the published manifest already exists in `DMCR` under the expected
    controller-owned repository path;
  - `HEAD /v2/<repo>/manifests/published` returns `200` and
    `Docker-Content-Digest`;
  - therefore `pack` and `push` succeeded, and the failure is isolated to the
    post-push `KitOps inspect --remote` step.
- Outcome:
  - this smoke validated Slice 71: digest/manifest inspection must use the OCI
    registry API directly instead of parsing `KitOps` inspect output.

## Live `ModelPack` manifest/config inspection

- Scenario:
  - direct inspection of the failed-but-pushed artifact published to `DMCR`
    for `tiny-random-phi-debug2`;
- Result:
  - the top-level manifest already contained:
    - `artifactType=application/vnd.cncf.model.manifest.v1+json`
    - `config.mediaType=application/vnd.cncf.model.config.v1+json`
    - weight layer media type
    - `org.cncf.model.filepath` layer annotation;
  - the referenced config blob contained:
    - `descriptor`
    - `modelfs.type=layers`
    - non-empty `modelfs.diffIds`.
- Outcome:
  - this inspection proved that `ModelPack` semantics live in the published
    manifest+config pair, not only in the digest or in `KitOps` CLI output;
  - Slice 72 therefore hardened post-push success criteria to validate those
    `ModelPack` fields directly from `DMCR`.

## Live second `HF` smoke against a non-phi repo

- Scenario:
  - `Model` in namespace `ai-models-smoke`
  - `source.url=https://huggingface.co/hf-internal-testing/tiny-random-LlamaForCausalLM`
  - `inputFormat=Safetensors`
  - `runtimeHints.task=text-generation`
- Result:
  - controller accepted the spec and started remote ingest;
  - publish then failed fast with:
    `input format "Safetensors" rejects source file "onnx/model.onnx"`.
- Root-cause proof:
  - live HF API for the repo returned a valid safetensors checkpoint set:
    `config.json`, `model.safetensors`, tokenizer files, plus an `onnx/`
    export subtree;
  - publish worker logs showed the failure came from local source-file
    selection/validation rather than from `HF`, network, or `DMCR`;
  - therefore the bug was a false rejection of benign alternative export
    artifacts in the `Safetensors` ingest path.
- Outcome:
  - this smoke validated the need for the current corrective slice in
    `internal/adapters/modelformat`;
  - benign alternative export artifacts such as `onnx/` must be ignored for
    canonical `Safetensors` publication;
  - helper scripts and eval/docs payloads should be stripped rather than fail a
    valid repo;
  - hard reject should stay reserved for compiled/native or archive payloads.

## Live `Gemma 4` smoke on the current public `HuggingFace` contract

- Scenario:
  - `Model` in namespace `ai-models-smoke`
  - `source.url=https://huggingface.co/google/gemma-4-E2B-it`
  - `inputFormat=Safetensors`
  - `runtimeHints.task=text-generation`
  - no explicit `revision` or `authSecretRef`
- Result:
  - current public manifest shape worked unchanged on the live cluster;
  - controller accepted the spec, resolved the source as `HuggingFace`,
    completed publication, inspection and validation, and finished with:
    - `phase=Ready`
    - `ArtifactPublished=True`
    - `MetadataReady=True`
    - `Validated=True`
    - `Ready=True`;
  - published artifact:
    - `digest=sha256:d3a98df3d0fff2a2249cf61339492f260122b703621d667259e832681f008d55`
    - `mediaType=application/vnd.cncf.model.manifest.v1+json`;
  - resolved source metadata:
    - `resolvedType=HuggingFace`
    - `resolvedRevision=b4a601102c3d45e2b7b50e2057a6d5ec8ed4adcf`;
  - resolved technical profile included:
    - `family=gemma4`
    - `framework=transformers`
    - `architecture=Gemma4ForConditionalGeneration`
    - `format=Safetensors`
    - `parameterCount=1579352064`
    - `contextWindowTokens=131072`
    - `compatibleRuntimes=[KServe,KubeRay]`
    - `minimumLaunch.acceleratorMemoryGiB=12`.
- Root-cause proof:
  - the same live cluster had already proven that current `source.url` parsing,
    publish worker wiring and `DMCR` publication path are functional;
  - this run additionally proved that the plain user-facing
    `https://huggingface.co/<owner>/<repo>` contract is sufficient for an
    official small `Gemma 4` checkpoint and that the controller resolves the
    exact commit SHA into status without requiring the user to embed it in the
    manifest.
- Outcome:
  - users can already publish `Gemma 4 E2B IT` with the current runtime
    contract;
  - a future API redesign around `repoID + revision` can still improve UX, but
    it is not required to make the live path work today.

## Live `Gemma 4` published artifact integrity check

- Scenario:
  - inspect the published `DMCR` artifact for
    `ai-models-smoke/gemma-4-e2b-it-smoke-20260413-1` after `phase=Ready`;
  - verify not only status/digest, but actual manifest and blob payloads in the
    registry backend.
- Result:
  - live manifest was structurally valid but semantically wrong for a real
    model payload:
    - `config.size=353`
    - single weight-layer digest
      `sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef`
    - `layer.size=1024`;
  - direct blob fetch from `DMCR` confirmed the published layer tar itself was
    only `1024` bytes;
  - direct `tar -tvf` on that layer produced no file entries, i.e. the layer
    was an empty tar shell instead of a real model filesystem.
- Root-cause proof:
  - publication code in `internal/adapters/modelpack/kitops/adapter.go`
    prepared a temporary `kitops` context via:
    - temp dir
    - `model -> <checkpointDir>` symlink
    - `Kitfile` with `model.path: model`;
  - live artifact shape is consistent with `kitops pack` publishing the symlink
    shell rather than dereferenced checkpoint contents;
  - therefore `Ready` was reached with a false-positive published artifact.
- Outcome:
  - the defect required a corrective slice: `kit pack` must run directly on the
    real model directory with `Kitfile model.path: .`, not on a symlink-based
    wrapper context;
  - this path also avoids introducing one more full local copy of the model
    just to satisfy `kitops`.

## Live `Gemma 4` smoke after source-mirror transport landing

- Scenario:
  - `Model` in namespace `ai-models-smoke`
  - `source.url=https://huggingface.co/google/gemma-4-E2B-it`
  - same live cluster after resumable source-mirror byte path landed
- Result:
  - object was accepted, but failed before resolved revision/artifact publish;
  - status and events showed:
    - `RemoteIngestStarted`
    - `PublicationFailed`
    - `Put "https://s3.api.apiac.ru/.../UploadPart...": tls: failed to verify certificate: x509: certificate signed by unknown authority`
- Root-cause proof:
  - object-storage adapter in `internal/adapters/uploadstaging/s3/adapter.go`
    already builds a CA-aware HTTP transport from the configured CA bundle;
  - source-mirror multipart upload in
    `internal/adapters/sourcefetch/huggingface_mirror_transport.go` was still
    issuing presigned `PUT` requests via `http.DefaultClient`;
  - therefore the new source-mirror byte path bypassed the configured S3 CA
    trust and regressed on clusters with custom-CA object storage.
- Outcome:
  - corrective slice required:
    - preserve the CA-aware HTTP client inside the S3 adapter;
    - propagate it through publishworker/sourcefetch into
      `SourceMirrorOptions`;
    - use it for presigned multipart `UploadPart`;
    - add regression coverage against a TLS endpoint trusted only by that
      custom client.

## Live delete / GC evidence

- Scenario:
  - delete `ai-models-smoke/tiny-random-phi-smoke-20260412-1` after successful
    `HF -> Ready` publication;
- Result:
  - `ai-model-cleanup-97d13bfc-70b9-43b1-9d35-d77c0b37d7ac` completed;
  - `dmcr-gc-97d13bfc-70b9-43b1-9d35-d77c0b37d7ac` appeared and then
    disappeared;
  - `Model.status.phase=Deleting` temporarily exposed
    `CleanupCompleted=False reason=CleanupPending`;
  - the `Model` object was removed after finalizer release;
  - direct registry reads of both:
    - `.../manifests/published`
    - `.../manifests/<digest>`
    returned `404` after cleanup.
- Extra inspection:
  - bucket inspection proved that the old GC path still left visible residue:
    - old `raw/<uid>/...` objects for failed publishes;
    - `dmcr/.../repositories/.../_layers/*/link` metadata for deleted repos.
- Outcome:
  - live GC protocol was confirmed as functional for logical delete and blob
    reachability;
  - Slice 73 was required to close the user-visible object-storage residue that
    remained after those logical steps.
