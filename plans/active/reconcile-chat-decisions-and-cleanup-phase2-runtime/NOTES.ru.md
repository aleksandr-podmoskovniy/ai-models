# Notes

## Agreed invariants from chat

The current project memory must treat the following as durable decisions:

1. public and runtime contract always consume `OCI from registry`;
2. backend storage hidden under `DVCR` does not affect public or runtime
   contract;
3. published artifact contract is `ModelPack`;
4. concrete tools such as `KitOps`, `Modctl`, or a future module-owned
   implementation are adapters behind ports;
5. runtime always sees only a local path prepared by an init-container /
   materializer;
6. `mlflow` remains adjacent and must not be pulled back into the phase-2
   publication/runtime contract.

## Drift inventory

### Runtime / docs drift

- `images/controller/internal/application/publication/start_publication.go`
  still had user-facing wording around the old `KitOps/OCI` path.
- `docs/README.md` and `docs/README.ru.md` still described the module mainly as
  an internal registry backend instead of the agreed split:
  `ModelPack` contract + hidden backend + runtime materializer.
- `docs/CONFIGURATION.md` and `docs/CONFIGURATION.ru.md` still used the vague
  phrase `separate backend publication runtime`, which invites backend-centric
  reading instead of controller-owned publication/runtime adapters.

### Active-bundle drift

The active set contains several bundles that are already completed or now
subsumed by the current cleanup/runtime/refactor workstreams:

- `archive-stale-active-bundles`
- `fix-controller-distroless-runtime-shell`
- `implement-source-auth-secret-projection`
- `structure-project-skills-and-agent-memory`
- `design-full-kitops-model-lifecycle`

These are safe archive candidates because their durable outputs are already
carried either by the repo state itself or by the current canonical bundles:

- `refactor-controller-hexagonal-architecture-and-quality-gates`
- `reconcile-chat-decisions-and-cleanup-phase2-runtime`

## Active set after cleanup

The active tree is now intentionally reduced to two canonical workstreams:

- `refactor-controller-hexagonal-architecture-and-quality-gates`
- `reconcile-chat-decisions-and-cleanup-phase2-runtime`

The speculative runtime-only bundle
`implement-runtime-materializer-and-registry-release-baseline` was archived
once its detached controller code was removed from the live tree.

## Conservative cleanup rule

Do not archive cluster-specific or phase-1 investigation bundles unless the
workstream is clearly completed or obviously superseded. Archive first the
bundles that are both:

- completed or already reflected in repo state;
- overlapping with the current canonical cleanup/runtime/refactor path.
