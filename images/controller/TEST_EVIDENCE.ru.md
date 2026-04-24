# Test Evidence

Этот файл — единственный канонический inventory decision-coverage для
`images/controller`.

История migration slices больше не живёт здесь. Historical log остаётся в
archived bundles, а этот файл фиксирует только current live runtime, его byte
paths и test surfaces.

Жёсткие правила:

- `_test.go` файлы под `images/controller/internal` живут под тем же LOC-budget,
  что и production files:
  - production: `tools/check-controller-loc.sh`
  - tests: `tools/check-controller-test-loc.sh`
  - budget: `< 350` строк без allowlist-first мышления
- tests режутся по decision surface, а не по “что удобно было положить рядом”;
- helper-only test file допустим только как thin shared seam внутри одного
  package;
- package-local branch matrix и historical slice notes рядом с кодом не
  допускаются: evidence централизована только здесь.

## 1. Current byte path proofs

### Direct `HuggingFace`

- Byte path:
  - `resolved HF objects -> bounded companion bundle + raw OCI publish`
- Runtime selection:
  - `artifacts.sourceFetchMode=Direct`
- Full-size local copies during successful publish:
  - `0`
- Primary evidence:
  - `internal/adapters/sourcefetch/huggingface_fetch_direct_test.go`
  - `internal/adapters/sourcefetch/huggingface_fetch_failure_test.go`
  - `internal/dataplane/publishworker/huggingface_fetch_test.go`
  - `internal/dataplane/publishworker/huggingface_streaming_test.go`
  - `internal/adapters/k8s/sourceworker/build_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_object_source_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_mixed_layout_test.go`
  - `internal/adapters/modelpack/oci/published_model_path_test.go`
- Proved properties:
  - worker no longer allocates local `workspace/model`;
  - remote profile summary/object-source planning is mandatory;
  - planning failure is explicit and no longer degrades to local materialization;
  - multi-file object source can publish as bounded companion bundle plus raw
    weight layers without breaking the stable materialized model contract;
  - controller/sourceworker wiring can disable raw-stage mirror args entirely.

### Mirrored `HuggingFace`

- Byte path:
  - `HF objects -> source mirror objects -> bounded companion bundle + raw OCI publish`
- Runtime selection:
  - `artifacts.sourceFetchMode=Mirror`
- Full-size local copies during successful publish:
  - `0`
- Primary evidence:
  - `internal/adapters/sourcefetch/huggingface_mirror_fetch_test.go`
  - `internal/adapters/sourcefetch/huggingface_mirror_resume_test.go`
  - `internal/dataplane/publishworker/huggingface_streaming_test.go`
  - `internal/dataplane/publishworker/rawstage_test.go`
  - `internal/adapters/k8s/sourceworker/build_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_object_source_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_mixed_layout_test.go`
- Proved properties:
  - publish continues from mirrored objects, not from local rematerialization;
  - mirror resume/TLS edges stay covered on the source fetch boundary;
  - companion files can stay in bounded bundle layers while heavy model payloads
    remain standalone raw blobs for reuse and resume;
  - mirror state/manifest JSON now decodes through `OpenRead`, not temp-file
    download.

### `DMCR` upload transport

- Primary evidence:
  - `internal/adapters/modelpack/oci/publish_test.go`
  - `internal/adapters/modelpack/oci/direct_upload_test.go`
  - `internal/adapters/modelpack/oci/direct_upload_resume_test.go`
  - `internal/adapters/modelpack/oci/direct_upload_server_test_helpers_test.go`
  - `internal/adapters/modelpack/oci/direct_upload_protocol_test_helpers_test.go`
  - `internal/adapters/k8s/sourceworker/build_test.go`
  - `internal/adapters/k8s/sourceworker/validation_test.go`
  - `internal/adapters/k8s/sourceworker/service_progress_test.go`
  - `cmd/ai-models-controller/config_test.go`
