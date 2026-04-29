# PLAN: Efficient ModelPack compression and resumable distribution

## Active bundle disposition

- keep `plans/active/live-e2e-ha-validation`: отдельный live validation
  workstream.
- keep `plans/active/observability-signal-hardening`: отдельный observability
  hardening workstream.
- archived `plans/archive/2026/ray-a30-ai-models-registry-cutover`:
  KubeRay-specific cutover premise superseded by generic PodTemplate CSI
  delivery contract.
- keep `plans/active/modelpack-efficient-compression-design`: текущий
  architecture/design workstream.

## Orchestration

Mode: `full`.

Read-only reviews used:

- `integration_architect`: HA/storage/runtime review for chunked layout,
  registry range, node-cache materialization and observability.
- `backend_integrator`: DMCR/direct-upload, checkpoint and OCI compatibility
  review.
- `repo_architect`: package/boundary review.

Captured decisions from reviews:

- Keep v2 logic inside `adapters/modelpack/oci`; do not create a new
  domain/application/nodecache package for chunk internals in v1.
- Do not expand `ports/modelpack` with DMCR presign, OCI media types or
  per-chunk transport maps.
- Add chunked materialize/ranged read support before enabling chunked publish.
- Raw chunked rollout comes before mixed zstd/raw compression.
- Minimal range/resume/fallback metrics must exist before canary.

## Current code findings

- `internal/ports/modelpack/contract.go` уже имеет `LayerFormatRaw`,
  `LayerFormatTar`, `LayerCompressionNone/Gzip/GzipFastest/Zstd`.
- `raw` object-source layer сейчас запрещает compression, что защищает
  range/resume path от sequential compressed stream.
- `direct_upload_transport_raw_flow.go` сохраняет `UploadedSizeBytes`,
  `DigestState` и part state, поэтому raw layer upload можно продолжить после
  сбоя.
- `publish_object_source_range.go` умеет синтезировать ranged tar только для
  uncompressed archive.
- legacy `materialize_layers.go` тянет file-layer blob целиком. Chunked v1
  materialize routing уже добавлен для synthetic chunk-index/chunk-pack
  artifacts, но registry range reader, per-chunk markers and resume ещё не
  реализованы.
- Текущий `tar+zstd` полезен для маленьких config/doc/code слоёв, но не решает
  быструю доставку больших weight shards.
- DMCR direct-upload complete API принимает итоговые `Digest` и `SizeBytes`, а
  raw direct-upload path открывает session после вычисления `rawPublishLayerSize`.
  Поэтому compressed `chunk-pack` нельзя честно стримить прямо в registry, пока
  его compressed size ещё неизвестен. V1 должен либо:
  - собирать bounded pack в durable temp object/file, затем direct-upload с
    известным размером;
  - либо расширять DMCR direct-upload protocol отдельным backend slice.
- Текущие OCI validation/materialize assumptions file-layer-oriented:
  каждый layer обязан иметь `org.cncf.model.filepath`, а `diffIds` считаются по
  физическим layers. Для chunked artifacts нужно явно разделить logical file
  index и opaque pack blobs.
- Текущий direct-upload checkpoint хранится одним Secret и перезаписывается на
  каждый save. Per-chunk checkpoint без compaction может упереться в размер
  Secret и API churn.
- Registry range pull сейчас не является контрактом: blob reader ждёт full
  `200 OK`; chunked materialize должен добавить `206 Partial Content`, fallback
  and recovery semantics.

## External references

- OCI Distribution Spec: blob upload is chunked/resumable and blob pull supports
  range semantics where registry implements it:
  `https://github.com/opencontainers/distribution-spec/blob/main/spec.md`.
- OCI Image Spec: descriptors carry media type, digest, size and annotations;
  this is enough to encode immutable chunk/index metadata without exposing
  backend internals as public API:
  `https://github.com/opencontainers/image-spec/blob/main/descriptor.md`.
