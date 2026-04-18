## MLflow surface inventory at reset start

На момент старта reset workstream `MLflow`/historical backend surface ещё живёт
в следующих местах:

### 1. Runtime shell

- `templates/backend/*`
- `images/backend/*`
- backend-related build and smoke targets in `Makefile`

### 2. Config and docs

- `openapi/config-values.yaml`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- historical mentions in `README*`, `docs/README*`, `docs/development/*`

### 3. Tooling and tests

- `tools/upload_hf_model.py`
- `tools/run_hf_import_job.sh`
- `tools/run_model_cleanup_job.sh`
- `tools/helm-tests/validate-renders.py`

### 4. Current reset stance

- `MLflow` is no longer treated as a live architectural center;
- slice 1 rewrites narrative and repo baseline around controller-owned
  publication/runtime flow;
- later slices must delete the actual runtime shell and stale render/test
  assumptions, not keep them as hidden fallback.

## Render-shell cut landed

В текущем execution slice уже удалены:

- `templates/backend/*`;
- `templates/auth/dex-client.yaml`;
- `templates/module/backend-*`;
- user-facing `auth` contract и backend-only `artifacts.pathPrefix`;
- second managed-postgres auth database;
- backend-oriented helm render checks.

## Backend-shell deletion landed

Следующим slice уже удалены:

- `images/backend/*`;
- backend build/smoke targets в `Makefile`;
- legacy import/cleanup tools, которые обслуживали historical backend shell.

Итоговая живая форма repo после этого cut:

- retired backend/auth shell больше не рендерится и не собирается;
- legacy import helpers больше не висят в repo как скрытый fallback;
- оставшийся reset scope теперь смещён с удаления shell на native publisher
  cutover и финальное сжатие residual docs/tooling.

## Validation-shell cleanup landed

После удаления backend shell вычищен и residual validation drift:

- `tools/kubeconform/kubeconform.sh` больше не знает про `DexClient` как про
  живой schema-skip;
- `tools/helm-tests/validate-renders.py` больше не использует `DexClient` как
  legacy render marker;
- render fixtures renamed away from `managed-sso-*` wording, потому что live
  сценарий здесь уже не про historical SSO/backend shell, а про generic module
  baseline и discovered Dex CA trust input.

## PostgreSQL shell deletion landed

PostgreSQL больше не рассматривается как часть live module contract:

- user-facing `aiModels.postgresql` removed from OpenAPI and module defaults;
- `templates/database/*` deleted;
- render fixtures and validation no longer exercise or tolerate `Postgres` /
  `PostgresClass`;
- docs and repo-layout guidance no longer describe managed-postgres as part of
  the ai-models baseline.

## Native publisher cutover contract

Следующий execution slice фиксируется как bounded replacement, а не как
финальная stream architecture:

- вход native publisher остаётся текущим `checkpointDir` из publication worker;
- `KitOps` binary и shell удаляются полностью;
- ai-models-owned publisher сам пишет registry blobs и manifest по live OCI
  contract, который уже читает `internal/adapters/modelpack/oci`;
- первый cut публикует один weight layer tar под contract path `model/`;
- materializer, inspect и runtime delivery shape не меняются в этом slice.

## Native publisher cutover landed

Текущий live publication path больше не зависит от `KitOps`:

- `internal/adapters/modelpack/oci` теперь владеет controller-side
  publish/remove плюс consumer-side inspect/materialize;
- publish path пишет registry blobs и OCI manifest напрямую по HTTP без
  external binary;
- первый cut публикует один weight-layer tar rooted at `model/`, что уже
  доказано round-trip тестом `Publish -> Materialize -> Remove`;
- worst-case byte path в landed cut всё ещё materialized, не streaming:
  `checkpointDir` plus один full tar рядом с ним на том же bounded worker
  volume/PVC, затем published blob в registry;
- `images/controller/werf.inc.yaml` больше не тащит отдельный publisher
  artifact stage, а `images/controller/kitops.lock` и
  `images/controller/install-kitops.sh` удалены;
- `internal/adapters/modelpack/kitops/*` удалён из live repo.

## Streaming publisher follow-up

Следующий bounded slice на том же native publisher path:

