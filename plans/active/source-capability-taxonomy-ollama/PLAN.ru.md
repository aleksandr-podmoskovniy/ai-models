# Plan: source capability taxonomy and Ollama source support

## 1. Current phase

Phase 1/2 boundary: publication baseline уже поддерживает HF/upload,
Diffusers/GGUF/Safetensors profile extraction and registry-backed
publication. Эта задача готовит чистую provider taxonomy и расширяет source
fetch без runtime placement обещаний.

## 2. Orchestration

Mode: `light` for implementation.

Reason: task touches public CRD/status semantics and source provider
architecture. Before implementation, request read-only review for:

- `api_designer`: public `status.resolved` field shape and compatibility.
- `integration_architect`: Ollama source byte path, storage/mirror and HA
  semantics.

Current note: this bundle records research/design first; implementation should
start only after the field-shape decision below is accepted.

## 3. Active bundle disposition

- `capacity-cache-admission-hardening` — keep; executable storage/cache
  workstream.
- `live-e2e-ha-validation` — keep; canonical post-rollout validation runbook.
- `observability-signal-hardening` — keep; observability workstream.
- `pre-rollout-defect-closure` — keep until current cleanup diff is committed
  or archived.
- `ray-a30-ai-models-registry-cutover` — keep; external manifest/load
  validation workstream.
- `source-capability-taxonomy-ollama` — current metadata/source provider
  workstream.

## 4. Research baseline

Hugging Face:

- `pipeline_tag` is a model-card metadata field that indicates intended task,
  is shown on model pages, enables task filtering and drives widget/API
  selection on HF.
- HF widgets infer a single widget from `pipeline_tag` for simplicity; for
  some libraries HF can infer it from config, otherwise task tags and
  model-card metadata are used.
- `model_info` can return explicit fields such as `pipeline_tag`, `tags`,
  `library_name`, `siblings`, `safetensors`, `gguf`, `inference` and
  `inferenceProviderMapping`.

Ollama:

- API `show` exposes `details` (`format`, `family`, `parameter_size`,
  `quantization_level`), `model_info` (`general.parameter_count`,
  architecture/context fields), `capabilities`, template, parameters and
  license.
- API `pull` is streaming and reports manifest/layer download progress.
- Registry manifest for `ollama.com/library/qwen3.6:latest` is Docker
  distribution shaped and currently contains:
  - one `application/vnd.ollama.image.model` layer, size `23938321664`;
  - `application/vnd.ollama.image.license`;
  - `application/vnd.ollama.image.params`;
  - config with `model_format=gguf`, `model_family=qwen35moe`,
    `model_type=36.0B`, `file_type=Q4_K_M`.
- The public HTML page also shows UX labels such as `tools`, `Text`, `Image`
  and `256K context window`, but controller code must not depend on scraping
  HTML; for exact metadata use manifest/config plus a bounded GGUF header
  range read from the model layer.

Conclusion:

- HF `task` and Ollama `capabilities` are provider/source taxonomy.
- `supportedEndpointTypes` is our normalized serving taxonomy.
- Keeping both is valid only if names/docs make that distinction impossible to
  miss. If not, public `task` should be renamed or moved out of the main
  summary.

## 5. Field-shape decision

Preferred minimal-noise CRD shape:

```yaml
status:
  resolved:
    format: GGUF
    architecture: qwen35moe
    family: qwen35moe
    parameterCount: 36000000000
    quantization: Q4_K_M
    contextWindowTokens: 131072
    supportedEndpointTypes:
      - Chat
      - TextGeneration
    supportedFeatures:
      - ToolCalling
    sourceCapabilities:
      provider: Ollama
      tasks:
        - completion
      features: []
```

Rationale:

- `supportedEndpointTypes` remains the consumer-facing field for
  `ai-inference`.
- `supportedFeatures` remains provider-neutral and orthogonal.
- `sourceCapabilities` is provenance/evidence and can hold HF `pipeline_tag`
  or Ollama `capabilities` without pretending they are our runtime contract.
- Existing `task` becomes either deprecated projection of the primary source
  task for compatibility or is removed before public stabilization.

Fallback if we avoid CRD churn in this slice:

- keep `task`;
- document it as `source/provider task`;
- never treat it as the scheduler input;
- add `TaskConfidence`/internal evidence only in snapshot, not public status.

## 6. Slices

### Slice 1. Contract tightening

Goal:

- decide whether to keep, rename, or nest public `task`;
- update docs and tests so `text-ranking` + `Rerank` no longer looks like two
  competing task fields.

Files:

- `api/core/v1alpha1/types.go`
- `docs/development/MODEL_METADATA_CONTRACT.ru.md`
- generated CRDs/deepcopy if CRD changes.

Checks:

- `cd api && go test ./...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`

### Slice 2. Ollama source detection contract

Goal:

- support `https://ollama.com/library/<name>[:tag]` and equivalent library
  references without relaxing HF validation accidentally.

Files:

- `api/core/v1alpha1/types.go`
- `images/controller/internal/domain/modelsource/*`
- source admission tests.

Checks:

- source admission/modelsource tests;
- CRD verify if URL pattern changes.

### Slice 3. Ollama remote adapter

Goal:

- add `sourcefetch/ollama` narrow logic for registry manifest/config/layer
  discovery, size planning, digest validation and source mirror integration.

Files:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/ports/sourcemirror/*` only if a generic remote
  manifest abstraction is needed.

Checks:

- sourcefetch tests with fixture manifest/config/layers;
- negative malformed manifest tests.

### Slice 4. Ollama profile projection

Goal:

- project Ollama GGUF metadata into internal profile and public status without
  filename guesses.
- add a bounded GGUF header parser/range-reader path before full model
  download, so context length, architecture and tokenizer facts come from the
  artifact itself rather than from Ollama HTML.

Files:

- `images/controller/internal/adapters/modelprofile/gguf/*`
- `images/controller/internal/publishedsnapshot/*`
- `images/controller/internal/dataplane/publishworker/*`

Checks:

- GGUF profile tests;
- publishworker remote profile tests.

### Slice 5. End-to-end evidence

Goal:

- publish one small Ollama model and one larger Ollama GGUF model through
  mirror/direct paths and prove DMCR path, status metadata, cleanup and logs.

Files:

- `plans/active/live-e2e-ha-validation/*`
- docs if runbook changes.

Checks:

- live e2e runbook after rollout;
- no controller/DMCR restarts;
- no retained publish Pods/state after Ready.

## 7. Rollback point

Rollback is reverting the CRD/sourcefetch/profile changes. If only docs/design
are changed, rollback is deleting this bundle and reverting docs.

## 8. Final validation

- `cd api && go test ./...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./...`
- `make helm-template`
- `make kubeconform`
- `make verify`
