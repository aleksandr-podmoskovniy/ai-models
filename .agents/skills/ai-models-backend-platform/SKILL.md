---
name: ai-models-backend-platform
description: Ai-models-specific overlay for phase-1 and later work around the internal managed backend engine on top of the generic platform/runtime and 3p integration skills.
---

# Ai-models backend platform overlay

## Read first

1. `AGENTS.md`
2. `docs/development/REPO_LAYOUT.ru.md`
3. `docs/development/TZ.ru.md`
4. `docs/development/PHASES.ru.md`
5. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task changes internal backend deployment or configuration;
- the task changes PostgreSQL or object storage wiring;
- the task changes auth, SSO, monitoring, logging, or workspaces;
- the task changes import/publish boundaries between the module and the internal backend engine.

## Workflow

1. Treat the backend engine as an internal managed component of the module.
2. Keep storage, auth, and observability consistent with DKP primitives.
3. Keep raw backend entities out of the future public platform API.
4. Prefer reproducible upstream baselines before patching or slimming.
5. Do not drag phase-2 catalog API work into a phase-1 backend task.

## Hard rules

- No backend entity leaked as public platform contract.
- No phase-2 runtime shortcut justified by phase-1 backend convenience.
- No backend/storage/auth change without matching module-shell and config
  contract review.

## Output

A working, explainable, and upgradeable internal backend integrated into DKP.