- Proved properties:
  - published model blobs no longer have a runtime transport toggle and always
    upload through the `DMCR`-owned direct-upload v2 helper into backing
    storage;
  - streamable multi-file object-source inputs now keep large payload files as
    dedicated raw layers while bundling small companion files into one bounded
    tar layer;
  - range-capable raw layers no longer require a controller-side full
    descriptor pre-read before upload; the helper receives final digest/size
    only at session completion;
  - raw object-source retry/recovery stays on ranged reads and no longer falls
    back to whole-object `OpenRead()` on the hot publish path;
  - interrupted raw-layer upload can continue from persisted controller-side
    checkpoint state plus helper `listParts()` while the direct-upload session
    is still alive;
  - running publication status now carries machine-readable progress reasons
    for started / uploading / resumed / sealing / committed edges instead of
    exposing only one opaque message string;
  - top-level `status.progress` for sourceworker-driven publication now comes
    from explicit checkpoint totals, stays independent from condition-message
    scraping, and remains capped below `100%` until terminal `Ready`;
  - direct transport still preserves final remote inspect and consumer-side
    materialization correctness;
  - interrupted direct part upload recovers through helper `listParts()` instead
    of degrading to local rematerialization or restarting the whole publish.

### Local direct upload

- Byte path:
  - direct `GGUF`: `local file -> native OCI raw publish`
- Full-size local copies during successful publish:
  - `1`, only the original input file
- Primary evidence:
  - `internal/domain/ingestadmission/upload_session_test.go`
  - `internal/domain/ingestadmission/upload_probe_test.go`
  - `internal/domain/ingestadmission/upload_probe_shape_test.go`
  - `internal/dataplane/publishworker/upload_probe_test.go`
  - `internal/dataplane/publishworker/upload_workspace_test.go`
- Proved properties:
  - invalid direct uploads fail before workspace creation;
  - successful direct local publish no longer creates `checkpointDir` or empty
    upload workspace roots.

### Local streamable archive upload

- Byte path:
  - `local archive -> archive inspection -> native OCI archive-source publish`
- Supported live happy-path families:
  - `tar`
  - `tar.gz` / `tgz`
  - `zip`
  - `tar.zst` / `tar.zstd` / `tzst`
- Full-size local copies during successful publish:
  - `1`, only the original archive file
- Primary evidence:
  - `internal/adapters/sourcefetch/archive_test.go`
  - `internal/adapters/sourcefetch/archive_zstd_test.go`
  - `internal/adapters/sourcefetch/archive_inspect_zip_reader_test.go`
  - `internal/dataplane/publishworker/upload_streaming_test.go`
  - `internal/dataplane/publishworker/upload_streaming_zstd_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_zip_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_zstd_test.go`
- Proved properties:
  - canonical archives publish without extracted `checkpointDir`;
  - archive-root normalization and benign-extra stripping stay in the native
    publisher path, not in ad-hoc worker temp shells.

### Staged upload

- Byte path:
  - `staged object -> raw object-read / range-read streaming publish`
- Full-size local copies during successful publish:
  - `0`
- Primary evidence:
  - `internal/adapters/uploadstaging/s3/object_io_test.go`
  - `internal/dataplane/publishworker/upload_stage_streaming_test.go`
  - `internal/dataplane/publishworker/upload_stage_release_test.go`
- Proved properties:
  - staged publish requires streaming-capable object reads;
  - `Download`-only successful fallback is gone;
  - staged valid uploads no longer route through `checkpointDir`;
  - single-file staged inputs now publish as one raw layer instead of a
    synthetic tar layer.

### Consumer-side materialization

- Byte path:
  - `published OCI artifact -> workload cache root`
- Full-size local copies during successful materialization:
  - `1`, inside the consumer-owned cache/storage surface
- Primary evidence:
  - `internal/adapters/modelpack/oci/materialize_test.go`
  - `internal/adapters/k8s/modeldelivery/render_test.go`
  - `internal/adapters/k8s/modeldelivery/service_apply_test.go`
  - `internal/adapters/k8s/modeldelivery/service_test.go`
  - `internal/adapters/k8s/modeldelivery/service_topology_test.go`
- Proved properties:
  - publication-path zero-copy claims do not pretend away consumer-side cache
    materialization;
  - workload cache policy remains separate from publish-worker byte path;
  - workload-facing runtime contract is stable through
    `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST`, and
    `AI_MODELS_MODEL_FAMILY`; per-pod delivery now reads the stable
    `/data/modelcache/model` projection, while shared PVC bridge topology
    reads a digest-scoped path inside the shared store instead of depending on
    a global `/data/modelcache/current` link.

## 2. Domain and application evidence

### `internal/domain/publishstate`

- Decision surface:
  - publication phases and terminal semantics
  - upload status equality
  - worker/session observation decisions
  - status/condition projection
- Primary evidence:
  - `operation_test.go`
  - `runtime_decisions_test.go`
  - `status_test.go`
  - `status_success_test.go`
  - `policy_validation_test.go`

### `internal/domain/ingestadmission`

