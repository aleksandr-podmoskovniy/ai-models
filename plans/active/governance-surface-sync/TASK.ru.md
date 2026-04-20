# Governance surface sync after runtime baseline reset

## 1. Заголовок

Синхронизировать repo-local governance surfaces с live publication/runtime
baseline

## 2. Контекст

После phase-reset и phase-2 runtime workstreams верхние instruction surfaces
разъехались с live repo baseline:

- `AGENTS.md` всё ещё описывает backend-first roadmap;
- `docs/development/CODEX_WORKFLOW.ru.md` держит stale examples;
- repo-local governance surface перестал быть единым source of truth для
  current project phase.

По правилам репозитория это отдельная governance task и не должна смешиваться с
product/runtime reset bundle.

## 3. Постановка задачи

Нужно выровнять `AGENTS.md` и связанные workflow-governance surfaces с текущим
live baseline репозитория после завершения product/runtime reset.

## 4. Scope

- `AGENTS.md`
- `.codex/README.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- при необходимости `docs/development/TASK_TEMPLATE.ru.md`
- при необходимости `docs/development/REVIEW_CHECKLIST.ru.md`
- task bundle для этой задачи

## 5. Non-goals

- не менять runtime code;
- не смешивать сюда CI shell fixes или product docs;
- не плодить новые agent roles или skills без необходимости.

## 6. Затрагиваемые области

- repo-local precedence surface;
- workflow/governance docs;
- durable project roadmap narrative.

## 7. Критерии приёмки

- `AGENTS.md`, `.codex/README.md` и workflow docs больше не противоречат live
  runtime baseline;
- stale backend-first phase narrative removed;
- `make lint-codex-governance` passes;
- updated instruction surfaces remain mutually consistent.

## 8. Риски

- можно случайно смешать roadmap wording fix with new runtime design decisions;
- можно обновить только один слой и оставить split-brain между precedence
  surfaces.