- не меняет OCI artifact contract и не трогает consumer-side materializer;
- заменяет temp tar file на streaming layer upload через OCI blob upload
  protocol;
- целевой worst-case local copy count:
  только `checkpointDir` на bounded worker volume/PVC плюс streamed network
  bytes, без второй full-size tar-копии на диске.

## Streaming publisher landed

Native publisher теперь stream'ит weight layer напрямую в registry upload flow:

- layer bytes идут `tar writer -> PATCH upload -> PUT finalize`;
- config blob и manifest по-прежнему остаются small in-memory payloads;
- local worst-case copy count сократился до одного full-size `checkpointDir`
  на bounded worker volume/PVC;
- end-to-end round-trip test теперь явно доказывает не только
  `Publish -> Materialize -> Remove`, но и сам streaming PATCH path.

## Pre-publish source artifact release follow-up

Следующий bounded slice после streaming publisher:

- не обещает полный source-to-registry stream, потому что profile/validation
  всё ещё требуют local `checkpointDir`;
- зато режет уже не publisher-only, а pre-publish byte path:
  к моменту старта `Publish()` временные source artifacts должны быть
  освобождены;
- целевой state:
  - `HF`: `.hf-snapshot` удалён после materialization/validation;
  - `upload staging`: локально скачанный upload object удалён после
    materialization/validation;
  - publish step видит только `checkpointDir`.

## Pre-publish source artifact release landed

Current live byte path перед стартом native publisher ужат ещё на один slice:

- `FetchRemoteModel(HuggingFace)` больше не несёт source-preparation tree
  alongside final publish root; historical `.hf-snapshot -> checkpointDir`
  handoff retained only as superseded context for the previous slice;
- `publishFromUpload` при `upload staging` удаляет локально скачанный source
  object сразу после materialization и до вызова `Publish()`;
- к моменту старта OCI upload локально остаётся только final model root
  (`HuggingFace`) или один materialized upload root; later archive-streaming
  slices narrow that upload path further;
- true source-to-registry streaming всё ещё не landed, потому что
  byte path всё ещё требует один локальный model root on the worker PVC.

## Selected non-checkpoint profile/materialization follow-up

Следующий bounded slice после pre-publish cleanup:

- не пытается сразу убрать `checkpointDir` для всех formats и sources;
- вместо этого режет только реально готовые paths:
  - `HF Safetensors`: profile resolution можно считать из remote summary
    (`config.json` + weight bytes);
  - direct uploaded `GGUF`: publisher уже умеет паковать single file, значит
    `checkpointDir` можно не создавать;
- archive uploads, direct safetensors uploads и generic HF publish materialization
  всё ещё остаются за следующим slice.

## Selected non-checkpoint profile/materialization landed

Current live state after this slice:

- `HF Safetensors` path now has a remote profile-summary path:
  - `config.json` fetched directly from source;
  - total `.safetensors` bytes summed via remote `HEAD`;
  - publish-worker can resolve profile from that summary instead of the local
    `checkpointDir`;
- this remote summary path is best-effort for now: on transport failure the
  worker still falls back to local model-root-based profile resolution instead
  of failing publication outright;
- direct uploaded `GGUF` file no longer goes through `PrepareModelInput` and
  `checkpointDir`; publish-worker resolves profile and publishes from the
  original file path directly;
- streamable archive uploads were the next real cut candidate and are now
  landed below; unsupported archive layouts still materialize a local
  `checkpointDir`.

## Hugging Face single-root publish landed

Current live state after the next slice:

- `FetchRemoteModel(HuggingFace)` no longer builds a second local
  `.hf-snapshot -> checkpointDir` chain;
- selected files are downloaded/materialized directly into one final local
  model root under the bounded worker workspace;
- validation now runs against that same model root in place;
- publish-worker streams OCI layer bytes from that same model root;
- remote `Safetensors` summary remains best-effort, but its fallback now lands
  on the same local model root instead of a second `checkpointDir`.

## Mirrored Hugging Face object-source publish landed

Current live state after the next slice:

- when source mirror is enabled and remote profile summary already exists,
  `FetchRemoteModel(HuggingFace)` no longer materializes `workspace/model`;