- Decision surface:
  - owner binding invariants
  - declared input-format allowlist
  - filename normalization
  - bounded upload session validation
  - bounded upload probe classification for archive vs direct file
  - owner-agnostic probe-shape validation
- Primary evidence:
  - `common_test.go`
  - `upload_session_test.go`
  - `upload_probe_test.go`
  - `upload_probe_shape_test.go`

### `internal/application/publishplan`

- Decision surface:
  - source-worker vs upload-session planning
  - source auth secret projection
  - upload-session issuance policy
- Primary evidence:
  - `start_publication_test.go`
  - `plan_source_worker_test.go`
  - `issue_upload_session_test.go`

### `internal/application/publishobserve`

- Decision surface:
  - runtime result decoding
  - worker/session observation mapping
  - reconcile gating
  - status mutation planning
- Primary evidence:
  - `ensure_runtime_source_worker_test.go`
  - `ensure_runtime_upload_test.go`
  - `ensure_runtime_failure_test.go`
  - `ensure_runtime_clock_test.go`
  - `observe_runtime_test.go`
  - `observe_source_worker_test.go`
  - `observe_upload_session_test.go`
  - `reconcile_gate_test.go`
  - `status_mutation_test.go`

### `internal/application/sourceadmission`

- Decision surface:
  - fail-fast preflight for `source.url`
  - obvious non-HF remote rejection before byte-path starts
- Primary evidence:
  - `preflight_test.go`

### `internal/application/publishaudit`

- Decision surface:
  - append-only audit record planning over lifecycle edges
  - final publication message shaping from provenance plus OCI result
- Primary evidence:
  - `plan_test.go`

### `internal/application/deletion`

- Decision surface:
  - cleanup finalizer ownership
  - cleanup job / GC progress decision table
  - finalizer removal policy
- Primary evidence:
  - `ensure_cleanup_finalizer_test.go`
  - `finalize_delete_backend_artifact_test.go`
  - `finalize_delete_upload_staging_test.go`
  - `finalize_delete_progress_test.go`

## 3. Adapter and port evidence

### `internal/adapters/modelpack/oci`

- Decision surface:
  - native OCI publish/remove/materialize
  - resumable upload session handling
  - layer/media-type matrix
  - manifest/config validation
  - object-source, archive-source and local-file publish paths
- Primary evidence:
  - `publish_test.go`
  - `registry_server_test_helpers_test.go`
  - `registry_dispatch_test_helpers_test.go`
  - `registry_upload_state_test_helpers_test.go`
  - `registry_content_state_test_helpers_test.go`
  - `file_assert_test_helpers_test.go`
  - `adapter_test.go`
  - `inspect_test.go`
  - `materialize_test.go`
  - `layer_matrix_publish_test.go`
  - `layer_validation_test.go`
  - `layer_matrix_object_source_test.go`
  - `layer_matrix_zip_test.go`
  - `layer_matrix_zstd_test.go`

### `internal/adapters/sourcefetch`

- Decision surface:
  - `HuggingFace` remote planning
  - source mirror transfer/resume/TLS
  - archive inspection and format-safe selection
  - remote profile summary extraction
- Primary evidence:
  - `huggingface_test.go`
  - `huggingface_fetch_direct_test.go`
  - `huggingface_fetch_failure_test.go`
  - `huggingface_mirror_fetch_test.go`
  - `huggingface_mirror_resume_test.go`
  - `huggingface_mirror_tls_test.go`
  - `huggingface_profile_summary_test.go`
  - `archive_unpack_test.go`
  - `archive_inspect_safetensors_test.go`
  - `archive_inspect_gguf_test.go`
  - `archive_inspect_zip_reader_test.go`
  - `archive_zstd_test.go`

### `internal/adapters/sourcemirror/objectstore`

- Decision surface:
  - persisted manifest/state lifecycle over object storage
  - stream-decoded mirror control state
- Primary evidence:
  - `adapter_test.go`

### `internal/adapters/uploadstaging/s3`

- Decision surface:
  - multipart upload staging
  - object reads and ranged reads for publish/runtime paths
- Primary evidence:
  - `object_io_test.go`

### `internal/adapters/modelformat`

- Decision surface:
  - source-agnostic format detection
  - required-file and benign-extra policy
  - validation before packaging
- Primary evidence:
  - `detect_test.go`
  - `validation_test.go`

### `internal/adapters/modelprofile`

- Decision surface:
  - endpoint type mapping
  - precision / quantization inference
  - parameter count and minimum launch estimation
  - format-specific profile extraction
