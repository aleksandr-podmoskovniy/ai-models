# Target Architecture

## Core contract

- Public publication contract: `ModelPack` stored in OCI registry.
- Public/runtime consumer input: only `OCI from registry`.
- Backend storage implementation under DVCR-style artifact plane stays hidden.
- Runtime always consumes only a local path prepared by a materializer/init
  step.

## Ownership split

### Our domain

- `Model` / `ClusterModel`
- publication lifecycle and public status
- `ModelPack` publication contract
- ai-inference-oriented resolved metadata calculation and projection
- runtime integration contracts for local materialization

### Reused backend/artifact plane

- registry/object-storage plumbing
- upload/auth/session patterns inspired by virtualization
- hidden backend storage implementation

### Replaceable implementation adapters

- `ModelPack` pack/push/inspect implementation
  - `KitOps`
  - `Modctl`
  - future module-owned native implementation

## Implementation target

### Must move to Go

- source worker runtime logic
- upload session HTTP serving path
- archive validation / unpack policy
- metadata calculation for publication result
- cleanup execution path for published artifacts

### May stay shell/build-time

- tool installation (`install-kitops.sh`-style) inside dedicated runtime image build
- upstream/backend packaging shell for phase-1 MLflow-adjacent runtime

## Alignment with virtualization

- controller orchestration in Go
- data-plane upload/import serving in Go
- K8s supplements/runtime lifecycles in Go
- shell remains build/install glue, not the core publication/runtime path

## Delete / Collapse / Keep

### Delete

- dead public API knobs without live semantics
  - `spec.publish`
- intermediate service-state buses between controller and one-shot runtimes
  - service `ConfigMap` protocol
- phase-2 Python runtime entrypoints in backend image

### Collapse

- `sourceworker` and `uploadsession` K8s lifecycle shells
  - keep only one shared controlled-resource pattern and one shared workload-pod
    pattern
- controller-owned source acquisition and ingest flow
  - keep one clear path:
    fetch or upload -> validate input format -> calculate metamodel ->
    build `ModelPack` -> push to registry
- controller binary and phase-2 runtime image layout
  - move toward a stricter split where manager wiring and data-plane execution
    no longer feel like one mixed executable concern

### Keep

- `Model` / `ClusterModel`
- `spec.source`
- `spec.inputFormat`
- `spec.runtimeHints`
- `ModelPack` as internal publication contract
- hidden DVCR-style backend artifact plane
- format-specific metamodel calculation as our real product logic
