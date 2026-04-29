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
- `source-capability-taxonomy-ollama` — this metadata/source provider
  workstream; archived after Slice 3/4 implementation and runbook handoff.

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
- Upstream Ollama Go registry client exists in
  `server/internal/client/ollama`, but it is intentionally not a reusable
  library for us:
  - Go `internal` package import is forbidden outside the upstream module;
  - the client owns local `.ollama/models` disk cache, chunk store, manifest
    links and resumable pull semantics;
  - importing or vendoring it would drag a second model-store contract into
    `ai-models`, while our source of truth is controller-owned `ModelPack` in
    `DMCR`.
- The correct integration shape is therefore a narrow registry adapter that
  implements only the stable byte-path we need:
  - parse `ollama.com/library/<model>[:tag]`;
  - resolve to `registry.ollama.ai/v2/library/<model>/manifests/<tag>`;
  - read manifest/config blobs;
  - select exactly one `application/vnd.ollama.image.model` layer;
  - range-read the GGUF header from that layer;
  - stream the layer into `ModelPack`/DMCR without full local materialization.

Observed registry proof for `ollama.com/library/qwen3.6:latest`:

- manifest media type is Docker distribution manifest v2;
- config digest is
  `sha256:5d1c86a949f7f3b5e75370e129765af7526f0cc1812a9de21a541da042596faa`;
- model layer is
  `application/vnd.ollama.image.model`,
  digest `sha256:f5ee307a2982106a6eb82b62b2c00b575c9072145a759ae4660378acda8dcf2d`,
  size `23938321664`;
- config contains `model_format=gguf`, `model_family=qwen35moe`,
  `model_type=36.0B`, `file_type=Q4_K_M`;
- model layer supports HTTP range reads; bytes `0..31` return `GGUF` header
  with `206 Partial Content`.

Conclusion:

- HF task taxonomy and Ollama `capabilities` are provider/source taxonomy.
- `supportedEndpointTypes` is our normalized serving taxonomy.
- Public scalar `status.resolved.task` is removed now; provider taxonomy moves
  into `status.resolved.sourceCapabilities`.

## 5. Field-shape decision

Preferred minimal-noise CRD shape:

```yaml
status:
  resolved:
    format: GGUF
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
- `status.resolved.task` is intentionally absent. Consumers must not infer a
  primary task from `sourceCapabilities.tasks`; serving decisions use
  `supportedEndpointTypes`.

## 6. Slices

### Slice 1. Contract tightening

Goal:

- remove public `status.resolved.task`;
- add `sourceCapabilities` projection and generated CRDs;
- update docs/tests so source taxonomy and serving endpoint do not compete.

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

Status:

- done in this slice for URL schema and `modelsource` parsing;
- publication no longer fails closed after Slice 3: source-worker passes the
  Ollama URL into the publish runtime and the runtime uses the registry adapter.

### Slice 3. Ollama remote adapter

Goal:

- add `sourcefetch/ollama` narrow logic for registry manifest/config/layer
  discovery, size planning, digest validation and source mirror integration.
- do not import or vendor upstream `ollama` loader; keep this as a small
  replaceable adapter around the registry protocol.

Implementation outline:

1. Add `OllamaReference` parser for `https://ollama.com/library/<name>[:tag]`
   and normalized registry path.
2. Add `OllamaRegistryClient` with `FetchManifest`, `FetchBlob`,
   `OpenLayerRange` and `OpenLayerStream` methods.
3. Add manifest projection that validates:
   - schema v2 Docker/Ollama-compatible manifest;
   - config digest and size are present;
   - exactly one model layer exists;
   - model layer media type is `application/vnd.ollama.image.model`;
   - digest/size are carried into the publish plan.
4. Integrate size planning with existing storage reservation path before
   transfer starts.
5. Implement direct publication as stream-to-DMCR raw model layer. Mirror mode
   may store the model layer as a raw object first, but must still avoid HTML
   scraping and filename guesses.
6. Keep license/params/config as metadata/evidence inputs, not as model
   weights.

