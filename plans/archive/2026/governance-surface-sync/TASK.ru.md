# Governance surface sync for portable multi-agent baseline

## 1. Заголовок

Привести repo-local skills, agents и governance docs к переносимому
multi-agent baseline для DKP modules.

## 2. Контекст

В репозитории уже есть сильный repo-local Codex surface, но сейчас он
неоднороден:

- верхний слой (`AGENTS.md`, `.codex/README.md`) уже декларирует reusable core
  plus project-specific overlays;
- часть core skills и review surfaces всё ещё формулирует правила через
  `ai-models`-specific narrative;
- agent profiles и governance inventory отражают capability split только
  частично;
- следующий модуль (`ai-inference`) должен уметь перенять этот baseline без
  копирования лишней предметной специфики.

По правилам репозитория это отдельная governance task и не должна смешиваться
с product/runtime workstreams.

## 3. Постановка задачи

Нужно вычистить и актуализировать `AGENTS.md`, `.agents`, `.codex` и связанные
workflow docs так, чтобы:

- reusable core был объективным, компактным и переносимым между DKP modules;
- `ai-models`-specific правила оставались только в явных overlays;
- skill/agent responsibilities были различимы и управляемы;
- governance surface можно было копировать в следующий модуль без
  split-brain между слоями.

## 4. Scope

- `plans/active/governance-surface-sync/*`
- `AGENTS.md`
- `.codex/README.md`
- `.codex/governance-inventory.json`
- `.codex/agents/*.toml`
- `.agents/skills/*`
- связанные workflow docs:
  - `docs/development/CODEX_WORKFLOW.ru.md`
  - `docs/development/TASK_TEMPLATE.ru.md`
  - `docs/development/REVIEW_CHECKLIST.ru.md`
  - `plans/README.md`
  - при необходимости `docs/development/REPO_LAYOUT.ru.md`

## 5. Non-goals

- не менять product/runtime code, API, values, templates или CI shell;
- не проектировать новый runtime/API behavior под видом governance cleanup;
- не плодить новые agent roles или skills, если тот же результат достигается
  tightening existing boundaries;
- не удалять `ai-models`-specific overlays, которые реально нужны текущему
  модулю.

## 6. Затрагиваемые области

- repo-local precedence surface;
- reusable core skills baseline;
- read-only vs write-capable agent split;
- governance inventory и machine-checkable guardrails;
- explicit porting contract for baseline reuse in a new module repo;
- workflow docs, которые должны объяснять этот baseline одинаково.

## 7. Критерии приёмки

- `AGENTS.md`, `.codex/README.md`, `.agents/skills/*`, `.codex/agents/*.toml`
  и связанные workflow docs описывают один и тот же governance baseline;
- reusable core явно отделён от `ai-models`-specific overlays;
- core skills не тащат предметную специфику `ai-models` там, где правило должно
  переноситься в другие DKP modules;
- project-specific overlays остаются явными и не маскируются под generic core;
- agent profiles задают role-specific focus и не дублируют skills слово в
  слово;
- governance inventory остаётся актуальным источником для
  `make lint-codex-governance`;
- porting contract явно перечисляет overlay skills/agents и files that must be
  reviewed and rewritten in a new module repo before product work starts;
- `make lint-codex-governance` passes;
- `git diff --check` passes.

## 8. Риски

- можно вычистить слишком агрессивно и потерять полезную repo-local память для
  текущего модуля;
- можно оставить split-brain, если обновить only skill texts without sync of
  agents, inventory and workflow docs;
- можно случайно превратить generic baseline в абстрактную методичку без
  конкретных guardrails.
