# GPU-direct runtime delivery: что делаем и почему

## 1. Короткий ответ

В этом bundle мы не реализуем новый режим доставки в `ai-models`.

Мы фиксируем архитектурное решение:

- текущий production baseline остаётся filesystem-based:
  `SharedDirect` и `SharedPVC`;
- “грузить модель напрямую из registry/object storage в GPU memory” не является
  универсальной обязанностью `ai-models`;
- accelerator/direct loading должен появляться позже как runtime-specific
  optimization в `ai-inference`, поверх безопасных artifact read grants,
  chunk/range layout и node-cache write-through.

Проще:

```text
ai-models = catalog + immutable artifact + safe delivery/read contract
ai-inference = chooses runtime + starts server + loads model into CPU/GPU memory
```

Так мы не смешиваем Kubernetes delivery, registry storage и внутренности
`vLLM` / `TensorRT-LLM` / `llama.cpp` / Diffusers в одном контроллере.

## 2. Почему нельзя сделать “просто напрямую в GPU”

Контроллер Kubernetes и CSI driver не могут корректно “залить веса в VRAM”:

- GPU memory принадлежит процессу inference runtime, а не controller'у;
- у разных runtime разные loaders, flags, memory layout and cache behavior;
- GPUDirect Storage, RDMA, NVMe-oF and similar paths требуют поддержки на всех
  уровнях: hardware, kernel, driver, filesystem/object client and application;
- generic OCI/S3 URL сам по себе не является GPU-load ABI;
- если сделать это в `ai-models`, получится vendor/runtime-specific монолит и
  протекание абстракций в публичный model catalog.

Поэтому `ai-models` отдаёт безопасный, проверяемый source of truth: artifact by
digest. Конкретный runtime решает, как из этого artifact получить веса в память.

## 3. Production baseline

### `SharedDirect`

Режим для GPU/local-disk нод.

```text
DMCR ModelPack
  -> node-cache runtime downloads/verifies artifact
  -> node-local digest store
  -> ai-models CSI read-only mount
  -> workload reads /data/modelcache/models/<model-name>
```

Почему так:

- быстрый restart path: модель уже лежит на локальном диске ноды;
- workload видит обычный read-only filesystem path;
- cache layout, integrity markers and eviction принадлежат `ai-models`;
- inference runtime не зависит от нашего registry protocol.

Если runtime и node реально поддерживают GDS, он может ускорять чтение с этого
локального filesystem path. Это runtime/node capability, а не новый
`Model.spec`.

### `SharedPVC`

Режим для кластеров без local disks.

```text
DMCR ModelPack
  -> controller-owned materializer writes verified model files
  -> controller-owned RWX PVC
  -> workload read-only mounts /data/modelcache/models
  -> runtime reads /data/modelcache/models/<model-name>
```

Почему он нужен:

- это универсальный POSIX-filesystem path для runtime'ов, которые не умеют
  network/direct loader;
- RWX StorageClass закрывает “работает на любой ноде” без RWO-per-pod мусора;
- PVC lifecycle можно привязать к workload owner and model-set;
- не нужны long-lived DMCR credentials в workload namespace.

SharedPVC нельзя делать через старый fallback с копированием общего read Secret.
Нужен digest-scoped materializer grant и controller-owned materializer lifecycle.

### `Disabled/Blocked`

Если нет ни `nodeCache.enabled=true`, ни настроенного `sharedPVC.storageClassName`,
workload должен получить понятный blocked condition/reason.

Скрытой загрузки в namespace, `emptyDir`, RWO fallback или ручного PVC быть не
должно. Эти варианты плохо контролируются, ломают HA and quota story and
рождают неочевидные full-size copies.

## 4. Future optimization: `AcceleratedColdLoad`

Это будущий режим не для generic workload mutation, а для `ai-inference`.

Целевой path:

```text
ai-inference runtime adapter
  -> asks ai-models for digest-scoped artifact read grant
  -> reads chunks/ranges from DMCR/object source
  -> runtime-specific loader fills CPU/GPU memory
  -> optionally tees verified chunks into node-cache write-through lease
```

Важные правила:

- no raw registry/S3 credentials in workload namespace;
- grant scoped by digest, workload identity, service account, namespace,
  audience and short TTL;
