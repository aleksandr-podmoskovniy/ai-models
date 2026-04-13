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
- `controller-runtime-implementation`

Reusable doctrine lives primarily in:

- `AGENTS.md`
- `task-intake-and-slicing`
- `review-gate`
- `controller-architecture-discipline`
- `controller-runtime-implementation`

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
- backend-engine-specific runtime details -> `backend_integrator`
- `Model` / `ClusterModel` / API / CRD / conditions -> `api_designer`
- scoped implementation after clear boundaries -> `module_implementer`
- substantial final handoff -> `review-gate`; если использовалась delegation
  или задача была multi-area, ещё и `reviewer`

Engineering expectations carried by this baseline:

- bounded slices over giant redesign prose
- durable rules in repo-local docs/skills instead of chat memory
- no wrapper-on-wrapper architecture
- test methodology by decision surface, not by helper accretion
- explicit review of governance changes as a first-class scope

Cadence:
1. До первого code change выбрать режим `solo` / `light` / `full`.
2. Если режим не `solo`, вызвать нужных read-only subagents до реализации.
3. Коротко зафиксировать findings в текущем `plans/active/<slug>/`.
4. Только после этого менять код.
5. Если меняется repo-local workflow surface, не смешивать это с product/runtime
   slice: делать отдельный governance bundle.

Для ai-models-specific задач поверх этого добавлять:
- `backend_integrator` для internal backend engine и 3p runtime details
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
