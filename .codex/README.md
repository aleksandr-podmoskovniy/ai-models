# ai-models Codex context

Главный источник правил для Codex — `AGENTS.md`.

Precedence внутри repo-local Codex surface:

1. `AGENTS.md`
2. `.codex/README.md`
3. `.agents/skills/*`
4. `.codex/agents/*.toml`

Нижний уровень не должен противоречить верхнему. Если меняется один из этих
слоёв, consistency review обязателен для всех затронутых уровней.

Назначение этой поверхности:

- давать переносимый reusable baseline для DKP module repos;
- удерживать engineering doctrine вне chat-only context;
- не допускать split-brain между skills, agent profiles и task bundles.
- удерживать workflow docs в `docs/development/` и `plans/README.md`
  выровненными с этим instruction surface.
- держать reusable core отдельно от project-specific overlays.

Current ai-models baseline:

- live roadmap is publication/runtime-first, not backend-first;
- internal registry/publication backend stays implementation detail of the
  module;
- workflow surface must describe the current `Model` / `ClusterModel` plus
  publication/runtime baseline, not archived pre-phase attempts.

## Reusable core

Этот репозиторий хранит не только project-specific роли, но и reusable baseline
для DKP module repos.

Core skills:
- `task-intake-and-slicing`
- `review-gate`
- `dkp-module-shell`
- `module-config-contract`
- `third-party-component-integration`
- `platform-runtime-integration`
- `k8s-api-design`
- `controller-architecture-discipline`
- `controller-runtime-implementation`

Reusable doctrine lives primarily in:

- `AGENTS.md`
- `task-intake-and-slicing`
- `review-gate`
- `controller-architecture-discipline`
- `controller-runtime-implementation`

Правило переносимости:

- core skills должны оставаться module-agnostic;
- product/runtime/API specifics текущего модуля должны жить в overlays;
- agent profiles должны задавать role-specific focus, а не копировать skill
  texts слово в слово.

Core read-only agents:
- `repo_architect` для layout и anti-patchwork решений
- `integration_architect` для runtime/build/integration boundaries
- `api_designer` для Kubernetes/DKP API semantics
- `reviewer` для финального review

Write-capable agents:
- `task_framer` — docs-only writer for `plans/active/<slug>/`; не пишет код
- `module_implementer` — scoped implementation writer после того, как plan и boundaries уже ясны

Все остальные роли считаются read-only advisory roles, если явно не указано
обратное.

## Project-specific overlays

`ai-models` добавляет поверх reusable baseline только два domain overlays:
- `ai-models-backend-platform` / `backend_integrator`
- `model-catalog-api`

Они нужны только там, где generic core уже не покрывает предметную область.
`backend_integrator` относится к внутренним publication backend/runtime
деталям, а не к backend-first roadmap как project phase.

Project-specific overlay skills:
- `ai-models-backend-platform`
- `model-catalog-api`

Project-specific overlay agents:
- `backend_integrator`

## Recommended orchestration

Режимы:
- `solo` — small scoped task, без subagents
- `light` — task bundle + один read-only architect
- `full` — task bundle + несколько read-only architects + финальный `reviewer`

Decision matrix:
- fuzzy/broad intake -> `task-intake-and-slicing`, а `task_framer` только если
  нужен отдельный docs-only framing pass
- layout/module shell/repo topology -> `repo_architect`
- runtime/auth/storage/ingress/build/HA/observability -> `integration_architect`
- publication-backend-specific runtime details -> `backend_integrator`
- `Model` / `ClusterModel` overlay semantics -> `model-catalog-api`
- broader Kubernetes / DKP API / CRD / conditions semantics -> `api_designer`
- scoped implementation after clear boundaries -> `module_implementer`
- substantial final handoff -> `review-gate`; если использовалась delegation
  или задача была multi-area, ещё и `reviewer`

Engineering expectations carried by this baseline:

- bounded slices over giant redesign prose
- durable rules in repo-local docs/skills instead of chat memory
- no wrapper-on-wrapper architecture
- test methodology by decision surface, not by helper accretion
- explicit review of governance changes as a first-class scope
- portable core over disguised module-specific prose

Machine-checkable governance baseline:

- `.codex/governance-inventory.json` is the repo-local inventory for the
  precedence chain, core skills, agent capability split, and workflow-doc
  guardrails.
- `make lint-codex-governance` validates that `AGENTS.md`, `.codex/README.md`,
  skills, agent profiles, and core workflow docs still match that inventory
  instead of drifting silently.

Cadence:
1. До первого code change выбрать режим `solo` / `light` / `full`.
2. Если режим не `solo`, вызвать нужных read-only subagents до реализации.
3. Коротко зафиксировать findings в текущем `plans/active/<slug>/`.
4. Только после этого менять код.
5. Если меняется repo-local workflow surface, не смешивать это с product/runtime
   slice: делать отдельный governance bundle.

## Porting pattern

Когда этот baseline переносится в другой DKP module repo:

- сначала открыть dedicated governance porting bundle, а не начинать с
  product/runtime diff;
- копировать precedence chain, core skills, core agents,
  `.codex/governance-inventory.json` и workflow docs как baseline mechanics;
- оставлять reusable core module-agnostic;
- заменять product phase docs и project-specific overlays осознанно, а не
  переписывать под них generic core.

Files that must be reviewed and rewritten during porting:
- `AGENTS.md`
- `.codex/README.md`
- `docs/development/TZ.ru.md`
- `docs/development/PHASES.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/development/CODEX_WORKFLOW.ru.md`

Baseline porting bundle must capture:
- source repo baseline;
- copied reusable core surfaces;
- replaced or removed overlay skills and agents;
- rewritten repo-specific docs and phase narrative;
- `make lint-codex-governance` result before the first product/runtime slice.

Для ai-models-specific задач поверх этого добавлять:
- `backend_integrator` для internal publication backend/runtime и 3p runtime
  details
- `model-catalog-api` для `Model` / `ClusterModel`

Практический нюанс:
- repo хранит named agent profiles в `.codex/agents/`;
- repo-local skills — durable source of truth для workflow discipline;
- agent profiles не должны копировать skills слово в слово, а должны задавать
  role-specific focus и constraints;
- если runtime даёт только generic subagent slots, intent repo-local agent
  profile должен быть отражён в prompt делегированного subagent.

## Quick entry points

- `docs/development/TZ.ru.md` — что строим и по каким этапам.
- `docs/development/CODEX_WORKFLOW.ru.md` — как вести задачи через Codex.
- `plans/` — task bundles по конкретным изменениям.
