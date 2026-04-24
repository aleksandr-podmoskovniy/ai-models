---
name: k8s-api-design
description: Use for Kubernetes and DKP API design: CRD boundaries, scope, spec/status split, immutability, conditions, naming, and ownership semantics.
---

# Kubernetes API design

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task adds or changes CRDs or user-facing resources;
- the task changes spec/status boundaries, conditions, defaults, or immutability;
- the task changes reconciliation ownership or public API semantics.

## Workflow

1. Keep desired state in `spec` and observed state in `status`.
2. Use stable naming and stable condition reasons.
3. Make scope, ownership, and deletion semantics explicit.
4. Keep internal backend details behind the public API.
5. For DKP user-facing resources, state RBAC access-level coverage: personas,
   namespaced vs cluster-wide scope, allowed verbs, and denied subresources.
6. Prefer boring, stable API semantics over clever shortcuts.

## Hard rules

- No internal transport/storage/output format leaked into public `spec`.
- No conditions whose reasons/messages are unstable by design.
- No mixed ownership between `spec`, `status`, and hidden backend state.
- No placeholder API knob kept “for later”.
- No user-facing RBAC plan that grants status/finalizers or internal runtime
  state by accident.

## Output

A Kubernetes-native API that can survive versioning and long-term support.
