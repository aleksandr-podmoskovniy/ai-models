---
name: dkp-module-shell
description: Use for DKP module shell work: module.yaml, Chart.yaml, values/OpenAPI wiring, templates, werf, CI/CD, docs layout, and staged rollout of internal components.
---

# DKP module shell

## Read first

1. `AGENTS.md`
2. `DEVELOPMENT.md`
3. `docs/development/REPO_LAYOUT.ru.md`
4. `docs/development/TZ.ru.md`
5. `docs/development/PHASES.ru.md`
6. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task changes the DKP module root or release shell;
- the task changes `module.yaml`, `Chart.yaml`, `openapi/`, templates, `werf`, CI/CD, docs layout, or bundle packaging;
- the task introduces or wires internal managed components of the module.

## Workflow

1. Keep the repository module-oriented, not operator-repo-oriented.
2. Keep metadata, values schema, templates, docs, and build files aligned.
3. Prefer narrow vertical slices that can be rendered and verified.
4. Do not mix controller internals into module shell work unless the plan explicitly says so.
5. Record every new moving part in docs when it changes future engineering work.

## Output

A supportable DKP module shell that can survive long-term maintenance.