Files:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/ports/sourcemirror/*` only if a generic remote
  manifest abstraction is needed.

Checks:

- sourcefetch tests with fixture manifest/config/layers;
- negative malformed manifest tests.

Status:

- done in this slice:
  - `sourcefetch` resolves Ollama library URLs through registry
    manifest/config/blob endpoints;
  - validates schema, digest, layer count, model media type and GGUF magic;
  - plans direct object-source streaming into the existing ModelPack/DMCR path;
  - reserves storage before transfer;
  - supports mirror state/transfer through the shared source-mirror tracker.
- remaining live evidence moves to `live-e2e-ha-validation`, not this design
  bundle.

Continuation on 2026-04-29:

- Implement this slice now. The target is not a second loader or a local
  `.ollama/models` layout clone; it is a source adapter that turns an Ollama
  registry reference into the same controller-owned `ModelPack` publication
  path used by HF/upload.
- Runtime selection stays out of `ai-models` status. `ai-models` publishes
  facts: source provider, source taxonomy where reliable, format, family,
  parameter count, quantization, context window and artifact reference. Public
  `architecture`, endpoint types and features stay empty when the source only
  exposes runtime/renderer hints. Future `ai-inference` chooses `vllm`,
  `ollama`, `llama.cpp`, KubeRay+vLLM, etc. from those facts plus its own
  runtime compatibility matrix and cluster policy.
- The first implementation must be fail-closed on uncertain facts: unknown
  media type, missing model layer, multiple model layers, invalid digest,
  non-GGUF model bytes or missing layer size are errors, not hints.
- E2E must assert both catalog metadata and `ai-inference` handoff inputs:
  `sourceCapabilities.provider=Ollama`, registry-derived profile facts are
  present when reliable, `supportedEndpointTypes` remains the serving-facing
  summary, and no public field claims that a concrete runtime is guaranteed.

### Slice 4. Ollama profile projection

Goal:

- project Ollama GGUF metadata into internal profile and public status without
  filename guesses.
- add bounded GGUF range validation before full model download. Richer GGUF
  parsing for context length, architecture and tokenizer facts remains a
  separate future parser slice unless those facts come from registry config.
- project provider taxonomy into `sourceCapabilities` conservatively:
  Ollama config/capabilities are source evidence; they do not automatically
  guarantee a future `ai-inference` runtime endpoint.

Files:

- `images/controller/internal/adapters/modelprofile/gguf/*`
- `images/controller/internal/publishedsnapshot/*`
- `images/controller/internal/dataplane/publishworker/*`

Checks:

- GGUF profile tests;
- publishworker remote profile tests.

Status:

- done in this slice for registry-declared GGUF facts:
  `model_family`, `model_type`, `file_type` and params `num_ctx` are projected
  as declared source facts when present.
- Ollama config `architecture=amd64` is not projected as model architecture;
  renderer/parser are also not model class. Public `architecture` waits for a
  model-level fact or GGUF parser.
- generic filename-only GGUF remains low-confidence; endpoint types are still
  omitted unless a reliable task/capability source exists.

### Slice 5. End-to-end evidence

Goal:

- publish one small Ollama model and one larger Ollama GGUF model through
  direct path first, then mirror path when enabled, and prove DMCR path, status
  metadata, cleanup and logs.
- verify future ai-inference handoff inputs without encoding runtime choice in
  `ai-models`:
  - Ollama provider/source capability evidence is present;
  - normalized endpoint/features are present only when reliable;
  - artifact `format=GGUF`, quantization and context fields are public only
    when they came from registry config or bounded GGUF evidence;
  - `vllm`/`ollama` runtime selection is not present in CR status.
- include interruption cases: controller restart, source worker restart during
  layer streaming, DMCR read-only/503 window retry, cleanup after delete.

Files:

- `plans/active/live-e2e-ha-validation/*`
- docs if runbook changes.

Checks:

- live e2e runbook after rollout;
- no controller/DMCR restarts;
- no retained publish Pods/state after Ready.

Status:

- runbook updated here; actual live execution remains in
  `plans/active/live-e2e-ha-validation`.

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

## 9. Slice validation log

2026-04-29:

- `cd api && go test ./...`
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `cd images/controller && go test ./internal/domain/modelsource ./internal/domain/publishstate ./internal/publishedsnapshot ./internal/adapters/modelprofile/... ./internal/dataplane/publishworker ./internal/adapters/sourcefetch ./internal/adapters/k8s/sourceworker ./internal/monitoring/catalogmetrics`
- `cd images/controller && go test ./...`
- `make helm-template`
- `make kubeconform`
- `git diff --check`
- `make verify`

2026-04-29 continuation:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker ./internal/adapters/k8s/sourceworker ./internal/adapters/modelprofile/gguf ./internal/ports/publishop ./cmd/ai-models-artifact-runtime`
- `cd images/controller && go test ./...`
- `cd api && go test ./...`
- `make lint-docs`
- `make lint-codex-governance`
- `git diff --check`
- `git diff --cached --check`
- live registry smoke:
  - `GET https://registry.ollama.ai/v2/library/qwen3.6/manifests/latest`
  - config blob contains `model_format=gguf`, `model_family=qwen35moe`,
    `model_type=36.0B`, `file_type=Q4_K_M`;
  - ranged model blob read returns `GGUF`.
