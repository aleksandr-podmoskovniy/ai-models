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
6. Keep runtime/materialization internals behind the public contract.
7. Treat `ModelPack` as the publication contract and keep concrete
   implementation brands such as `KitOps` or `Modctl` behind adapters.
8. Keep the public publication contract simple:
   - users choose `spec.source`
   - users choose `spec.inputFormat`
   - `spec.source` should stay close to user intent:
     `source.url` or `source.upload`, not nested provider trees
   - `spec.inputFormat` may be omitted only when the controller can determine
     it unambiguously from the actual contents
   - fixed internal output format stays hidden and must not be echoed back into
     public `spec`
   - dead public knobs with no live controller semantics, such as
     placeholder `spec.publish` blocks, must be removed instead of kept as
     speculative contract noise
9. Public input format names must be real format names such as `Safetensors`
   or `GGUF`, not source-coupled names such as `HuggingFaceDirectory` or vague
   internal names such as `HFCheckpoint`.
10. Do not expose prebuilt external artifacts as a normal public input path.
    The module must build its own canonical `ModelPack` artifact from validated
    raw model contents.
11. If the task also touches controller boundaries, read `.agents/skills/controller-architecture-discipline/SKILL.md` before implementation.

## Output

A DKP-native API that users can understand without knowing internal backend mechanics.
