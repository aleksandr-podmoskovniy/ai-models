# Рабочий процесс Codex для ai-models

## Принцип

Любая нетривиальная задача проходит один и тот же цикл:

1. intake;
2. task bundle;
3. выбор orchestration mode;
4. архитектурная проверка;
5. план;
6. реализация по slice;
7. review;
8. verification;
9. синхронизация документации.

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
- "Нужно поднять backend ai-models в модуле и подключить PostgreSQL и S3".
- "Нужно сделать values и OpenAPI для конфигурации backend ai-models".
- "Нужно начать проектирование Model и ClusterModel поверх backend ai-models".

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

### Шаг 2. Выбрать и вызвать нужных subagents

Сабагенты вызываются **до первого изменения кода**, если задача не в режиме
`solo`.

- для структуры репозитория — `repo_architect`;
- для runtime/build/integration boundaries — `integration_architect`;
- для Kubernetes/DKP API — `api_designer`;
- для backend-specific деталей поверх этого — `backend_integrator`.

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
