# DESIGN: chunked and resumable ModelPack storage layout

## Problem

Большие модели нельзя ускорить простым включением `tar+zstd`:

- `.safetensors` и quantized `.gguf` часто плохо сжимаются;
- один большой compressed stream плохо resume-ится после сбоя;
- materialize становится sequential: надо скачать и распаковать всё до конца;
- CPU compression может стать bottleneck сильнее сети;
- node-cache не может переиспользовать частично готовые данные.

Для HA registry нужен не "сжать всё", а "разбить, проверить, при необходимости
сжать маленькими независимыми кусками, продолжить после сбоя".

Главная поправка: ускорение старта модели не равно compression. Для больших
LLM/CV/audio weights чаще выигрывают:

- параллельная доставка независимых диапазонов;
- resume на границе уже проверенного chunk;
- отсутствие повторной полной загрузки при рестартах;
- node-local reuse уже готовых immutable chunks;
- ранний отказ по storage capacity до rollout.

Compression остаётся дополнительной оптимизацией только там, где замер показал
реальную экономию байт.

## Target layout

```text
ModelPack manifest
  config: modelpack metadata
  layers:
    small-files.tar+zstd
    file-index.json
    chunk-pack-000.raw-or-zstd
    chunk-pack-001.raw-or-zstd
```

Наружный runtime contract не меняется: после materialize workload видит обычный
каталог модели с оригинальными путями и байтами. Chunk/index layout — это
внутренний транспортно-хранилищный формат `ModelPack`, а не новый формат модели
для vLLM/Ollama/Diffusers.

`file-index.json`:

```json
{
  "schemaVersion": "modelpack.chunked.v1",
  "files": [
    {
      "path": "model-00001-of-00004.safetensors",
      "sizeBytes": 3900000000,
      "digest": "sha256:...",
      "source": {
        "kind": "object",
        "etag": "\"...\"",
        "sizeBytes": 3900000000
      },
      "chunkSizeBytes": 134217728,
      "chunks": [
        {
          "index": 0,
          "offset": 0,
          "uncompressedSizeBytes": 134217728,
          "storedSizeBytes": 104857600,
          "storedDigest": "sha256:...",
          "compression": "zstd",
          "pack": "chunk-pack-000",
          "packOffset": 0
        }
      ]
    }
  ]
}
```

In the OCI manifest this is not a normal file layer. The index layer owns
logical model file paths. Chunk-pack layers are opaque payload blobs and must not
carry `org.cncf.model.filepath`. Validation must therefore understand two
layouts:

- legacy file layers: each layer has a target path and extracts directly;
- chunked layout: one index layer plus opaque pack layers; the index reconstructs
  logical files.

## Compression policy

Default policy: `Auto`. Rules:

- sample first chunks and a middle chunk when range reads are available;
- compress only if estimated ratio beats threshold, for example 8-10%;
- use low-level zstd first (`level 1-3`) and bounded workers;
- store raw chunks when compression is not beneficial;
- always compress small config/doc/code archive layer with zstd;
- never require full file buffering to decide compression.

Important implementation constraint:

- raw chunks can be read/uploaded with known source size;
- compressed chunks do not have a final stored size until compression finishes;
- current DMCR direct-upload completes a blob with known `digest` and
  `sizeBytes`, and the raw path opens upload after computing layer size.

Therefore v1 compressed chunk-packs need bounded staging before direct-upload,
or a separate DMCR protocol extension. The feature must not pretend to be
fully streaming if the registry API cannot commit unknown-size blobs safely.

Explicit non-goals:

- не сжимать весь `.safetensors` или `.gguf` одним потоком;
- не оставлять runtime читать кастомный compressed-файл;
- не считать quantization/model conversion частью storage compression;
- не давать пользователю публичную крутилку compression level в первой версии.

## Chunking policy

Default large-file chunks should be fixed-offset chunks, not content-defined
chunks.

Reason:

- materialize writes reconstructed files by offset;
- per-chunk markers are deterministic and easy to resume;
- source range reads map directly to file offsets;
- validation can prove there are no overlaps or gaps.

