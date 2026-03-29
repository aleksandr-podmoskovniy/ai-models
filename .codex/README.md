# ai-models Codex context

Главный источник правил для Codex — `AGENTS.md`.

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

Core read-only agents:
- `task_framer` для task bundle
- `repo_architect` для layout и anti-patchwork решений
- `integration_architect` для runtime/build/integration boundaries
- `api_designer` для Kubernetes/DKP API semantics
- `reviewer` для финального review

Core write-capable agent:
- `module_implementer` для scoped implementation после того, как plan и boundaries уже ясны

Все перечисленные роли считаются read-only advisory roles, если явно не указано
обратное. В текущем baseline write-capable роли только `task_framer` и
`module_implementer`.

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
- fuzzy/broad intake -> `task-intake-and-slicing` и при необходимости `task_framer`
- layout/module shell -> `repo_architect`
- runtime/auth/storage/ingress/build/HA -> `integration_architect`
- backend-engine-specific runtime details -> `backend_integrator`
- `Model` / `ClusterModel` / API / CRD / conditions -> `api_designer`
- stable scoped implementation delegation -> `module_implementer`
- substantial final handoff -> `review-gate`; если использовалась delegation или задача multi-area, ещё и `reviewer`

Cadence:
1. До первого code change выбрать режим `solo` / `light` / `full`.
2. Если режим не `solo`, вызвать нужных read-only subagents до реализации.
3. Коротко зафиксировать findings в текущем `plans/active/<slug>/`.
4. Только после этого менять код.

Для ai-models-specific задач поверх этого добавлять:
- `backend_integrator` для internal backend engine и 3p runtime details
- `model-catalog-api` для `Model` / `ClusterModel`

Практический нюанс:
- repo хранит named agent profiles в `.codex/agents/`;
- если runtime даёт только generic subagent slots, эти профили всё равно
  остаются source of truth для роли, а их intent должен быть отражён в prompt
  делегированного subagent.

## Quick entry points

- `docs/development/TZ.ru.md` — что строим и по каким этапам.
- `docs/development/CODEX_WORKFLOW.ru.md` — как вести задачи через Codex.
- `plans/` — task bundles по конкретным изменениям.
