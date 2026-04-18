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
  - `resolved HF objects -> native OCI publish`
- Full-size local copies during successful publish:
  - `0`
- Primary evidence:
  - `internal/adapters/sourcefetch/huggingface_fetch_direct_test.go`
  - `internal/adapters/sourcefetch/huggingface_fetch_failure_test.go`
  - `internal/dataplane/publishworker/huggingface_fetch_test.go`
  - `internal/dataplane/publishworker/huggingface_streaming_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_object_source_test.go`
- Proved properties:
  - worker no longer allocates local `workspace/model`;
  - remote profile summary/object-source planning is mandatory;
  - planning failure is explicit and no longer degrades to local materialization.

### Mirrored `HuggingFace`

- Byte path:
  - `HF objects -> source mirror objects -> native OCI publish`
- Full-size local copies during successful publish:
  - `0`
- Primary evidence:
  - `internal/adapters/sourcefetch/huggingface_mirror_fetch_test.go`
  - `internal/adapters/sourcefetch/huggingface_mirror_resume_test.go`
  - `internal/dataplane/publishworker/huggingface_streaming_test.go`
  - `internal/adapters/modelpack/oci/layer_matrix_object_source_test.go`
- Proved properties:
  - publish continues from mirrored objects, not from local rematerialization;
  - mirror resume/TLS edges stay covered on the acquisition boundary;
  - mirror state/manifest JSON now decodes through `OpenRead`, not temp-file
    download.

### Local direct upload

- Byte path:
  - direct `GGUF`: `local file -> native OCI publish`
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
  - `staged object -> object-read / range-read streaming publish`
- Full-size local copies during successful publish:
  - `0`
- Primary evidence:
  - `internal/adapters/uploadstaging/s3/object_io_test.go`
  - `internal/dataplane/publishworker/upload_stage_streaming_test.go`
  - `internal/dataplane/publishworker/upload_stage_release_test.go`
- Proved properties:
  - staged publish requires streaming-capable object reads;
  - `Download`-only successful fallback is gone;
  - staged valid uploads no longer route through `checkpointDir`.

### Consumer-side materialization

- Byte path:
  - `published OCI artifact -> workload cache root`
- Full-size local copies during successful materialization:
  - `1`, inside the consumer-owned cache/storage surface
- Primary evidence:
  - `internal/adapters/modelpack/oci/materialize_test.go`
  - `internal/adapters/k8s/modeldelivery/service_test.go`
  - `internal/adapters/k8s/modeldelivery/service_topology_test.go`
- Proved properties:
  - publication-path zero-copy claims do not pretend away consumer-side cache
    materialization;
  - workload cache policy remains separate from publish-worker byte path.

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
- Primary evidence:
  - `auth_secret_test.go`
  - `build_test.go`
  - `service_identity_test.go`
  - `service_auth_projection_test.go`
  - `service_concurrency_test.go`
  - `service_roundtrip_test.go`
  - `validation_test.go`

### `internal/adapters/k8s/uploadsession` and `uploadsessionstate`

- Decision surface:
  - controller-owned session issuance/reuse
  - secret-backed phase persistence
  - controller phase sync for `publishing/completed/failed`
  - multipart state persistence
- Primary evidence:
  - `service_test.go`
  - `service_get_or_create_test.go`
  - `service_projection_test.go`
  - `service_delete_test.go`
  - `service_phase_sync_test.go`
  - `internal/adapters/k8s/uploadsessionstate/secret_test.go`

### `internal/adapters/k8s/modeldelivery`

- Decision surface:
  - workload mutation for runtime delivery
  - storage topology checks
  - cache-root and digest rollout annotations
- Primary evidence:
  - `render_test.go`
  - `service_test.go`
  - `service_topology_test.go`
  - `workload_hints_test.go`

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
- `internal/publicationartifact`: `contract_test.go`, `location_test.go`
- `internal/publishedsnapshot`: `snapshot_test.go`
- `internal/support/cleanuphandle`: `handle_test.go`
- `internal/support/modelobject`: `modelobject_test.go`
- `internal/support/resourcenames`: `names_test.go`

## 7. Residual risks

- Current evidence proves the live publication/runtime baseline, but does not
  claim future distribution work such as `DMZ` registry or node-local cache.
- Consumer-side materialization into workload cache remains intentional and is
  not hidden by publication-path zero-copy claims.
- Historical migration detail remains available in archived bundles, not in this
  file.
