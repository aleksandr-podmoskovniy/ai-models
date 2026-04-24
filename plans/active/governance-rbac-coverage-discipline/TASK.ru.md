## 1. Заголовок

Добавить DKP RBAC coverage discipline в reusable skills и agent profiles

## 2. Контекст

RBAC audit для `ai-models` показал, что repo-local workflow surfaces требуют
учитывать auth/storage/API boundaries в целом, но не заставляют явно
перечислять Deckhouse user-facing RBAC access-level coverage.

Это governance task, потому что меняются:

- `AGENTS.md`;
- `.codex/README.md`;
- `.agents/skills/*`;
- `.codex/agents/*.toml`;
- docs/development workflow surfaces;
- `.codex/governance-inventory.json`.

## 3. Постановка задачи

Добавить переносимое правило: если задача меняет DKP user-facing API,
values/OpenAPI, auth/exposure, RBAC templates или runtime entrypoints, task
bundle и final review должны явно фиксировать:

- intended RBAC access-level coverage;
- scope: namespaced vs cluster-wide;
- allowed verbs/personas;
- intentional deny paths;
- validation evidence.

## 4. Scope

- `AGENTS.md`;
- `.codex/README.md`;
- `.agents/skills/task-intake-and-slicing/SKILL.md`;
- `.agents/skills/review-gate/SKILL.md`;
- `.agents/skills/k8s-api-design/SKILL.md`;
- `.agents/skills/platform-runtime-integration/SKILL.md`;
- `.codex/agents/task-framer.toml`;
- `.codex/agents/api-designer.toml`;
- `.codex/agents/integration-architect.toml`;
- `.codex/agents/reviewer.toml`;
- `docs/development/CODEX_WORKFLOW.ru.md`;
- `docs/development/TASK_TEMPLATE.ru.md`;
- `docs/development/REVIEW_CHECKLIST.ru.md`;
- `.codex/governance-inventory.json`.

## 5. Non-goals

- не создавать новый skill или agent только для RBAC;
- не добавлять ai-models-specific RBAC matrix в reusable core;
- не менять product templates в этом governance bundle;
- не менять `model-catalog-api` и `backend_integrator`, пока нет
  module-specific RBAC doctrine beyond generic DKP coverage.

## 6. Затрагиваемые области

- reusable governance baseline;
- planning/review workflow;
- API/integration/reviewer agent focus.

## 7. Критерии приёмки

- reusable core остался module-agnostic;
- project-specific RBAC details остались в product task bundle, а не протекли
  в generic skills/agents;
- existing skills/agents tightened instead of adding new role;
- task intake требует RBAC coverage для relevant tasks;
- review-gate проверяет RBAC coverage evidence;
- `api_designer` owns resource-level access semantics;
- `integration_architect` owns Deckhouse/global-vs-local RBAC wiring;
- `reviewer` checks missing RBAC coverage as finding;
- workflow docs and `.codex/governance-inventory.json` синхронизированы;
- `make lint-codex-governance` проходит.

## 8. Риски

- можно превратить reusable core в ai-models-specific policy;
- можно продублировать один и тот же текст во всех profiles вместо короткого
  role-specific focus;
- можно забыть governance inventory и получить не-enforced правило.