- containerd stargz/eStargz: production pattern for seekable/lazy access is an
  indexed compressed format, not blind monolithic gzip:
  `https://github.com/containerd/stargz-snapshotter/blob/main/docs/estargz.md`.
- zstd seekable format: random-access compression requires frame index; plain
  zstd stream alone is sequential:
  `https://github.com/facebook/zstd/blob/dev/contrib/seekable_format/zstd_seekable_compression_format.md`.
- Nydus/RAFS class of designs: split metadata/bootstrap from content chunks to
  enable lazy pull and deduplicated distribution:
  `https://github.com/dragonflyoss/nydus/blob/master/docs/nydus-design.md`.
- AWS SOCI: production-grade lazy loading for OCI images is implemented through
  a separate seekable index, confirming that index-first distribution is safer
  than changing the payload contract blindly:
  `https://github.com/awslabs/soci-snapshotter`.
- Hugging Face Xet storage: model hosting already moved toward content-addressed
  chunking/dedup for large files; useful reference for future cross-model dedup,
  but `ModelPack` should start with simpler fixed-offset chunks:
  `https://huggingface.co/docs/hub/storage-backends`.
- Hugging Face Safetensors: runtime value is safe/lazy tensor loading from the
  original file bytes, so materialization must reconstruct the exact original
  file layout instead of requiring runtimes to read a custom compressed format:
  `https://huggingface.co/docs/safetensors/index`.
- Deckhouse virtualization docs: user-facing storage UX separates stored size
  from unpacked size for images; `ModelPack` should expose the same kind of
  operational truth as `storedBytes` / `materializedBytes` / `reservedBytes`.

## Decision

Do not compress the whole model as one `tar+zstd`.

Target: add `ModelPack v2 chunked large-file layout`:

- small metadata/config/code/doc files remain grouped as `tar+zstd`;
- large weight files become chunk-indexed immutable files;
- every large file has a compact index with target path, total size, source
  evidence, file digest, chunk size and chunk descriptors;
- each chunk descriptor records uncompressed offset/size, compressed size,
  digest, codec and pack offset when chunks are grouped;
- compression is `Auto`: sample/probe first, then choose `none` or low-level
  zstd per chunk;
- upload/materialize checkpoints are per chunk, not per whole artifact.
- startup speed comes from parallel chunk download, resume, local chunk reuse
  and avoiding repeated full-blob pulls; compression is only an opportunistic
  byte-saver when measured ratio justifies the CPU.
- Implementation contract lives in `CONTRACT.ru.md`: media types, index schema,
  validation rules, bin-pack policy, checkpoint budget, range pull semantics and
  rollout gates.

## Online vs post-download compression

There are two valid modes, both internal:

- `ingest-time chunking`: for range-capable stable sources, build chunk packs
  while publishing. Raw chunks can be streamed; compressed chunks require
  bounded pack staging because final pack size is unknown before compression.
- `post-ingest repack`: for already uploaded legacy/raw artifacts, create a new
  optimized artifact digest from the canonical source artifact in the background
  and switch status atomically only after the optimized manifest is complete.

Do not mutate an existing OCI artifact in place. OCI digests are immutable; a
repacked artifact is a new digest with lineage to the previous digest.

## Key correction from code review

The current raw direct-upload path already has a strong resume primitive:
`UploadedSizeBytes`, persisted digest state and ranged source reads. The weak
point is materialization: `materialize_layers.go` downloads each blob as one
stream and exposes the result only after full extraction. Therefore the first
implementation slice must not rewrite source fetch first. It must introduce the
chunk index and chunk-aware materializer while keeping legacy raw/tar artifacts
readable.

## Why this shape

- Resume is simple: completed chunks never need to be recompressed or reuploaded.
- CPU is bounded: compression happens on independent chunks with worker limits.
- Bad compression ratio is harmless: the planner stores raw chunks when zstd is
  not worth the CPU and memory.
