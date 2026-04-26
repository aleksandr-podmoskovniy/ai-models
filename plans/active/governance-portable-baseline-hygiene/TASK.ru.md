# Унифицировать governance baseline и очистить active bundles

## Контекст

`plans/active` разросся до смеси текущих задач, завершённых slices,
исследований и исторических bundles. Одновременно repo-local skills/agents
используются как reusable baseline для будущего соседнего DKP module repo
`ai-inference`, поэтому generic core не должен тащить ai-models-specific
narrative и старые архитектурные болячки.

## Постановка задачи

Сделать governance surface переносимее и строже:

- оставить в `plans/active` только реально активные рабочие surfaces;
- архивировать завершённые/исторические bundles;
- усилить reusable skills/agents правилами active hygiene и porting discipline;
- явно отделить reusable core от ai-models overlays перед переносом в
  `ai-inference`.

## Scope

- `plans/active/*` и `plans/archive/2026/*`;
- `.codex/README.md`;
- `.codex/governance-inventory.json`;
- `.agents/skills/task-intake-and-slicing/SKILL.md`;
- `.agents/skills/review-gate/SKILL.md`;
- `.codex/agents/task-framer.toml`;
- `.codex/agents/repo-architect.toml`;
- `.codex/agents/reviewer.toml`;
- текущий bundle `plans/active/governance-portable-baseline-hygiene`.

## Non-goals

- Не менять product/runtime код, templates, API, RBAC или build shell.
- Не портировать baseline в `ai-inference` прямо сейчас.
- Не создавать новые skills или agent roles, если достаточно tightening
  существующих.
- Не удалять инженерную историю без необходимости: завершённые bundles
  архивируются, а не теряются.

## Критерии приёмки

- `plans/active` не содержит завершённых rootCA/TLS/auth/RBAC/code-reduction
  bundles и одноразовых research/smoke surfaces.
- В reusable core явно закреплено, что active bundle должен быть executable
  working surface, а не historical log.
- Skills/agents требуют dedicated baseline-porting bundle перед работой в
  соседнем модуле.
- Project-specific overlays остаются отдельными от reusable core.
- `.codex/governance-inventory.json` проверяет новые reusable guardrails.
- Пройдены `make lint-codex-governance`, `git diff --check`, а при
  возможности `make verify`.

## Риски

- Можно заархивировать bundle, который ещё нужен как активный workstream.
  Поэтому переносим только завершённые, reviewed или одноразовые surfaces.
- Можно accidentally усилить generic core ai-models-specific формулировками.
  Поэтому новые правила должны говорить про DKP module repos в целом.
