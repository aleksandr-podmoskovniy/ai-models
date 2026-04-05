# REVIEW

## Slice 1 result

Закрыт first corrective slice для release baseline:

- public API и generated CRD больше не рекламируют old `ObjectStorage`
  artifact kind;
- materialization seam сведён к одному согласованному split:
  - `internal/domain/materialization`
  - `internal/application/materialization`
- speculative `internal/ports/materialization` did not survive without a real
  adapter consumer and was removed once proven unused;
- добавлен application facade, который строит init-container materialization
  contract напрямую из public `status.artifact`;
- release baseline явно закрепляет virtualization-style invariant: runtime
  работает только с immutable OCI ref from registry и не различает backend
  storage под registry plane;
- `ModelPack` contract и текущие `KitOps`-based tools разведены: конкретная
  implementation brand больше не считается частью public/runtime architecture.

## Slice 2 result

Закрыт следующий bounded runtime slice:

- landed adapter-side init rendering in `internal/modelpackinit`;
- adapter-specific envs, mounts, and secret volumes are now kept out of
  `domain/materialization` and `application/materialization`;
- current runtime path is clearer:
  - contract planning in domain/application;
  - concrete v0 init rendering in outermost adapter code.

## What changed

- `api/core/v1alpha1/types.go`
- `crds/ai-models.deckhouse.io_models.yaml`
- `crds/ai-models.deckhouse.io_clustermodels.yaml`
- `images/controller/internal/domain/materialization/*`
- `images/controller/internal/application/materialization/*`
- `images/controller/internal/modelpackinit/*`
- `images/controller/README.md`
- `plans/active/implement-runtime-materializer-and-registry-release-baseline/*`

## Validations

- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `go test ./internal/modelpackinit ./internal/application/materialization ./internal/domain/materialization ./internal/application/publication` in `images/controller`
- `bash tools/check-controller-branch-matrix.sh`
- `bash tools/check-controller-complexity.sh`
- `bash tools/check-controller-loc.sh`
- `bash tools/check-thin-reconcilers.sh`
- `bash tools/test-controller-coverage.sh`
- `git diff --check`

## Residual risks

- No consumer wiring or workload mutation/injection exists yet.
- Current renderer intentionally assumes standard runtime secret conventions:
  `.dockerconfigjson` for OCI auth and `cosign.pub` for key-based Cosign
  verification.
- Bundle still needs the next slice for concrete consumer/module wiring.

## Next step

Следующий bounded slice:

- consume the current adapter renderer from a real runtime/workload integration
  path without leaking adapter-specific shapes into domain/application code.
