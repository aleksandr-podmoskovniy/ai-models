## 1. Current phase

Governance tightening for reusable DKP module workflow baseline.

Это отдельная governance task, не product/runtime slice.

## 2. Orchestration

`full`

Причина:

- меняется repo-local workflow surface;
- нужно проверить reusable-vs-overlay boundary;
- требуется финальный review и `make lint-codex-governance`.

Read-only reviews:

- `repo_architect` — minimal governance-surface edits and no new skills/agents;
- final `reviewer` after implementation.

## 3. Slices

### Slice 1. Зафиксировать read-only governance findings

Цель:

- сохранить выводы по тому, куда добавлять RBAC discipline.

Файлы/каталоги:

- `plans/active/governance-rbac-coverage-discipline/NOTES.ru.md`

Проверки:

- manual consistency review

Артефакт результата:

- decision: tighten existing reusable surfaces, no new skill/agent.

### Slice 2. Обновить reusable skills и docs

Цель:

- добавить RBAC coverage requirement в planning/review/API/integration
  workflow.

Файлы/каталоги:

- `AGENTS.md`
- `.codex/README.md`
- `.agents/skills/task-intake-and-slicing/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`
- `.agents/skills/k8s-api-design/SKILL.md`
- `.agents/skills/platform-runtime-integration/SKILL.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`

Проверки:

- manual consistency review

Артефакт результата:

- relevant tasks must name RBAC access levels, scope, verbs and deny paths.

### Slice 3. Обновить agent profiles

Цель:

- добавить role-specific RBAC focus без копирования skill text.

Файлы/каталоги:

- `.codex/agents/task-framer.toml`
- `.codex/agents/api-designer.toml`
- `.codex/agents/integration-architect.toml`
- `.codex/agents/reviewer.toml`

Проверки:

- manual consistency review

Артефакт результата:

- `api_designer`, `integration_architect`, `reviewer`, `task_framer`
  отражают RBAC responsibilities.

### Slice 4. Синхронизировать governance inventory

Цель:

- сделать новое правило machine-checkable.

Файлы/каталоги:

- `.codex/governance-inventory.json`

Проверки:

- `make lint-codex-governance`

Артефакт результата:

- inventory enforces RBAC discipline phrases.

## 4. Rollback point

После Slice 1 можно остановиться без governance edits.

После реализации rollback — вернуть только governance surfaces и inventory; product
templates не затронуты.

## 5. Final validation

- `make lint-codex-governance`
- final `review-gate`
- final `reviewer`
