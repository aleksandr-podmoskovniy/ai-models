# GPU-direct runtime delivery target design plan

## Current phase

Эта задача относится к Phase 2: distribution/runtime topology. Она не должна
ломать текущий publication baseline и не должна тащить accelerator-specific
knobs в `Model.spec`.

## Active bundle disposition

- `csi-workload-ux-contract`: keep. Текущий executable slice — SharedPVC RWX
  delivery contract; этот design может уточнить, что `SharedPVC` остаётся
  baseline, а accelerator-direct — отдельная optimization lane.
- `modelpack-efficient-compression-design`: keep. Chunked immutable layout and
  range/resume are prerequisites for any safe network-direct loading.
- `live-e2e-ha-validation`: keep. Исполняемый e2e после выката текущих changes.
- `observability-signal-hardening`: keep. Нужно расширить metrics/logs для
  cache-hit/cold-load paths после design approval.
- `production-readiness-hardening`: keep. Security and RBAC checks remain the
  umbrella stream.

## Orchestration

Mode: `full`.

Read-only reviews:

- `integration_architect`: storage/runtime/security/HA boundary.
- `reviewer`: challenge direct-to-GPU assumptions and production risks.

## Slice 1. Research and feasibility boundary

Artifacts:

- `plans/active/gpu-direct-runtime-delivery-design/DESIGN.ru.md`

Validation:

- References are official docs or primary project docs where possible.
- Design clearly separates `ai-models` and `ai-inference` responsibilities.

Status: implemented. Design rewritten as a short decision document with clear
`ai-models` vs `ai-inference` ownership.

## Slice 2. Target topology decision

Decision points:

- Keep `SharedDirect` as current fast path.
- Keep `SharedPVC` as universal no-local-disks filesystem path until
  accelerator-direct runtime proof exists.
- Add future `AcceleratedColdLoad` lane only through ai-inference runtime
  adapter, not generic workload mutation.

Validation:

- Reviewer must not find hidden credential, HA, or API-convention blocker.

Status: implemented in `DESIGN.ru.md`: `SharedDirect` and `SharedPVC` remain
the production filesystem baseline; `AcceleratedColdLoad` is a future
`ai-inference` optimization lane.

## Slice 3. Implementation roadmap

Expected slices:

1. Digest-scoped artifact read grant API.
2. Range/resume/chunk-index backed read path over canonical `ModelPack`.
3. Node-cache write-through lease and ready-marker semantics.
4. ai-inference runtime adapter POC for one runtime, for example vLLM or
   TensorRT-LLM, with explicit feature gate.
5. Scheduler capability model and e2e on supported hardware.

Status: implemented as roadmap sections in `DESIGN.ru.md`.

## Rollback point

Keep current `SharedDirect` + planned `SharedPVC` contract. Do not expose
accelerator-direct behavior until runtime proof and security design are accepted.

## Final validation

- `git diff --check`
- `make lint-docs` if docs are touched
- read-only reviewer sign-off for design consistency
