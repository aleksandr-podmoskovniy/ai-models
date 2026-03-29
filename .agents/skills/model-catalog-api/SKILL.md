---
name: model-catalog-api
description: Ai-models-specific overlay for phase-2 and later work on Model and ClusterModel, controller boundaries, status.conditions, immutability, and internal backend synchronization semantics.
---

# Model catalog API overlay

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task touches `Model` or `ClusterModel`;
- the task changes status, conditions, validation, or immutability;
- the task changes controller ownership or synchronization with the internal backend.

## Workflow

1. Make creator, reconciler, and deletion owner explicit.
2. Keep desired state in `spec` and computed state in `status`.
3. Use `metav1.Condition` and stable reasons.
4. Keep `Model` and `ClusterModel` semantically aligned.
5. Keep internal backend details behind the public contract.

## Output

A DKP-native API that users can understand without knowing internal backend mechanics.
