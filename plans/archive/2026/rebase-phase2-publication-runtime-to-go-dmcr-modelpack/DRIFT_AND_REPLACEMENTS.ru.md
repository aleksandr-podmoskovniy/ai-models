# Drift And Replacements

## Current drift

### Python publication worker

Was:

- `images/backend/scripts/ai-models-backend-source-publish.py`

Problem:

- owns too many concerns at once and makes phase-2 publication backend-script
  centric instead of controller/data-plane centric.

Replacement landed:

- `internal/adapters/sourcefetch/*`
- `internal/adapters/modelformat/*`
- `internal/adapters/modelprofile/safetensors/*`
- `internal/adapters/modelprofile/gguf/*`
- `internal/ports/modelpack/contract.go`
- `internal/adapters/modelpack/kitops/adapter.go`
- `internal/dataplane/publishworker/run.go`
- `cmd/ai-models-controller/publish_worker.go`

### Python upload session runtime

Was:

- `images/backend/scripts/ai-models-backend-upload-session.py`

Problem:

- upload serving path for phase-2 publication is in Python, unlike the
  virtualization-style Go uploader/runtime.

Replacement landed:

- `internal/dataplane/uploadsession/run.go`
- `cmd/ai-models-controller/upload_session.go`

Remaining drift:

- runtime moved to Go, the live edge now exposes session URLs instead of
  `port-forward`, and the upload runtime is now only a session/control plane:
  clients upload parts directly into object-storage staging through
  presigned multipart URLs and a separate publish worker continues the flow.

Remaining drift:

- PVC-specific uploader/staging path is still not landed;
- resumability exists only through the current multipart session API and secret
  state, not through a richer standalone client protocol such as `tus`.

Target replacement:

- controller-owned upload session with:
  - shared upload gateway
  - `Service`
  - `Ingress`
  - short-lived upload auth/session state
- user-facing `external` / `inCluster` upload URLs in status
- staging-first upload flow
- separate async publish worker after upload completion
- direct multipart/presigned staging path for very large uploads in
  object-storage mode
- dedicated uploader/staging path when a future PVC-specific upload backend
  is actually needed

### Hard-coded `KitOps` path

Was:

- `images/backend/scripts/ai-models-backend-source-publish.py`
- `images/backend/scripts/install-kitops.sh`
- `images/backend/kitops.lock`

Problem:

- concrete tool is pinned and executable, but not yet behind a proper
  replaceable `ModelPack` adapter contract.

Replacement landed:

- `images/controller/kitops.lock`
- `images/controller/install-kitops.sh`
- `internal/ports/modelpack/contract.go`
- `internal/adapters/modelpack/kitops/adapter.go`

Remaining drift:

- `KitOps` is still a concrete CLI runtime dependency, even though it now lives
  behind a proper port and packaging seam.

Target replacement:

- native Go `ModelPack` publication/remove implementation behind the same
  `internal/ports/modelpack` contract

### Incomplete resolved metadata

Current drift:

- API and internal snapshot already reserve richer resolved profile fields;
  current publication path calculates only a narrow subset and controller
  projects only that subset.

Replacement landed as baseline:

- metadata extraction in `internal/adapters/modelprofile/*`
- projection in `internal/domain/publishstate/conditions.go`

### DMCR auth/trust discipline is still incomplete only on the consumer side

Current drift:

- publication-side controller/runtime wiring now already uses:
  - split write vs read root credentials
  - controller-owned projected write auth into worker/session/cleanup runtime
  - explicit CA projection and explicit cleanup of derived auth/trust objects;
- the remaining honest gap is the consumer side:
  - a standalone materializer runtime now exists, but there is still no
    consumer-side projection/wiring that would hand it read-only projected
    DMCR access in its own namespace.

Target replacement:

- module-owned `dockerconfigjson` secret as the root credential source
- controller-owned copies/projections only into the namespaces that need them
- separate write vs read credentials for publisher/cleanup vs consumer runtime
- explicit CA bundle projection for internal clients
- cleanup of derived auth/trust objects after runtime completion

### Consumer delivery/materialization is still absent

Current drift:

- phase-2 publication no longer stops strictly at `ModelPack` stored in
  `DMCR`: a standalone runtime command already materializes immutable OCI
  artifacts into a local path;
- there is still no live consumer-side wiring that turns this into a normal
  `ai-inference` runtime dependency.

Target replacement:

- dedicated materializer/init runtime
- read-only DMCR auth/trust projection into the consumer namespace
- local expanded model path as the only consumer-facing runtime input

### Phase-2 should stay independent from MLflow backend lifecycle

Current drift:

- some design notes tried to re-introduce `MLflow` runs/workspaces/registry
  into the phase-2 publish path;
- that would create a second lifecycle/state machine on top of `CRD + DMCR`;
- current live Go path is cleaner precisely because it publishes directly to
  `DMCR` without mandatory `MLflow` coupling.

Target replacement:

- controller-owned raw object URI allocation and deterministic storage layout;
- `CRD.status` remains the only platform truth for final publication state;
- optional future audit/log export stays append-only and non-authoritative;
- `MLflow` may remain only a phase-1/backend concern and must not become a
  required phase-2 publish dependency.