- selected source bytes still flow into durable source-mirror objects first,
  but publish-worker now hands native publisher one explicit mirror-backed
  object-source layer instead of `ModelDir=workspace/model`;
- native publisher reads those immutable mirror objects twice as needed:
  - once for deterministic layer descriptor precompute;
  - once for resumable OCI upload;
- no extra worker-local full-size copy is created for that mirrored `HF`
  publish path.

## Deterministic resumable OCI push landed

Current live state after the next slice:

- native publisher no longer depends on one-shot `PATCH` streaming with
  end-of-stream digest discovery;
- layer digest and size are precomputed deterministically from the local model
  root without creating a temp tar file;
- blob upload now uses:
  - `HEAD /v2/<name>/blobs/<digest>` for de-duplication;
  - `POST /blobs/uploads/` to start a session;
  - chunked `PATCH` with `Content-Range`;
  - `GET <location>` to recover current offset after transient failures;
  - `PUT ?digest=<digest>` to finalize;
- on lost upload session the worker restarts the session from offset `0`,
  keeping the same deterministic blob digest and still without any second local
  full-size copy;
- current runtime therefore prefers correctness and bounded retries over opaque
  persisted upload-URL state, which could be registry-specific or expiring.

## Upstream `ModelPack` contract check

Primary-source check against upstream `kitops-ml/kitops` confirms that current
ai-models-owned runtime matches the upstream `ModelPack` wire contract formerly
provided by `KitOps`:

- artifact type: `application/vnd.cncf.model.manifest.v1+json`;
- config media type: `application/vnd.cncf.model.config.v1+json`;
- supported layer base types:
  - `weight`
  - `weight.config`
  - `dataset`
  - `code`
  - `doc`
- supported wire formats/compressions:
  - `raw`
  - `tar`
  - `tar+gzip`
  - `tar+zstd`
- path annotation: `org.cncf.model.filepath`.

Current live worker call sites still default to one weight tar layer unless a
future slice chooses to publish auxiliary assets explicitly. That is now a
caller choice, not an adapter limitation: native publish/validate/materialize
path already supports the broader upstream layer/media-type matrix without
bringing `KitOps` back.

## Streaming upload archive path landed

Current live state after the next slice:

- `publishFromUpload` now inspects `tar` / `tar.gz` / `tgz` / `tar.zst` /
  `tar.zstd` / `tzst` / `zip` bundle structure directly from the archive
  stream instead of extracting to `checkpointDir` first;
- archive root normalization and benign-extra stripping happen from archive
  entry names:
  - singleton prefixes like `checkpoint/` are stripped;
  - files dropped by `modelformat.SelectRemoteFiles()` do not enter the OCI
    layer at all;
- `Safetensors` profile is resolved from archive summary:
  - `config.json` payload read from the archive;
  - total `.safetensors` bytes summed from archive metadata;
- archive-wrapped `GGUF` profile is now also resolved from archive summary:
  - selected `.gguf` entry name;
  - selected `.gguf` uncompressed size;
- publish-worker sends the original archive file to native publisher as one
  streamed `weight tar` / `weight tar+gzip` / `weight tar+zstd` layer with
  explicit archive-source metadata, tar/zip root normalization and
  selected-file filtering, not as `PrepareModelInput -> checkpointDir`;
- for staged upload this means one temporary downloaded archive remains locally
  until publish completes, then gets cleaned up; the extra extracted
  `checkpointDir` is gone for canonical streamable archive paths
  (`Safetensors` and `GGUF`), including zstd-compressed tar bundles.

## Direct Hugging Face object-source publish landed

Current live state after the next slice:

- direct `Hugging Face` `Safetensors` publish can now skip `workspace/model`
  too, not only the mirrored `HF` path;
- the direct remote object-source seam lives in `sourcefetch/*`, not in
  publish-worker:
  - resolved file URLs still come from the same `resolve/<revision>/<path>`
    contract;
  - selected file `size` and `etag` are captured there via `HEAD`;
  - authenticated `GET` read is also owned there;
- publish-worker now maps source-owned direct object-source metadata into the
  same native `oci` object-source layer contract already used for mirror
  objects;
- current fallback remains bounded and explicit:
  if direct object-source planning fails, `FetchRemoteModel` logs the reason
  and falls back to the existing one-local-copy `workspace/model` path instead
  of failing publication outright.

