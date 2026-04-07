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

### Incomplete resolved metadata

Current drift:

- API and internal snapshot already reserve richer resolved profile fields;
  current publication path calculates only a narrow subset and controller
  projects only that subset.

Replacement landed as baseline:

- metadata extraction in `internal/adapters/modelprofile/*`
- projection in `internal/domain/publishstate/conditions.go`
