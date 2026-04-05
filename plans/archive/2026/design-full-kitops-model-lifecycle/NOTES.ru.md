# Notes

## Upstream references

- KitOps init-container docs:
  `https://kitops.org/docs/integrations/k8s-init-container/`
- KitOps why/overview:
  `https://kitops.org/docs/why-kitops/`
- KitOps security overview:
  `https://kitops.org/docs/security/`
- Upstream init-container README:
  `https://raw.githubusercontent.com/kitops-ml/kitops/main/build/dockerfiles/init/README.md`
- Upstream init-container entrypoint:
  `https://raw.githubusercontent.com/kitops-ml/kitops/main/build/dockerfiles/init/entrypoint.sh`
- Upstream init-container Dockerfile:
  `https://raw.githubusercontent.com/kitops-ml/kitops/main/build/dockerfiles/init/Dockerfile`

## Relevant upstream init contract

For v0 design we treat these as upstream implementation details behind our
adapter boundary:

- `MODELKIT_REF`
- `UNPACK_PATH`
- `UNPACK_FILTER`
- `EXTRA_FLAGS`
- `COSIGN_KEY`
- `COSIGN_CERT_IDENTITY`
- `COSIGN_CERT_OIDC_ISSUER`

## What upstream init already solves

- bounded `OCI ModelKit -> local path` init-container flow
- optional Cosign verification
- unpack into shared volume / PVC
- simple runtime handoff before main container start

## What upstream init does not solve for us

- DKP auth projection for private OCI pulls
- runtime-class-specific policy around unpack filters
- delete leases and active-consumer guards
- public API shape for `Model` / `ClusterModel`
- long-term hardening, patching, distroless and rebase discipline

## ModelPack implementations

Official `modelpack.org` currently states that the specification has two
implementations:

- `Modctl` is the reference implementation;
- `KitOps` is the enterprise implementation.

This matters for our architecture:

- `ModelPack` is the open format and should stay the public artifact contract;
- `KitOps` is an implementation choice behind our packaging/runtime adapters,
  not the public platform contract;
- if we later need to switch implementation details, the public `Model` /
  `ClusterModel` UX should not change.

### Implementation implications

- `Modctl` matters as the smaller and more spec-centric implementation path.
- `KitOps` matters as the broader ecosystem/tooling path we already integrate
  with today.
- `model-csi-driver` matters as a signal that the surrounding ecosystem is also
  moving toward `ModelPack -> mounted local path`, not only toward direct
  runtime-side remote pulls.

## Popular model delivery patterns in current ecosystem

### 1. OCI artifact + runtime sidecar/init

KServe `modelcar` treats a model as an OCI artifact and exposes it to the
runtime from a dedicated helper container. This avoids pulling from source
systems inside the main runtime and reuses the node image cache.

Key properties:

- runtime points to an immutable `oci://` artifact;
- helper container handles the OCI fetch path;
- local cache effectiveness depends on immutable tags/digests;
- main runtime consumes a local path.

### 2. Controller-managed local cache

KServe `LocalModelCache` uses a controller plus node agent `DaemonSet` to
pre-download models into local NVMe-backed storage, then schedules workloads
against the warmed cache.

Key properties:

- canonical source stays remote;
- runtime uses node-local cached copies;
- cache lifecycle is a first-class control-plane concern;
- this is better for very large models than per-pod repeated downloads.

### 3. Runtime-local PVC or hostPath cache

Official `vLLM` Kubernetes docs explicitly show PVC-backed cache as an optional
pattern and mention `hostPath` or other storage options as alternatives.

Key properties:

- runtime can consume already-downloaded model bytes from local storage;
- PVC is a simple baseline;
- node-local or shared cache strategies can be added later.

### 4. CSI-style model mounting

`model-csi-driver` shows another ecosystem direction: publish the model as
`ModelPack`, then mount it into a workload through a storage-like control-plane
adapter instead of teaching every runtime how to fetch from OCI by itself.

Key properties:

- keeps the runtime contract as a local filesystem path;
- centralizes fetch/unpack logic outside the main serving container;
- fits the same long-term public contract we already want: immutable artifact
  ref plus local mounted path.

## Implications for ai-models

- Using `KitOps` for publication does not force us into one runtime-delivery
  path.
- There are at least two valid runtime patterns:
  - `kit init` / unpack into shared volume or PVC;
  - sidecar/agent/cache-oriented delivery that avoids repeated full copies.
- For very large models, a naive `registry copy + per-pod PVC copy` strategy is
  not the long-term target.
- A better target is:
  - canonical OCI artifact in the backend publication plane;
  - runtime cache plane with either shared RWX storage or node-local cache;
  - main runtime container isolated from source credentials and source fetch
    logic.

## Current repo snapshot

As of the current phase-2 worktree:

- public API is source-first:
  - `spec.source = HuggingFace | HTTP | Upload`;
  - published artifact is reflected in `status.artifact`;
  - technical profile is reflected in `status.resolved`;
- live publication path already exists for:
  - `HuggingFace`;
  - archive-based `HTTP`;
  - `Upload(HuggingFaceDirectory)`;
- publication target is a module-owned OCI registry contract wired through
  `publicationRegistry`;
- controller-owned auth projection exists for `HuggingFace` and `HTTP`;
- runtime delivery is still not production-ready:
  - no final `kitinit` or materializer integration with `ai-inference`;
  - no final cache-plane design implemented yet.
