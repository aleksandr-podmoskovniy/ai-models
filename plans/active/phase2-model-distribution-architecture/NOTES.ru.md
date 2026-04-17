## Detailed interpretation of the requested target state

### 1. Canonical publication stays inside ai-models

The user intent is not to expose external registries as the primary publication
contract.

Target interpretation:

- ai-models keeps one canonical published artifact contract;
- current `KitOps`-based publisher is only an implementation detail and may be
  replaced;
- the long-term target is a controller-owned `ModelPack` publisher
  implemented by ai-models itself, without dependency on external packaging
  brands in the runtime contract;
- the published output still remains OCI-by-digest and stays the source for
  later runtime distribution.

Practical consequence:

- "streaming to OCI" means building our own stream-capable publisher adapter,
  not wrapping `KitOps` in more temp-dir logic;
- current filesystem-based `KitOps` path should be treated as transitional.

### 2. DMZ registry is a distribution tier, not a source-ingest tier

Requested scenario:

- one cluster inside the protected contour publishes model artifacts;
- a cluster in `DMZ` serves as the pull-facing registry for less trusted or
  externally separated runtimes;
- internal runtimes pull from the `DMZ` registry when topology or policy
  requires it.

Target interpretation:

- canonical source of truth remains internal publication state by digest;
- `DMZ` registry is an additional distribution tier:
  - mirror
  - promotion target
  - or bounded proxy cache;
- this must not leak into `Model.spec.source`, because it is not user source
  intent;
- topology, lag and trust for this tier should be module/runtime config plus
  status/metrics, not new public model spec knobs by default.

### 3. Runtime delivery target is node-local shared cache, not per-pod materialization

Requested scenario:

- no init container per workload;
- one node-local cache on local disk;
- many pods on the same node can reuse the same model;
- if the model is idle for some time, the cache may evict it;
- distributed inference should converge to the same per-node behavior across
  all nodes participating in a workload.

Target interpretation:

- current init-container path becomes fallback/baseline;
- long-term runtime delivery should move to a node-scoped cache service with a
  mount contract into workloads;
- this is closer to a CSI/node-plugin-like model than to current
  `PodTemplateSpec` init-only mutation;
- the cache must be digest-addressed and registry-backed;
- cache management becomes its own control plane:
  - fetch
  - refcount/use tracking
  - eviction
  - integrity
  - mount fan-out.

### 4. Role of sds-node-configurator

Requested relation:

- use local disks on nodes;
- size the node cache from module config;
- rely on existing Deckhouse storage machinery where reasonable.

Target interpretation:

- `sds-node-configurator` is a storage substrate provider:
  - `LocalStorageClass`
  - node-bound local volumes
  - local disk provisioning primitives;
- it is not the cache manager itself;
- ai-models still needs a separate node runtime that understands:
  - artifact digests
  - model cache layout
  - registry pulls
  - mount lifecycle
  - eviction policy.

This boundary must stay explicit to avoid storage-specific API leakage.

### 5. FUSE/overlay idea: interpreted as "runtime-visible lazy files"

User idea:

- files appear in the workload immediately;
- runtime such as `vLLM` starts reading them while bytes are still arriving
  into cache;
- ideally bytes flow to both cache and GPU warm-up path on first start.

Target interpretation:

- this is a different level of sophistication than "node cache with mounts";
- it implies lazy file semantics and partial availability visibility to the
  runtime;
- likely implementation families:
  - FUSE-backed lazy file system;
  - user-space mount service exposing placeholder files and demand fetch;
  - runtime-specific prefetch loader.

Current engineering caution:

- generic runtimes typically expect stable model files, not partially materialized
  files with evolving contents;
- exposing incompletely materialized files to generic loaders is likely unsafe
  unless the runtime is explicitly integrated with the lazy loader semantics;
- therefore a generic FUSE layer should be treated as an advanced optional path,
  not the first implementation slice.

### 6. "Load to VRAM while cache is still filling" interpreted in two variants

Variant A:

- runtime reads lazily exposed files through a mount;
- loader begins parsing/reading model segments immediately;
- missing ranges are fetched on demand and simultaneously persisted into node
  cache.

Variant B:

- on first launch, a specialized runtime-side preloader pulls OCI layers from
  registry;
- decodes them back into model files or tensors;
- feeds tensors toward GPU memory while also writing the same bytes to the
  node-local cache.

Interpretation and likely priority:

- Variant A is more generic but much riskier, because it relies on existing
  runtimes tolerating lazy/incomplete file realization;
- Variant B is more defendable but runtime-specific and no longer a generic
  filesystem-only contract;
- both are significantly beyond the current init-container model and should be
  treated as later-stage optimization after the node-local cache service exists.

### 7. Recommended priority order

1. Replace `KitOps` with ai-models-owned publisher implementation.
2. Add bounded `DMZ` distribution tier above canonical internal OCI artifact.
3. Build node-local cache delivery with explicit mount contract and fallback to
   current init-container path.
4. Only then evaluate advanced lazy-delivery options:
   - FUSE lazy files
   - runtime-specific first-run VRAM preloading
   - simultaneous cache-fill and runtime warm-up.

### 8. Questions to keep explicit in later design slices

- Is node cache digest-global across all models or partitioned by family/runtime?
- Does workload scheduling need awareness of cache heat on nodes?
- Is `DMZ` registry pull-through cache enough, or do we need deterministic
  promotion/replication jobs?
- Does the new ai-models-owned publisher emit exactly the same OCI contract as
  current `ModelPack`, or do we need a v2 artifact layout?
- Which runtime families, if any, are allowed to participate in lazy/FUSE or
  VRAM-first loading paths?
