# Рабочий процесс Codex для ai-models

## Принцип

Любая нетривиальная задача проходит один и тот же цикл:

1. intake;
2. `TASK.ru.md`;
3. `PLAN.ru.md`;
4. выбор orchestration mode;
5. read-only review по границам, если он обязателен;
6. реализация по одному slice;
7. узкие проверки после каждого slice;
8. финальный review;
9. repo-level verification;
10. синхронизация документации.

## Режимы orchestration

- `solo` — одна узкая задача; сабагенты не нужны.
- `light` — task bundle и один read-only subagent по главному риску.
- `full` — task bundle, несколько read-only subagents и финальный `reviewer`.

`solo` подходит для:
- одного понятного bugfix;
- mechanical cleanup;
- docs-only правки без архитектурных решений.

`light` или `full` обязательны, если задача меняет:
- больше одной области репозитория;
- layout, build shell, CI shell или module boundaries;
- values/OpenAPI/API contract;
- auth, storage, ingress/TLS, observability, HA;
- upstream patching/rebase или phase boundary.

## Что делает человек

Человек пишет задачу обычным языком.

Примеры:
- "Нужно стабилизировать publication/runtime baseline ai-models и object-storage wiring".
- "Нужно сделать values и OpenAPI для publication/runtime конфигурации ai-models".
- "Нужно спроектировать следующий runtime slice поверх текущего `Model` / `ClusterModel` baseline".

Человек не обязан сам раскладывать задачу на архитектуру, slice и rollback.

## Что обязан сделать Codex

### Шаг 1. Превратить задачу в task bundle

Создать папку `plans/active/<slug>/` и положить туда:
- `TASK.ru.md`
- `PLAN.ru.md`

`TASK.ru.md` отвечает на вопросы:
- что нужно сделать;
- что входит в scope;
- что не входит в scope;
- какие критерии приёмки;
- какие риски.

`PLAN.ru.md` отвечает на вопросы:
- какие slice нужны;
- какие каталоги и файлы затрагиваются;
- какие проверки нужны на каждом шаге;
- где rollback point.

Task bundle может оформить сам основной агент. `task_framer` нужен тогда, когда
intake слишком широкий, расплывчатый или смешивает несколько workstreams.

Bundle hygiene:
- reuse canonical active bundle, если задача продолжает текущий workstream;
- не заводить sibling source of truth в `plans/active/`;
- архивировать stale или oversized active bundle, если он перестал быть
  рабочей поверхностью.

Если задача меняет repo-local workflow surface:
- `AGENTS.md`
- `.codex/*`
- `.agents/skills/*`
- `.codex/agents/*`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `plans/README.md`

то это отдельный governance bundle, а не incidental wording fix inside another
product/runtime task.

Если задача меняет сам reusable governance baseline, bundle должен явно
зафиксировать:
- что остаётся reusable core;
- что остаётся project-specific overlay;
- какие instruction layers обязаны остаться взаимно согласованными.

Если baseline переносится из другого репозитория, это тоже governance task.
Первый bundle в новом repo должен явно зафиксировать:
- source repo baseline;
- какие reusable core surfaces копируются;
- какие overlay skills/agents заменяются или удаляются;
- какие repo-specific docs переписываются до первого product/runtime slice.

### Шаг 2. Выбрать и вызвать нужных subagents

Сабагенты вызываются **до первого изменения кода**, если задача не в режиме
`solo`.

- для структуры репозитория — `repo_architect`;
- для runtime/build/integration boundaries — `integration_architect`;
- для Kubernetes/DKP API — `api_designer`;
- для publication-backend-specific деталей поверх этого — `backend_integrator`.

Их findings должны быть коротко зафиксированы в текущем
`plans/active/<slug>/PLAN.ru.md` или соседних notes до реализации.

### Шаг 3. Реализовывать только по plan

Код и templates меняются только после task bundle.
Каждый slice должен быть маленьким и проверяемым.

### Шаг 4. Закрыть задачу review gate

Для substantial task финальный handoff обязательно проходит через
`review-gate`.

Если использовалась delegation или задача была multi-area, дополнительно
вызывается `reviewer`, который сверяет результат с task bundle.

### Шаг 5. Прогнать machine-checkable guardrails

Если менялся repo-local workflow/governance surface, обязательно прогнать:
- `make lint-codex-governance`

Перед завершением задачи repo-level verification всё равно идёт через обычный
`make verify`.

### Шаг 6. Проверить переносимость governance baseline

Если менялись skills/agents/workflow docs:
- reusable core должен остаться module-agnostic;
- project-specific product/runtime rules должны остаться в overlays или
  module docs;
- нельзя прятать предметную специфику одного модуля в supposedly generic
  core skill/agent.

Если это baseline porting task:
- overlays из source repo не должны пережить копирование по инерции;
- repo purpose, phases и layout docs должны быть переписаны под новый модуль;
- `make lint-codex-governance` должен быть зелёным до первого product diff.

## Как пользоваться из VS Code

### Для новой крупной задачи
Попросить:

> Сначала оформи задачу в task bundle по правилам репозитория и не меняй код.

### Для реализации после плана
Попросить:

> Работай строго по текущему `plans/active/<slug>/PLAN.ru.md`, реализуй только первый slice, выполни узкие проверки и покажи, что изменилось.

### Для финализации
Попросить:

> Сделай финальный review по `review-gate`, а если задача substantial или multi-area, добавь `reviewer`; перечисли замечания, недостающие проверки и остаточные риски.

## Правило по стадиям

Codex всегда должен сначала проверить, к какому этапу относится задача.
Если задача пытается перепрыгнуть через текущий working baseline, Codex должен сначала это зафиксировать и предложить нормальную последовательность.
