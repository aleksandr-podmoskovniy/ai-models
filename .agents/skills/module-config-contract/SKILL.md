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
2. Prefer global Deckhouse settings over local override knobs unless the module truly needs ownership.
3. Make secrets, ownership, and required external prerequisites explicit.
4. Keep defaults safe for rendering and clear for operators.
5. Do not expose unstable internals as public module API.

## Output

A short, stable, and explainable configuration contract.
