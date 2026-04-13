---
name: module-config-contract
description: Use for module values and OpenAPI contract work: user-facing vs internal values, defaults, globals, secrets, and compatibility boundaries.
---

# Module config contract

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task changes `openapi/config-values.yaml` or `openapi/values.yaml`;
- the task adds or removes module-level settings;
- the task changes defaults, secrets, global wiring, or compatibility semantics.

## Workflow

1. Keep the user-facing contract smaller than the internal runtime state.
2. Follow the virtualization-style split:
   - `openapi/config-values.yaml` only for stable user-facing module contract;
   - `openapi/values.yaml` and `internal.*` for derived/runtime wiring.
3. Do not expose adapter-specific runtime/materializer knobs as public module API.
4. Prefer global Deckhouse settings over local override knobs unless the module truly needs ownership.
5. Make secrets, ownership, and required external prerequisites explicit.
6. Keep defaults safe for rendering and clear for operators.
7. Do not expose unstable internals as public module API.

## Hard rules

- No dead public knobs without live semantics.
- No adapter/tool-specific runtime toggles in public values.
- No fake compatibility layers that outlive the migration they were added for.
- No public contract expansion without matching template/runtime ownership.

## Output

A short, stable, and explainable configuration contract.
