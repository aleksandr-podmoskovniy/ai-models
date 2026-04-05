# PLAN

## Current phase

Supporting workflow/runtime-discipline slice over phase-2 development.

## Orchestration

- mode: `light`
- read-only subagent:
  - audit current `.agents/skills` and recommend the minimal restructuring

## Slice 1. Audit Current Skills Against Current Repo Discipline

Цель:

- понять, что уже encoded in skills and what still lives only in `AGENTS.md`
  and recent bundles.

Файлы/каталоги:

- `.agents/skills/*`
- current corrective bundles

Проверки:

- manual consistency audit

Результат аудита:

- existing skills already covered the right trigger surface;
- the main remaining gap was brittle dependence on specific
  `plans/active/*` bundles for durable controller discipline;
- the right fix is a stable repo-local reference under the skill itself,
  not more active bundles and not many new skills.

## Slice 2. Tighten Existing Skills

Цель:

- update existing skills where the concern already belongs there.

Файлы/каталоги:

- `.agents/skills/task-intake-and-slicing/SKILL.md`
- `.agents/skills/controller-runtime-implementation/SKILL.md`
- `.agents/skills/model-catalog-api/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`

Проверки:

- manual consistency against current repo discipline

## Slice 3. Add Minimal Missing Skill If Needed

Цель:

- add one repo-specific skill only if current gaps cannot be expressed through
  existing skills.

Файлы/каталоги:

- `.agents/skills/*`

Проверки:

- trigger surface review
- `git diff --check`

Фактическое решение:

- keep the single new repo-specific skill `controller-architecture-discipline`;
- move durable controller rules from active-bundle references into a stable
  `references/` file under that skill;
- keep task-local bundle reads only for slice-specific execution context.

## Rollback point

Before editing existing skills or adding a new one.
