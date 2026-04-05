# Notes

## Drift inventory before implementation

- Public API still exposed `status.artifact.kind=OCI|ObjectStorage` even though
  live publication path is already OCI-first.
- `images/controller/README.md` still described runtime materializer only as a
  future direction, while the release baseline now needs an explicit init-based
  runtime contract.
- The repo had a partially prepared `materialization` seam; the right move was
  to complete it coherently instead of introducing another parallel runtime
  contract.

## Slice 1 findings

### OCI-first cleanup

- `ModelArtifactLocationKind` was reduced to `OCI` only in the public API.
- publication snapshot validation now accepts only OCI artifacts.
- `api/README.md` no longer describes `status.artifact` as a generalized
  location abstraction for non-OCI steady-state publication.

### Runtime materialization seam

- `domain/materialization` remains the owner of runtime materialization
  contract validation and invariants.
- `application/materialization` now exposes an init-container-oriented bridge
  from public `status.artifact` to the runtime contract through
  `PlanInitContainer`.
- The runtime seam is explicitly virtualization-style: it always consumes an
  immutable OCI reference from registry; whatever backend storage sits under
  that registry stays outside public and runtime contracts.
- `ModelPack` is the artifact contract; concrete tools such as `KitOps`,
  `Modctl`, or a future module-owned implementation must stay behind adapters
  and must not leak into domain/public semantics.

### Runtime cleanup correction

- The speculative `ports/materialization` layer did not survive without a real
  consumer and was removed as dead patchwork.
- `modelpackinit` now depends directly on the bounded materialization contract
  instead of keeping a pass-through seam alive only for symmetry.

## Remaining gaps after this slice

- No consumer wiring into workload mutation/injection yet.
- No workload mutation / injection path for `ai-inference` yet.
- No runtime observation, leases, or cache lifecycle yet.

## Slice 2 findings

### Adapter-side init rendering

- `internal/modelpackinit` now owns the current v0 init-adapter rendering.
- `domain/materialization` and `application/materialization` remain
  adapter-agnostic and stay on the `ModelPack` / immutable OCI contract.
- The renderer turns the materialization contract into:
  - init-container envs for immutable ref, unpack path, and verification;
  - shared-volume mounts for unpack and runtime consumption;
  - supplemental secret volumes for OCI auth and Cosign verification.

### Current assumptions

- The current OCI auth projection assumes a standard docker-style registry
  secret exposing `.dockerconfigjson`.
- The current Cosign key projection assumes the key secret exposes
  `cosign.pub`.
- These assumptions are intentionally kept at the adapter layer only and must
  not be pulled back into domain/public contracts.
