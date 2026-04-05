# REVIEW

## Slice result

Closed the first corrective cleanup slice for this workstream:

- explicit chat-agreement inventory is now recorded in
  `reconcile-chat-decisions-and-cleanup-phase2-runtime/NOTES.ru.md`;
- runtime/docs wording was aligned with the agreed invariants:
  - `OCI from registry`;
  - hidden backend under `DVCR`;
  - `ModelPack` as contract;
  - concrete implementations as adapters;
  - runtime sees only a local path prepared by materialization;
- repo-local skills were tightened so future continuation work is forced to:
  - reuse canonical active bundles;
  - keep `ModelPack`/`OCI` runtime invariants;
  - review whether changes landed in the right bundle;
- top-level docs now describe the module through the current phase-2 split
  instead of through a raw internal registry-backend lens;
- `plans/active` is reduced to two canonical workstreams:
  - controller corrective refactor;
  - this cleanup/reconciliation bundle.

Closed the next bounded cleanup slice on top of that baseline:

- deleted speculative detached runtime graph once it was proven outside the
  live controller path:
  - `images/controller/internal/application/materialization/*`
  - `images/controller/internal/domain/materialization/*`
  - `images/controller/internal/modelpackinit/*`
- removed low-signal duplicate tests:
  - `images/controller/internal/application/publication/facade_test.go`
  - `images/controller/internal/publicationoperation/type_aliases_test.go`
- aligned controller README and active bundles so the repo no longer describes
  that dead seam as part of the current structure;
- archived the superseded speculative bundle
  `implement-runtime-materializer-and-registry-release-baseline`.

## What changed

- `plans/active/reconcile-chat-decisions-and-cleanup-phase2-runtime/*`
- `.agents/skills/controller-runtime-implementation/SKILL.md`
- `.agents/skills/task-intake-and-slicing/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`
- `images/controller/internal/application/publication/start_publication.go`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/README.md`
- `plans/archive/2026/implement-runtime-materializer-and-registry-release-baseline/*`
- `plans/active/refactor-controller-hexagonal-architecture-and-quality-gates/*`

## Validations

- `go test ./internal/application/publication`
- `go test ./internal/application/publication ./internal/publicationoperation ./internal/domain/publication` in `images/controller`
- `bash tools/check-controller-branch-matrix.sh`
- `bash tools/test-controller-coverage.sh`
- `git diff --check`

## Residual risks

- Current active set is cleaner, but some archived bundles still carry
  vendor-specific names in `plans/archive/2026/`; this is acceptable as
  historical record and should not be used as current source of truth.
- More speculative seams may still remain in `publicationoperation`,
  `sourcepublishpod`, and `uploadsession`; this slice only removed the ones
  already proven dead.
- The next implementation slice should land only a real consumer/runtime path,
  not revive detached speculative materialization code inside controller.

## Next step

- continue hard cleanup on the clean active set;
- then land the next bounded runtime slice on top of the current hexagonal
  controller structure instead of reviving archived guidance.