## Direct Hugging Face `GGUF` object-source publish landed

Current live state after the next slice:

- direct `HF GGUF` no longer drops out of the zero-local-copy happy path just
  because remote profile summary used to exist only for `Safetensors`;
- `sourcefetch` now resolves remote `GGUF` summary from immutable selected file
  metadata:
  - chosen `.gguf` relative path;
  - remote object size from `HEAD`;
- publish-worker reuses the same remote-summary seam and maps it into
  `gguf.ResolveSummary()` rather than reintroducing a format-specific local
  model root;
- direct `HF` happy path is now aligned across both canonical formats:
  `Safetensors` and `GGUF`;
  the old one-local-copy path remains only as bounded fallback when remote
  summary planning itself fails.

## Ranged object-source resume landed

Current live state after the next slice:

- object-source readers now have optional ranged-read capability instead of
  only full `OpenRead`;
- this does not change product surface:
  runtime/inference image still stays external (`Docker Hub`), while ai-models
  continues to own only model artifact publication/delivery bytes;
- native `oci` adapter now uses a range-aware tar stream for uncompressed
  object-source layers:
  - descriptor precompute still reads the full object source deterministically;
  - interrupted upload resume no longer has to re-read all earlier object bytes
    from offset `0`;
  - source mirror object reads and direct `HF` object reads can both resume by
    file-range when their reader exposes that capability.

## Direct staged `GGUF` object-source publish landed

Current live state after the next slice:

- upload staging is no longer just a temporary download source for direct
  `GGUF`;
- when publication source is a staged direct file and staging client exposes
  object reads, publish-worker now:
  - resolves direct `GGUF` from requested format, file name or lightweight
    magic read;
  - validates the first `GGUF` header bytes via staging read instead of local
    file materialization;
  - publishes one object-source layer directly from the staged object;
- this keeps upload-stage cleanup semantics unchanged after successful publish,
  but removes the redundant local downloaded copy from that happy path;
- repo narrative is now explicit that ai-models owns controller / artifact
  runtime images only, while the consumer serving image stays external, for
  example from `Docker Hub`.

## Staged tar-family archive streaming landed

Current live state after the next slice:

- staged tar-family archives (`tar`, `tar.gz`, `tgz`, `tar.zst`, `tar.zstd`,
  `tzst`) no longer get downloaded locally before archive inspection;
- `sourcefetch` archive inspection now supports reopenable tar-family readers,
  so the staged object can be read twice from object storage:
  - first pass for archive inventory / selected-file decision;
  - second pass for summary extraction when the chosen format needs it;
- native `oci` archive-source publish seam now accepts an optional reader
  directly on `PublishArchiveSource`, so the same tar-family staged object can
  be repacked into the canonical `weight tar*` layer without local archive
  copy;
- staged `zip` was intentionally left out of this slice until a proper ranged
  `ReaderAt` seam existed for remote zip inspection/publish.

## Staged `zip` archive streaming landed

Current live state after the next slice:

- staged `zip` archives no longer fall back to local download before
  inspection/publish;
- `sourcefetch` now supports zip inspection through a real ranged `ReaderAt`
  seam over object storage:
  - `zip.NewReader(...)` runs over staged object range reads;
  - format selection and summary extraction still use the same selected-file
    rules as local zip inspection;
- native `oci` archive-source publish now supports remote `zip` readers too,
  with explicit archive size on `PublishArchiveSource`;
- staged archive streaming is now aligned across canonical archive shapes:
  - tar-family bundles use reopenable streaming reads;
  - zip bundles use ranged random-access reads;
- the old staged archive local-download tail is now gone for canonical archive
  publish paths, not only for tar-family bundles.

## Upload fail-fast probe landed before workspace preparation

Current live state after the next slice:

- publish-worker now reuses domain upload-probe semantics before workspace
  preparation instead of waiting for local materialization/validation to reject
  already-invalid inputs;
- the new preflight stays bounded:
  - it does not change valid direct/archive happy paths;
  - it only removes pointless local copy/error paths;
- for local uploads, invalid direct payloads such as bare `.safetensors` fail
  before workspace creation;
