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
  `port-forward`, and the upload runtime now only writes bytes into staging
  before a separate publish worker continues the flow.

Remaining drift:

- upload still flows through a controller-owned session Pod rather than direct
  multipart/presigned upload into staging object storage;
- there is still no resumable chunk/session protocol for interrupted large
  uploads.

Target replacement:

- controller-owned upload session with:
  - `Pod`
  - `Service`
  - `Ingress`
  - short-lived upload auth/session state
- user-facing `external` / `inCluster` upload URLs in status
- staging-first upload flow
- separate async publish worker after upload completion
- optional direct multipart/presigned staging path for very large uploads

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
  - there is still no materializer/runtime that would receive read-only
    projected DMCR access in its own namespace.

Target replacement:

- module-owned `dockerconfigjson` secret as the root credential source
- controller-owned copies/projections only into the namespaces that need them
- separate write vs read credentials for publisher/cleanup vs consumer runtime
- explicit CA bundle projection for internal clients
- cleanup of derived auth/trust objects after runtime completion

### Consumer delivery/materialization is still absent

Current drift:

- phase-2 publication currently stops at `ModelPack` stored in `DMCR`;
- there is still no live runtime that turns this artifact back into a local
  model path for `ai-inference`.

Target replacement:

- dedicated materializer/init runtime
- read-only DMCR auth/trust projection into the consumer namespace
- local expanded model path as the only consumer-facing runtime input