- Primary evidence:
  - `common/profile_test.go`
  - `safetensors/profile_test.go`
  - `gguf/profile_test.go`

### `internal/ports/sourcemirror` and `internal/ports/publishop`

- Decision surface:
  - shared runtime contract integrity
  - immutable mirror state shapes
- Primary evidence:
  - `internal/ports/sourcemirror/contract_test.go`
  - `internal/ports/publishop/operation_contract_test.go`
  - `internal/ports/publishop/ports_test.go`

## 4. K8s adapter evidence

### `internal/adapters/k8s/sourceworker`

- Decision surface:
  - worker Pod shaping
  - auth supplements
  - bounded resource contract
  - source-worker runtime result handoff
  - restart-safe worker recreation from persisted direct-upload state
  - bounded running progress handoff from persisted direct-upload checkpoint
- Primary evidence:
  - `auth_secret_test.go`
  - `build_test.go`
  - `service_progress_test.go`
  - `service_identity_test.go`
  - `service_auth_projection_test.go`
  - `service_concurrency_test.go`
  - `service_resume_test.go`
  - `service_roundtrip_test.go`
  - `validation_test.go`

### `internal/adapters/k8s/directuploadstate`

- Decision surface:
  - secret-backed direct-upload checkpoint normalization
  - owner-generation scoping and reset
  - current-layer, committed-layer, running-stage and terminal-phase payload validation
- Primary evidence:
  - `internal/adapters/k8s/directuploadstate/secret_test.go`

### `internal/adapters/k8s/uploadsession` and `uploadsessionstate`

- Decision surface:
  - controller-owned session issuance/reuse
  - top-level local-upload progress shaping
  - bearer-only Secret-reference status projection and token reuse
  - secret-backed phase persistence
  - persisted expected-size and multipart uploaded-part accounting
  - controller phase sync for `publishing/completed/failed`
  - multipart state persistence
- Primary evidence:
  - `service_test.go`
  - `service_get_or_create_test.go`
  - `progress_test.go`
  - `service_projection_test.go`
  - `service_delete_test.go`
  - `service_phase_sync_test.go`
  - controller-side multipart refresh from staging before progress projection
  - `internal/adapters/k8s/uploadsessionstate/secret_test.go`

### `internal/adapters/k8s/modeldelivery`

- Decision surface:
  - workload mutation for runtime delivery
  - storage topology checks
  - managed local bridge volume injection
  - cache-root, digest, delivery-mode and delivery-reason annotations
  - stable workload-facing runtime env projection and cleanup
- Primary evidence:
  - `render_test.go`
  - `service_apply_test.go`
  - `service_topology_test.go`
  - `service_validation_test.go`
  - `workload_hints_test.go`

### `internal/adapters/k8s/nodecachesubstrate`

- Decision surface:
  - managed `LVMVolumeGroupSet` shaping
  - managed thin `LocalStorageClass` shaping
  - ready managed `LVMVolumeGroup` extraction
- Primary evidence:
  - `desired_lvgset_test.go`
  - `desired_local_storage_class_test.go`
  - `managed_lvg_test.go`

### `internal/nodecache`

- Decision surface:
  - digest-addressed shared store layout
  - consumer materialization layout plus internal current-link and stable
    workload-model-link updates
  - shared-cache single-writer coordination
  - access timestamp handling
  - shared-store prefetch for desired published artifacts
  - bounded cache scan and eviction planning with protected desired digests
  - maintenance loop over malformed and idle entries
- Primary evidence:
  - `store_layout_test.go`
  - `materialization_layout_test.go`
  - `coordination_test.go`
  - `scan_test.go`
  - `plan_test.go`
  - `prefetch_test.go`
  - `maintain_test.go`

### `internal/adapters/k8s/nodecacheruntime`

- Decision surface:
  - stable per-node runtime Pod/PVC shaping
  - immutable published-artifact extraction from already-managed Pods on the
    current node only for future true shared-direct delivery
  - runtime-side loading of required published artifacts from live cluster
    truth without a dedicated mirror contract
- Primary evidence:
  - `desired_artifacts_test.go`
  - `pod_test.go`
  - `pvc_test.go`
  - `service_test.go`

### Stable per-node node-cache runtime plane