- Materialize can parallelize independent chunks, verify digest, write by offset
  into a temp file and resume from completed chunk markers.
- Node-cache can reuse chunk/index metadata for prefetch and eviction without a
  second layout.
- Existing artifacts remain readable because legacy `raw` and `tar` extraction
  stays intact.

## Boundary decision

- `publishworker` remains source selection and source normalization only.
- `adapters/modelpack/oci` owns chunk planning, index/pack media types, registry
  range reads, bounded pack staging and DMCR direct-upload details.
- `ports/modelpack` must not grow OCI media types, DMCR presign protocol or
  per-chunk transport maps. Keep it as the coarse publisher/materializer port
  unless a later cleanup separates existing direct-upload checkpoint state.
- `nodecache` must not know chunk manifests or pack ids. It calls
  `Materializer`; capacity/admission/eviction use materialized bytes.

## Implementation slices

### Slice 1. Chunk planning contract

Files:

- `images/controller/internal/adapters/modelpack/oci/chunk_plan*.go`
- `images/controller/internal/adapters/modelpack/oci/chunk_policy*.go`
- `plans/active/modelpack-efficient-compression-design/CONTRACT.ru.md`

Work:

- add adapter-internal chunk planning structs, not public CRD fields and not
  port-level OCI details;
- define `CompressionPolicyAuto`, chunk size defaults and ratio thresholds;
- record source ETag/size/generation evidence for resumable source reads;
- add planner tests proving low-compressibility data stays raw.
- add hard memory/disk bounds for chunk and pack building.

Checks:

- `cd images/controller && go test ./internal/adapters/modelpack/oci/...`

### Slice 2. OCI encoding

Files:

- `images/controller/internal/adapters/modelpack/oci/*`

Work:

- add chunk index media type and validation. The index owns logical file paths;
- add grouped chunk pack media type. Pack blobs are opaque and do not have
  `org.cncf.model.filepath`;
- define chunked `modelfs.diffIds` semantics separately from physical pack
  layer count;
- update validation/materialize routing so legacy file layers and chunked pack
  layers are both accepted;
- keep legacy media types accepted;
- encode index through OCI descriptors/annotations without huge manifests.
- validate path traversal, duplicate paths, overlapping offsets, invalid pack
  offsets, unsupported codec and mismatched file/chunk digests.

Checks:

- `cd images/controller && go test ./internal/adapters/modelpack/oci/...`

Implemented in current pass:

- added internal chunk-index/chunk-pack media types and immutable index structs;
- added manifest classification so legacy file layers and chunked pack layers
  are separate layouts instead of forcing `model.filepath` on opaque packs;
- added chunk-index validation for path traversal, duplicate paths, chunk
  coverage, pack range bounds, pack descriptor mismatch and unsupported codec;
- added chunked materializer path that reconstructs files from pack chunks,
  verifies per-chunk stored digest and final file digest, and preserves the
  stable `model/` contract path;
- kept publish chunking disabled.

### Slice 3. Streaming publish with resumable chunks

Files:

- `images/controller/internal/adapters/modelpack/oci/direct_upload_*`
- `images/controller/internal/dataplane/publishworker/*`
- source object readers where needed.

Work:

- read source by HTTP/object ranges when available;
- validate every ranged read against expected source evidence where backend
  returns `ETag`/generation/size; fail closed on mismatch;
- for upload/archive sources without range support, use the already durable raw
  stage as the restart boundary and then chunk from that stable object;
- compress/probe per chunk with bounded workers and memory caps;
- bin-pack chunks into bounded `chunk-pack` blobs; for compressed packs, stage
  the current pack to a durable temporary object/file before opening DMCR
  direct-upload, unless the DMCR direct-upload protocol is extended first;
- checkpoint completed chunks, current pack builder state and committed pack
  descriptors;
