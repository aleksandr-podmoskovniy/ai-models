# CONTRACT: ModelPack chunked v1

Этот файл фиксирует implementation-ready контракт для будущего slice. Это не
публичный CRD API.

## 1. Layout

Artifact остаётся OCI manifest:

```text
config: application/vnd.deckhouse.ai-models.modelpack.config.v1+json
layers:
  optional small-files.tar+zstd
  chunk-index.json
  chunk-pack-000
  chunk-pack-001
```

Legacy layout остаётся валидным: `raw` и `tar` layers с `model.filepath`.

Chunked layout отличается:

- index layer содержит logical files и reconstruction plan;
- pack layers opaque, без `org.cncf.model.filepath`;
- manifest не содержит one-layer-per-chunk;
- materializer выбирает legacy или chunked extractor по media types.

## 2. Media Types

Draft internal media types:

```text
application/vnd.deckhouse.ai-models.modelpack.chunk-index.v1+json
application/vnd.deckhouse.ai-models.modelpack.chunk-pack.v1
application/vnd.deckhouse.ai-models.modelpack.chunk-pack.zstd.v1
```

Rules:

- `chunk-index` is small JSON and SHOULD be zstd-compressed only if the OCI
  adapter already supports compressed JSON blob validation.
- `chunk-pack.v1` may contain raw and per-chunk zstd payloads; exact codec is in
  index per chunk.
- `chunk-pack.zstd.v1` is reserved for future whole-pack seekable zstd. Do not
  use it in v1 unless range seek table is implemented.

## 3. Index Schema

Minimal JSON:

```json
{
  "schemaVersion": "modelpack.chunked.v1",
  "createdBy": "ai-models-controller",
  "chunkSizeBytes": 134217728,
  "files": [
    {
      "path": "model-00001-of-00004.safetensors",
      "sizeBytes": 3900000000,
      "digest": "sha256:...",
      "source": {
        "kind": "object",
        "uri": "redacted-or-internal",
        "etag": "\"...\"",
        "generation": "optional",
        "sizeBytes": 3900000000
      },
      "chunks": [
        {
          "index": 0,
          "offset": 0,
          "uncompressedSizeBytes": 134217728,
          "storedSizeBytes": 104857600,
          "storedDigest": "sha256:...",
          "compression": "zstd",
          "pack": "chunk-pack-000",
          "packOffset": 0,
          "packLength": 104857600
        }
      ]
    }
  ],
  "packs": [
    {
      "id": "chunk-pack-000",
      "digest": "sha256:...",
      "sizeBytes": 1073741824,
      "mediaType": "application/vnd.deckhouse.ai-models.modelpack.chunk-pack.v1"
    }
  ]
}
```

## 4. Validation

Reject index if:

- path is empty, absolute, contains `..`, backslash traversal or duplicate path;
- file chunks do not cover `[0, sizeBytes)` exactly;
- chunk offsets overlap, have gaps or are not ordered by `index`;
- chunk `packOffset + packLength` exceeds pack `sizeBytes`;
- `storedSizeBytes != packLength`;
- unsupported `compression`;
- duplicate pack ids or missing pack referenced by chunk;
- digest fields are empty or invalid;
- file digest mismatches after reconstruction.

Pack layer digest still comes from OCI descriptor. Chunk `storedDigest` verifies
the byte slice extracted from a pack before decompression/write.

## 5. Bin-Pack Policy

Initial policy:

- large file threshold: 256MiB;
- logical chunk size: 64-128MiB;
- target pack size: 512MiB-1GiB;
- hard max pack size: 2GiB;
- chunks are sorted by `(file path, offset)`;
- append chunks until target size; never exceed hard max;
- seal pack, persist descriptor, then upload.

Raw chunked comes first. Mixed compression later.

For mixed compression:

- compress per chunk, not whole file;
- use zstd level 1-3;
- keep raw if saving is below threshold, for example 8-10%;
- abort compression and store raw if memory/time budget is exceeded;
- stage current pack to bounded durable temp storage before direct-upload,
  because compressed pack size is unknown until sealed.

## 6. Checkpoint Shape

Do not persist one verbose JSON record per chunk.

Checkpoint v2 stores:

```json
{
  "schemaVersion": "modelpack.publish.v2",
  "layoutPlanHash": "sha256:...",
  "sourceEvidenceHash": "sha256:...",
  "phase": "Planning|Packing|UploadingPack|CommittingManifest|Completed",
  "completedChunkRanges": {
    "model-00001-of-00004.safetensors": "0-31,32-63"
  },
  "committedPacks": [
    {"id":"chunk-pack-000","digest":"sha256:...","sizeBytes":1073741824}
  ],
  "currentPack": {
    "id": "chunk-pack-001",
    "stagingHandle": "s3://module-private/tmp/...",
    "completedChunkRanges": "64-70"
  }
}
```

Budget:

- target checkpoint payload below 256KiB;
- hard fail before 768KiB to stay below Kubernetes object limits;
- save after sealed pack and bounded progress intervals, not every small read.

## 7. Range Pull Contract

Registry blob reader must handle:

- `206 Partial Content`: use returned exact bytes;
- `200 OK` after Range request: treat as range unsupported, full-pack fallback;
- `416`: re-check index/descriptor, then fail corrupt/stale if range is still
  expected;
- short body: retry, then fail after bounded attempts;
- digest mismatch: discard chunk and retry.

Metrics/log fields:

- `range_supported`
- `full_pack_fallback`
- `pack_digest`
- `pack_offset`
- `pack_length`
- `chunk_index`
- `duration_ms`

## 8. Materialize State

Destination layout:

```text
<destination>/.ai-models-tmp/<digest>/
  files/<logical-path>.part
  markers/<logical-path>.chunks
  index.json
<destination>/model/
```

Rules:

- partial files never appear under final `model/`;
- chunk marker is written only after digest verification and successful offset
  write;
- file digest is checked before final rename;
- artifact marker is written only after all files are complete;
- node-cache scanner treats `.ai-models-tmp` as in-progress, not ready usage.

## 9. Rollout Gates

Implementation order:

1. Read-only validation for chunked manifests.
2. Chunked materializer with synthetic test artifact.
3. Ranged blob reader and fallback metrics.
4. Raw chunked publisher, feature-gated off.
5. Canary on small model.
6. Large HF model with worker/DMCR/controller restarts.
7. Mixed compression only after raw chunked is stable.

Do not enable by default until e2e proves restart at 25%, 50% and 90% publish
and materialize progress.