Content-defined chunking may be useful later for cross-version dedup, but it
adds rolling-hash CPU cost, less predictable offsets and harder corruption
recovery. It should not be the first production layout.

Initial values:

- large file threshold: start around 256MiB;
- chunk size: start around 64-128MiB;
- chunk-pack stored size: target 512MiB-2GiB;
- compression workers: bounded by explicit runtime config, default low;
- memory bound: at most `workers * chunkSize * 2` plus small buffers.

Exact numbers must be benchmarked on A30-class nodes and DMCR object storage.

Rollout order should be raw chunked first, mixed compression second. Raw chunked
already fixes resume/materialize parallelism without introducing unknown-size
compressed-pack staging. Mixed compression is enabled only after pack staging,
cleanup and metrics are proven.

## Publish flow

1. Planner classifies files into small archive files and large chunked files.
2. For every large file, planner records source size and ETag/generation.
3. Worker reads chunks with range reads.
4. Worker optionally compresses each chunk independently.
5. Worker uploads chunk packs through direct-upload.
6. Checkpoint stores completed file/chunk/pack descriptors.
7. On restart, worker loads checkpoint, verifies source ETag/size and resumes
   from the first missing chunk.

If source ETag/size changed, publication fails with explicit stale source
condition instead of producing mixed content.

### Pack building

Pack building is a small bin-packing problem:

- chunks are appended to a pack until target stored size is reached;
- a pack is sealed, digest-calculated and uploaded as one OCI blob;
- the index maps each logical chunk to `pack` and `packOffset`;
- chunk count may be high, but manifest layer count stays bounded.

For raw-only packs, pack size is predictable. For mixed compressed/raw packs,
the pack must be staged while it is built because compressed size is only known
after encoding. Staging must be bounded by pack size and cleaned by cleanup/GC
the same way upload raw-stage is cleaned.

Checkpoint state must stay compact. For large models, do not persist one verbose
JSON object per chunk into a Kubernetes Secret on every part. Persist:

- source evidence snapshot;
- layout plan hash;
- committed pack descriptors;
- current pack handle;
- compact completed chunk bitmap/ranges;
- manifest-pending stage.

The implementation must define a max checkpoint payload and a save cadence that
does not turn API writes into the bottleneck.

### Online vs background optimization

`ingest-time` mode:

- used for new publications from stable range-capable source objects;
- artifact becomes `Ready` only after the chunked manifest is committed;
- no second artifact is created.

`background repack` mode:

- optional future path for legacy/raw artifacts already in DMCR;
- reads the canonical artifact, creates a new optimized artifact digest and
  atomically updates status after success;
- never mutates an existing OCI digest in place.

The first implementation should prefer `ingest-time` for new source paths and
leave background repack for a later slice.

### Source modes

`HF` / mirrored object source with range support:

- read source chunks directly by range;
- checkpoint completed chunks and current pack;
- on restart, re-check ETag/generation/size and continue missing chunks.

Upload source:

- user upload remains a separate durable raw-stage problem;
- publication chunks from the stable uploaded object;
- no worker-local full copy is required.

Archive source without efficient range:

- small archives stay legacy archive path;
- large archives should first become a stable raw-stage object, then chunk from
  that object, otherwise resume would be fake.

## Materialize flow

1. Materializer reads manifest and detects legacy or chunked layout.
2. Legacy layout uses existing extraction.
3. Chunked layout creates temp destination files.
4. Completed chunk markers are loaded from destination state.
5. Missing chunks are downloaded in bounded parallelism.
6. Every chunk is digest-verified before write.
7. File digest is verified after all chunks are present.
8. Atomic rename exposes the model only after all files are valid.

This supports restart at any chunk boundary and lets node-cache prefetch the same
layout without special semantics.

Parallelism rules:

- download concurrency is bounded independently from decompression workers;
- every chunk write uses a temp file plus marker, never partial final exposure;
- final model directory is switched atomically only after all file digests pass;
- registry range GET is used when possible, but full-pack fallback remains valid.
- node-cache capacity/admission/eviction uses `materializedBytes`, not
  `storedBytes`;
