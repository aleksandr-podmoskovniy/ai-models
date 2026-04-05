# PLAN

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication, runtime delivery
direction к `ai-inference`.

## Orchestration

- mode: `full`
- read-only inputs before writing the bundle:
  - local repo audit of current publication/status/runtime boundaries;
  - upstream `KitOps` / `kit init` / security docs audit.

## Audit conclusions

### Local repo

- Public API уже source-first:
  - `spec.source = HuggingFace | HTTP | Upload`;
  - `status.artifact`;
  - `status.resolved`.
- Current live implementation уже публикует `KitOps` artifact в OCI, но runtime
  consumption path ещё не оформлен как production-ready architecture.
- `Upload(ModelKit)` пока explicit controlled failure.
- Controller architecture всё ещё fat и требует отдельного corrective refactor
  bundle; текущий design bundle не должен поверх этого рисовать implementation
  как будто она уже production-ready.

### Upstream KitOps

- Upstream `kit init` container already gives a bounded v0 runtime primitive:
  fetch ModelKit/ModelPack from OCI, optional Cosign verification, unpack into a
  shared volume, then exit.
- `KitOps` built-in integrity covers OCI digest verification during
  `pull/unpack`; cryptographic signatures are an external `Cosign` layer, not an
  all-in-one KitOps policy engine.
- Upstream docs explicitly position the init container as a generic Kubernetes
  adapter and recommend pinned image versions, optional Cosign verification, and
  PVC instead of `emptyDir` for larger models.

## Slice 1. Bundle Baseline And Rationale

Цель:

- зафиксировать, почему platform path должен идти через `KitOps` и OCI.

Артефакты:

- `TASK.ru.md`
- `RATIONALE.ru.md`

Проверки:

- manual consistency check against current API and runtime direction

## Slice 2. Full End-To-End Lifecycle

Цель:

- описать end-to-end flow:
  - source acceptance;
  - upload/publication;
  - status enrichment;
  - runtime materialization;
  - delete/cleanup.

Артефакты:

- `TARGET_ARCHITECTURE.ru.md`
- `USER_FLOWS.ru.md`
- `NOTES.ru.md`

Проверки:

- manual consistency check against current `Model` / `ClusterModel` API

## Slice 3. Security Model And Phased Rollout

Цель:

- зафиксировать security checks, non-guarantees, and implementation order.

Артефакты:

- `SECURITY.ru.md`

Проверки:

- manual consistency check against current controller/backend/runtime boundaries

## Rollback point

Это docs/design-only bundle. Безопасная остановка возможна в любой точке до
начала implementation slices.

## Final validation

- `git diff --check`
