# Canonical Multi-Source Ingest

## Зачем нужен этот note

Нужно выровнять source ingest path для `HTTP`, `HuggingFace`, `Upload` и
будущих источников так, чтобы:

- public API оставался source-agnostic;
- planner получал только scheduler-facing metadata;
- module runtime не зависел от одного source ecosystem как platform truth;
- future adapters (`HF`, `OCI`, возможно `Ollama` import) встраивались без
  переделки всего publication pipeline.

## Что требуют наши ADR

Исходный ADR по каталогу моделей в
`/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
требует:

- пользователь описывает источник модели, а не внутреннюю механику хранения;
- controller принимает модель из поддержанного источника и публикует свой
  managed artifact;
- рассчитанный технический профиль пишется в `status.resolved`;
- downstream consumers используют published artifact и resolved profile, а не
  raw source details;
- source-specific metadata не должна размывать platform contract.

ADR по inference service в
`/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-inference-service.md`
ожидает от модели planner-facing metadata:

- endpoint compatibility;
- runtime compatibility;
- accelerator vendor compatibility;
- precision compatibility;
- minimum launch requirements.

## Official reference practices

### Hugging Face Hub

Official docs:

- https://huggingface.co/docs/huggingface_hub/guides/download

Практика:

- canonical HF downloader — `hf_hub_download()` / `snapshot_download()`;
- snapshot semantics привязана к `revision` / full commit hash;
- download path version-aware and cache-aware;
- `snapshot_download()` умеет `allow_patterns` / `ignore_patterns`;
- local folder mode сохраняет original file structure и metadata cache.

Вывод:

- для `HuggingFace` правильнее использовать source-native snapshot downloader,
  а не ad-hoc loop `GET file -> write file`;
- для нашей платформы особенно важен `revision` pinning и filtered download по
  allowed file patterns.

### vLLM

Official docs:

- https://docs.vllm.ai/en/latest/models/supported_models/

Практика:

- by default vLLM loads models from HF Hub;
- runtime support проверяется по `config.json` / architecture;
- vLLM опирается на HF cache/tooling rather than inventing a parallel
  downloader contract.

Вывод:

- сильный runtime consumer читает scheduler-facing model characteristics, но
  не требует, чтобы platform CRD зеркалил весь HF repo card;
- source-native acquisition and local materialization are normal practice.

### Ollama

Official docs:

- https://docs.ollama.com/modelfile
- https://docs.ollama.com/api/pull

Практика:

- `ollama pull` работает по `model name`;
- `Modelfile` uses `FROM <model name>:<tag>` or local `FROM <model directory>`
  / `FROM ./model.gguf`;
- это distribution/runtime ecosystem со своей model registry semantics, а не
  generic importer contract для arbitrary sources.

Вывод:

- `Ollama` не должен определять наш общий ingest contract;
- если поддержка появится, её надо делать как отдельный source adapter или
  delivery target, а не как основу для всех источников.

### KServe

Official docs:

- https://kserve.github.io/website/docs/model-serving/storage/overview
- https://kserve.github.io/website/docs/model-serving/storage/storage-containers
- https://kserve.github.io/website/docs/model-serving/storage/providers/oci

Практика:

- один serving API принимает разные `storageUri` providers;
- download/materialization выполняет storage initializer;
- для custom URI formats используется отдельный storage adapter container;
- для immutable delivery of large models KServe вводит `Modelcars` over OCI.

Вывод:

- правильный общий pattern: source-specific acquisition adapter behind one
  stable platform contract;
- source ingress и final delivery artifact — разные уровни решения;
- immutable OCI delivery path имеет смысл как published truth, а не как ingest
  truth.

### BentoML

Official docs:

- https://docs.bentoml.com/en/latest/build-with-bentoml/model-loading-and-management.html

Практика:

- local/internal `Model Store` acts as canonical managed representation;
- HF source can be loaded through a dedicated adapter;
- runtime consumers work with a local path or model-store tag, not with raw
  HF/HTTP source semantics;
- models can be exported/imported across storage backends after normalization.

Вывод:

- сильные решения разделяют:
  - source acquisition;
  - internal canonical stored representation;
  - runtime delivery/materialization.

## Canonical decision for ai-models

Нужен один canonical ingest path:

1. `spec.source` stays source-agnostic:
   - `source.url`
   - `source.upload`
   - future sources are resolved internally, not through provider trees in the
     public API.
2. Source acquisition uses a source-native adapter:
   - `HuggingFace` -> native snapshot semantics with revision pinning and file
     filtering;
   - `HTTP` -> HTTP fetch with archive/single-file normalization;
   - `Upload` -> shared upload gateway + multipart raw staging;
   - future custom schemes -> dedicated internal adapter, not public API
     inflation.
3. Source bytes first become controller-owned raw staging:
   - canonical `raw/...` object layout;
   - persisted source provenance sufficient for retry, cleanup and audit.
4. Publish worker creates one normalized local checkpoint workspace:
   - same directory semantics regardless of the original source;
   - archive unpack / single-file materialization / snapshot directory all end
     in one normalized checkpoint tree.
5. Technical profile is calculated from actual bytes in that normalized
   checkpoint, not from source marketing metadata.
6. Published truth is internal `DMCR`, not HF cache, not Ollama registry, not
   raw object storage.
7. Public `CRD.status` exposes only platform-facing truth:
   - published artifact reference;
   - scheduler-facing resolved profile;
   - stable lifecycle conditions.

## What belongs in public status

Planner-facing contract:

- `status.resolved.task`
- `status.resolved.format`
- `status.resolved.parameterCount`
- `status.resolved.quantization`
- `status.resolved.contextWindowTokens`
- `status.resolved.supportedEndpointTypes`
- `status.resolved.compatibleRuntimes`
- `status.resolved.compatibleAcceleratorVendors`
- `status.resolved.compatiblePrecisions`
- `status.resolved.minimumLaunch`

Useful provenance, still acceptable in public status:

- `status.source.resolvedType`
- `status.source.resolvedRevision`
- `status.resolved.sourceRepoID`
- `status.resolved.framework` as a normalized profile label, not a mirror of
  source-specific provider metadata
- `status.resolved.license`
- `status.resolved.family`
- `status.resolved.architecture`

## What must stay out of public status

Do not mirror source ecosystems in public CRD:

- HF `downloads`
- HF `likes`
- raw HF `tags`
- `private/gated`
- full file manifest
- source-card blobs
- Ollama registry/layer metadata
- downloader/cache implementation details

These may exist only as internal provenance/audit state if they are needed at
all.

## Exact architectural implication

The next correct implementation slice is not "support more source kinds in
spec". It is:

- replace the current ad-hoc Hugging Face file loop with a source-native
  snapshot adapter implemented in Go over the official HTTP/API semantics,
  not through a bundled Python/CLI runtime;
- keep the rest of the publication pipeline unchanged:
  raw stage -> normalized checkpoint -> profile -> DMCR publish -> public
  status projection.

This keeps one stable module contract while improving correctness for a source
that matters most right now.