- for staged uploads with object reads, the same invalid direct payloads fail
  before any local download from object storage;
- this keeps source-of-truth and validation semantics in one place:
  `ingestadmission` still owns the cheap file-shape contract, while
  publish-worker only reuses it as fail-fast shell.

## Local zero-copy uploads no longer create eager workspaces

Current live state after the next slice:

- publish-worker no longer creates `ensureWorkspace(...)` before every local
  upload path by default;
- after the fail-fast probe, local zero-copy happy paths now run first:
  - direct `GGUF` file publish;
  - local streamable archive publish;
- only if those paths do not handle the input does the worker allocate
  `workspace` and continue to local preparation/materialization;
- this keeps the actual byte path unchanged, but removes empty temp
  directories and misleading `SnapshotDir` churn from zero-copy local uploads.

## Hugging Face workspace allocation is now lazy

Current live state after the next slice:

- `publishworker` no longer allocates `ensureWorkspace(...)` before every
  `HuggingFace` fetch attempt;
- the first `HF` fetch attempt now runs with empty workspace whenever
  streaming publish is supported;
- `sourcefetch` returns an explicit `ErrRemoteWorkspaceRequired` only when the
  path really needs local model-root materialization;
- only then does publish-worker allocate `SnapshotDir` and retry;
- successful direct or mirrored `HF` streaming happy paths now leave no empty
  temp workspace behind.

## Local upload materialized fallback is now gone

Current live state after the next slice:

- after fail-fast validation, local upload inputs now have exactly one allowed
  outcome:
  - direct `GGUF` zero-copy publish;
  - streamable archive zero-copy publish;
  - or explicit failure;
- publish-worker no longer keeps a silent local fallback into
  `publishMaterializedUpload(...)` for local uploads;
- materialized fallback remains only on staged paths, where bounded local
  preparation can still be needed when object-read contracts are not enough.

## Staged upload materialized fallback is now gone too

Current live state after the next slice:

- upload admission already narrows valid upload shapes to:
  - direct `GGUF`;
  - archive bundle;
- staged object-read happy paths already publish those shapes without local
  checkpoint materialization;
- therefore `publishMaterializedUpload(...)` was dead code rather than a real
  successful fallback and has been removed from the upload publish shell.

## Staged download-only upload fallback is now gone too

Current live state after the next slice:

- live publication wiring already passes the S3 staging adapter, which exposes
  `Reader` and `RangeReader`;
- staged upload publication no longer keeps a `Download`-only fallback for
  clients that cannot expose object reads;
- download-only staged clients now fail before:
  - local download;
  - workspace creation;
  - publisher invocation;
- current live staged upload contract is now strict:
  object-read streaming publish only.

## Source-mirror state store no longer downloads JSON into temp files

Current live state after the next slice:

- source-mirror `manifest.json` and `state.json` are now decoded directly from
  object-storage `OpenRead`, without a temporary local JSON file;
- this removes temp-file shell from the mirror control-state path without
  changing any model byte path;
- the objectstore adapter no longer depends on `Downloader` for state loads,
  only on `Reader`;
- this keeps the remaining `Download` dependency only on true file
  materialization paths, not on tiny control-state JSON.

## `HF` publish no longer keeps any local materialization fallback

Current live state after the next slice:

- `FetchRemoteModel(HuggingFace)` no longer carries a local `workspace/model`
  fallback for either direct or mirrored publish;
- remote profile summary resolution is now mandatory for the live `HF`
  publication path:
  if summary planning fails, publish fails explicitly instead of degrading to
  local download/materialization;
- direct object-source planning failure is now also terminal:
  the worker no longer retries with a local workspace after `HEAD`/planning
  errors;
- mirrored `HF` path still transfers bytes into source mirror object storage,
  but publication now always continues from mirrored objects instead of
  materializing them back into a local model root;
- `ErrRemoteWorkspaceRequired` and the publish-worker retry shell around it are
  gone;
- publish-worker staging contract is now narrowed to the streaming-capable
  surface that live paths already require:
  `MultipartStager + Reader + Uploader + Remover + PrefixRemover`;
- `publish-worker` CLI no longer exposes `--snapshot-dir`, and source-worker
  no longer wires it as a runtime arg.