- partial chunk markers and temp files are in-progress state, never ready cache
  entries.

Registry range behavior must be explicit:

- `206 Partial Content`: use exact requested segment;
- `200 OK` with ignored `Range`: fallback to full-pack local extraction;
- `416 Range Not Satisfiable`: re-read pack descriptor/index, then fail as
  corrupt if the descriptor still expects that range.

## Why not monolithic seekable zstd first

Seekable zstd is useful but still requires an index and format-specific
reader/writer. Independent chunk packs give the same operational properties with
simpler checkpoints and simpler corruption recovery. Seekable zstd can be added
later as an optimization inside chunk packs, not as the primary artifact
contract.

The same lesson appears in lazy OCI systems: eStargz, SOCI and Nydus all add an
index/bootstrap layer around immutable content instead of relying on one opaque
compressed blob.

## Why not one OCI layer per tiny chunk

Huge manifests are operationally risky. Default should group chunks into bounded
chunk packs, for example 512MiB-2GiB stored size per pack. The index maps logical
chunks to offsets inside packs. Registry range pull can then fetch exact stored
segments when supported, while full-pack fallback still works.

## API shape

No new user knob in first version.

Internal status/evidence may expose:

- `storageLayout: Legacy|Chunked`
- `compression: none|mixed|zstd`
- `chunkSizeBytes`
- `storedBytes`
- `uncompressedBytes`
- `compressionRatio`
- `materializedBytes`
- `reservedBytes`

`Model` / `ClusterModel` spec should not get compression controls unless real
production evidence proves that cluster admins need policy overrides.

User/admin UX should mirror virtualization's `STOREDSIZE` / `UNPACKEDSIZE`
idea:

- `storedBytes`: how much backend storage the committed artifact occupies;
- `materializedBytes`: how much space a workload/node-cache needs after
  materialization;
- `reservedBytes`: conservative capacity reserved before publication so we do
  not discover "not enough space" after rollout already started.

These values are operational facts, not scheduling hints for `ai-inference`.

## Failure model

- Source range read fails: retry same chunk.
- Worker restarts: resume from checkpointed chunk.
- DMCR restarts: direct-upload recovers uploaded parts or restarts current pack.
- Materialize pod restarts: resume missing chunks from destination markers.
- Chunk digest mismatch: discard chunk and retry; repeated mismatch marks artifact
  as corrupt.
- Insufficient storage: reserve worst-case raw bytes before publication and
  fail before rollout.
- Registry without range pull: materialize downloads whole pack and extracts the
  needed chunks locally; performance degrades but correctness remains.
- Compression worker OOM or CPU throttling: chunk falls back to raw only before
  commit; committed chunk descriptors are immutable.

## Rollout strategy

1. Add chunked materialize reader first while publishers still emit legacy
   layout; tests prove backwards compatibility.
2. Add ranged blob reader and fallback metrics.
3. Add raw chunked publisher behind internal feature gate, default disabled.
4. Enable raw chunked for small canary models and collect ratio/resume metrics.
5. Add mixed compression only after bounded pack staging and cleanup are proven.
6. Enable for large `HF` object-source models with range support.
7. Enable for upload source only after raw-stage chunking is proven stable.

Rollback is simple: disable chunked publishing. Existing chunked artifacts stay
readable because materializer supports both legacy and chunked layouts.

## Implementation review checklist

- Large weights are never forced into one compressed tar stream.
- Resume is tested at source read, registry upload, manifest commit and
  materialize stages.
- Chunk index validation rejects path traversal, duplicate paths, overlapping
  chunk offsets, invalid pack offsets and unsupported compression.
- Logs never dump full raw registry output.
- Metrics expose useful counters without high-cardinality labels.
- Benchmarks compare:
  - legacy raw full pull;
  - chunked raw parallel pull;
  - chunked mixed zstd/raw pull;
  - restart after 25%, 50% and 90% progress.
