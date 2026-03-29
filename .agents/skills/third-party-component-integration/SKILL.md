---
name: third-party-component-integration
description: Use for external 3p component integration: upstream import, source-of-truth selection, patch queue discipline, image build shape, rebasing, and update flow.
---

# Third-party component integration

## Read first

1. `AGENTS.md`
2. `DEVELOPMENT.md`
3. `docs/development/TZ.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task integrates an upstream project into the module;
- the task changes source import, patch queue, rebasing, or packaging shape;
- the task changes base images, distroless strategy, or 3p build reproducibility.

## Workflow

1. Keep upstream source separate from module runtime templates and controllers.
2. Keep a clear source-of-truth for versions, patches, and update mechanics.
3. Prefer reproducible upstream-like builds before local simplification.
4. Minimize local divergence and document every intentional fork point.
5. Keep update/rebase flow explicit enough to repeat without guesswork.

## Output

A 3p integration path that is repeatable, reviewable, and maintainable.