- keep checkpoint state compact: store ranges/bitsets or committed pack
  descriptors, not one verbose JSON object per chunk; define max Secret payload
  and save frequency;
- fail closed when ETag/size/generation changed during resume.

Checks:

- restart/replay tests around checkpointed chunk upload;
- object-source tests with forced mid-stream failure.
- source mutation test proving stale source is rejected before manifest commit.
- compressed-pack test proving restart does not recompress already committed
  chunks differently and does not duplicate bytes in DMCR.
- checkpoint-size test for a large model with many chunks.

### Slice 4. Ranged blob reader and resumable materialize

Files:

- `images/controller/internal/adapters/modelpack/oci/materialize*`
- `images/controller/internal/adapters/modelpack/oci/http_client.go`

Work:

- add registry ranged blob reader helper;
- handle `206 Partial Content`, `200 OK` full-body fallback, ignored `Range`
  and `416 Range Not Satisfiable`;
- emit a signal when materialize falls back to full-pack download;
- parallel chunk download with bounded concurrency;
- write to temp file by offset, verify per-chunk digest;
- persist chunk completion markers;
- atomic rename only after full file digest verification.
- use registry range GET for pack slices when supported; fallback to full-pack
  download with local extraction so registries without range still work.

Checks:

- materialize interrupted after N chunks then resumed;
- materialize tests for `206`, `200` fallback, ignored `Range` and `416`;
- corrupted chunk rejected and retried.

Not implemented yet:

- registry range reader and `206` / `200` / `416` behavior;
- per-chunk destination markers and resume after materializer restart;
- bounded parallel chunk download/decompression.

### Slice 5. Node-cache and observability guardrails

Files:

- `images/controller/internal/nodecache/*`
- `images/controller/internal/dataplane/nodecacheruntime/*`
- controller status/metrics/logging areas after concrete implementation.

Work:

- expose artifact layout summary in internal status/evidence;
- account `reservedBytes` using worst-case raw materialized size before
  compression result;
- account `storedBytes` after manifest commit from actual chunk/pack sizes;
- account `materializedBytes` from reconstructed model files, equivalent to the
  virtualization `UNPACKEDSIZE` concept;
- node-cache scheduling/admission/eviction must use `materializedBytes`, not
  `storedBytes`;
- partial chunk markers/temp files must be classified as in-progress and never
  as ready cache entries;
- pull minimal rollout observability forward before canary:
  `range_supported`, `full_pack_fallback`, `resume_count`,
  `pack_staged_bytes`, `compression_decision`;
- emit metrics for compression ratio, chunks completed, resume count and bytes
  saved;
- unify log fields: `artifact`, `source`, `chunk`, `offset`, `size_bytes`,
  `duration_ms`, `compression`, `stored_bytes`, `materialized_bytes`.

Checks:

- metrics/unit tests;
- node-cache scanner tests for in-progress chunk state;
- e2e runbook with DMCR/controller/publishworker restarts.

## Guardrails

- No user-facing compression knobs in first implementation.
- No sequential compressed large model blob as default.
- No second node-cache layout.
- No HTML scraping or provider-specific shortcuts in ModelPack adapter.
- No manifest with unbounded layer count; use chunk-pack grouping if chunk count
  is large.
- No per-chunk checkpoint JSON that can exceed Kubernetes Secret limits.
- No nodecache/CSI/workload code path depends on chunk index internals.
- No canary before minimal range/resume/fallback metrics exist.
- Preserve old materialize path for old artifacts.

## Final validation target

- `cd images/controller && go test ./...`
- `cd images/dmcr && go test ./...`
- `make helm-template`
- `make kubeconform`
- heavy e2e:
  - large HF model publication with worker restart;
  - upload publication with gateway restart;
  - controller restart during publish;
  - DMCR restart during direct upload and materialize;
  - node-cache prefetch interruption and resume;
  - insufficient storage fails before partial rollout.