- Decision surface:
  - controller render passes stable shared-volume size contract when
    `nodeCache.enabled=true`
  - module render keeps only service-account/RBAC surface for the runtime
    agent; legacy `DaemonSet` render is gone
  - stable per-node Pod/PVC contract over ai-models-owned `LocalStorageClass`
  - maintenance env contract for shared cache-root
  - runtime-side live Pod lookup scoped to the current node
  - DMCR read-auth projection for shared-store prefetch
  - read-only Pod-list RBAC contract for the node-cache agent
  - dedicated service-account contract for the node-cache agent
- Primary evidence:
  - `pod_test.go`
  - `pvc_test.go`
  - `service_test.go`
  - `options_test.go`
  - `reconciler_test.go`
  - `tools/helm-tests/validate_renders_test.py`
  - `tools/helm-tests/validate-renders.py`
  - `make helm-template`
  - `make kubeconform`

### Other concrete K8s adapters

- `auditevent`: `recorder_test.go`
- `ociregistry`: `projection_test.go`, `render_test.go`
- `ownedresource`: `lifecycle_test.go`

## 5. Dataplane and controller evidence

### `internal/dataplane/publishworker`

- Decision surface:
  - direct HF/mirror publish shell
  - remote profile resolution
  - upload probing and streaming publish
  - provenance shaping
- Primary evidence:
  - `upload_run_test.go`
  - `huggingface_fetch_test.go`
  - `huggingface_streaming_test.go`
  - `remote_profile_test.go`
  - `upload_probe_test.go`
  - `upload_stage_streaming_test.go`
  - `upload_stage_release_test.go`
  - `upload_streaming_test.go`
  - `upload_streaming_zstd_test.go`
  - `upload_workspace_test.go`
  - `workspace_test.go`
  - `result_test.go`
  - `provenance_test.go`
  - `rawstage_test.go`

### `internal/dataplane/uploadsession`

- Decision surface:
  - bearer-only request authentication
  - probe-time expected-size capture
  - session info and liveness
  - upload probe / multipart init
  - multipart completion and controller handoff rejection
  - expiry and closed-session rejection
- Primary evidence:
  - `run_session_info_test.go`
  - `run_probe_init_test.go`
  - `run_multipart_completion_test.go`
  - `run_multipart_handoff_test.go`
  - `run_session_expiry_test.go`

### `internal/dataplane/artifactcleanup`

- Decision surface:
  - backend prefix shaping
  - published artifact removal runtime
- Primary evidence:
  - `backend_prefix_test.go`
  - `run_test.go`

### Controllers

- `catalogstatus`:
  - `reconciler_test.go`
  - `reconciler_upload_test.go`
  - `runtime_fakes_test.go`
  - `reconciler_test_helpers_test.go`
  - `runtime_handle_test_helpers_test.go`
- `catalogcleanup`:
  - `apply_test.go`
  - `gc_request_test.go`
  - `job_test.go`
  - `observe_test.go`
  - `reconciler_finalizer_test.go`
  - `reconciler_delete_test.go`
  - `reconciler_gc_test.go`
- `nodecachesubstrate`:
  - `options_test.go`
  - `reconciler_test.go`
- `nodecacheruntime`:
  - `reconciler_test.go`
- `workloaddelivery`:
  - `annotations_test.go`
  - `options_test.go`
  - `predicate_test.go`
  - `reconciler_apply_test.go`
  - `reconciler_cleanup_test.go`
  - `reconciler_topology_test.go`

## 6. Shared shell evidence

- `internal/bootstrap`: `bootstrap_test.go`
- `internal/cmdsupport`: `common_test.go`
- `internal/monitoring/catalogmetrics`:
  - `collector_state_metrics_test.go`
  - `collector_incomplete_status_test.go`
  - `collector_test_helpers_test.go`
- `internal/monitoring/runtimehealth`:
  - `collector_nodecache_runtime_test.go`
  - `collector_workload_delivery_test.go`
  - `collector_test_helpers_test.go`
- `internal/publicationartifact`: `contract_test.go`, `location_test.go`
- `internal/publishedsnapshot`: `snapshot_test.go`
- `internal/support/cleanuphandle`: `handle_test.go`
- `internal/support/modelobject`: `modelobject_test.go`
- `internal/support/resourcenames`: `names_test.go`

## 7. Residual risks

- Current evidence proves the live publication/runtime baseline plus the first
  real node-local shared-cache prefetch plane, but does not claim the final
  workload-facing shared mount service.
- Consumer-side materialization into workload cache remains intentional and is
  not hidden by publication-path zero-copy claims.
- Historical migration detail remains available in archived bundles, not in this
  file.
