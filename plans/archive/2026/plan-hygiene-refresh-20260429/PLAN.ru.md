# Plan: active bundle hygiene refresh

## 1. Current phase

Workflow hygiene after implementation slices. Цель — уменьшить active context
до реального executable shortlist.

## 2. Orchestration

Mode: `solo`.

Reason: task changes only plan files and directory placement. No runtime/API
architecture decision is introduced.

## 3. Active bundle disposition

Keep:

- `live-e2e-ha-validation` — canonical executable live runbook after rollout.
- `observability-signal-hardening` — has pending Slice 4 for live metrics,
  alert wiring and log field dictionary cleanup.
- `ray-a30-ai-models-registry-cutover` — has pending Slice 5 for post-rollout
  endpoint load validation.

Archive:

- `capacity-cache-admission-hardening` — all implementation slices are marked
  implemented; remaining proof is covered by `live-e2e-ha-validation`.
- `pre-rollout-defect-closure` — all defect-closure slices and validation are
  completed.
- `public-docs-virtualization-style` — docs structure slice and review gate are
  completed; later content changes are owned by their feature bundles.
- `source-capability-taxonomy-ollama` — already closed after Ollama registry
  implementation and archived under `plans/archive/2026/`.

## 4. Slices

### Slice 1. Update active disposition

- update `live-e2e-ha-validation/PLAN.ru.md`;
- update `observability-signal-hardening/PLAN.ru.md`;
- update `ray-a30-ai-models-registry-cutover/PLAN.ru.md`.

Status: done.

### Slice 2. Archive completed bundles

- move completed bundles from `plans/active` to `plans/archive/2026`;
- leave this hygiene bundle archived after completion.

Status: done.

## 5. Rollback point

Move archived bundle directories back to `plans/active` and revert disposition
edits.

## 6. Validation

- `find plans/active -maxdepth 1 -mindepth 1 -type d`;
- `git diff --check`.

Status: passed.