- runtime chooses loader and fallback;
- cache hit wins: if node-cache already has digest, read local cache, not
  registry;
- cache miss may stream remote bytes and fill cache only with verified chunks;
- incomplete chunks are never visible as ready model files.

Почему это future:

- нужно сначала закончить chunked immutable `ModelPack` layout;
- нужен internal read-grant API;
- нужен runtime adapter at least for one runtime, например `vLLM`;
- нужна e2e-доказательная база: restart/resume, digest mismatch, node reboot,
  cache-hit/cache-miss, network throttling, metrics.

## 5. Кто за что отвечает

`ai-models` owns:

- `Model` / `ClusterModel` catalog and status;
- immutable `ModelPack` artifact in `DMCR`;
- source/upload publication;
- digest, size, metadata and layout;
- `SharedDirect` node-cache filesystem delivery;
- `SharedPVC` filesystem delivery;
- digest-scoped read grants for future runtime adapters;
- cache integrity, ready markers, GC and observability.

`ai-inference` owns:

- choosing `vLLM`, `Ollama`, `TensorRT-LLM`, Diffusers, Whisper etc.;
- deciding runtime parameters and scheduler placement;
- loading weights into CPU/GPU memory;
- using or not using GDS/RDMA/NVMe-oF/NIXL;
- falling back from direct loader to filesystem path;
- serving endpoint semantics: chat, embeddings, rerank, STT, TTS, CV, image or
  video generation.

`Model` must not say “use GDS” or “load via vLLM”. It describes artifact and
capabilities. Scheduler combines that with cluster/runtime facts.

## 6. Scheduler-facing model

`ai-models` should expose facts, not placement decisions:

- artifact digest and size;
- format/layout: Safetensors, GGUF, Diffusers, archive, raw layers;
- largest file, shard count, optional chunk index;
- endpoint/features from metadata;
- runtime compatibility hints where they are factual, not policy.

`ai-inference` scheduler decides using:

- model footprint;
- selected runtime support;
- GPU/MIG/MPS profile;
- node-cache hit/miss and free cache;
- RWX availability;
- node capabilities: local SSD, GDS, RDMA, NVMe-oF;
- rollout budget and expected cold-start cost.

## 7. Decision table

| Scenario | Decision |
| --- | --- |
| GPU node has local SDS cache | Use `SharedDirect` |
| No local disks, RWX StorageClass exists | Use `SharedPVC` |
| No local disks and no RWX | `Disabled/Blocked` |
| Runtime has proven direct loader and feature gate enabled | Future `AcceleratedColdLoad` |
| Runtime has no direct loader | Filesystem path only |
| GDS/RDMA not configured end-to-end | Normal filesystem/object read |
| Node-cache has digest | Read from cache, do not cold-load remotely |
| Cache miss and direct loader is enabled | Stream verified chunks and optionally warm cache |

## 8. Implementation roadmap

### Now

- Finish `SharedPVC` correctly:
  - controller-owned RWX PVC;
  - digest-scoped materializer grant;
  - materializer Job lifecycle;
  - ready markers;
  - cleanup/finalizer/GC;
  - no copied shared registry Secret.
- Keep `SharedDirect` as the fast path for local SDS nodes.

### Next

- Finish chunked immutable `ModelPack` layout:
  - chunk index;
  - range reads;
  - resume;
  - per-chunk digest validation;
  - parallel materialize with bounded concurrency.

### Later

- Add internal artifact read-grant service.
- Build one `ai-inference` runtime adapter POC, not a generic controller hack.
- Start with one runtime and one format, for example `vLLM` + Safetensors.
- Measure cold start, restart, cache hit, cache miss, network pressure and
  failure recovery.

## 9. Final decision

Do not remove `SharedPVC`.

Do not implement “controller/CSI writes to GPU memory”.

Do implement a clean two-layer architecture:

```text
ai-models:
  stable artifacts + filesystem delivery + safe read grants

ai-inference:
  runtime-specific loading + scheduling + accelerator integration
```

This is simpler operationally and cleaner architecturally: the platform has one
stable model artifact contract, while runtime-specific acceleration evolves
behind `ai-inference` feature gates and measurable production evidence.
