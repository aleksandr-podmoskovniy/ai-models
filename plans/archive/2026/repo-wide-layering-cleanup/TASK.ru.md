# TASK

## Контекст
После серии phase-1 auth/storage/import slices в `ai-models` накопились не только локальные, но и repo-wide наслоения: старые task bundles в `plans/active`, naming drift, остаточные legacy references, runtime/build мусор и docs/build/template paths, которые уже не соответствуют текущему каноническому baseline.

## Постановка задачи
Провести repo-wide cleanup наслоений в `ai-models` и удалить только те legacy paths, которые уже можно безопасно убрать без ломки текущего phase-1 backend baseline.

## Scope
- аудит repo-wide layering/legacy drift;
- cleanup stale `plans/active` bundles и naming/docs/build leftovers;
- удаление явного dead code/runtime/build мусора;
- выравнивание repo под текущий phase-1 auth/storage/import baseline.

## Non-goals
- не менять phase-2 API и controller design;
- не проводить новые архитектурные эксперименты поверх MLflow;
- не переписывать working phase-1 flows только ради stylistic refactor.

## Затрагиваемые области
- `plans/active/*`
- `docs/*`
- `templates/*`
- `images/*`
- `tools/*`
- repo-level hygiene files

## Критерии приёмки
- в репозитории не остаются явные мёртвые пути, stale active bundles и живые compatibility shims, которые уже superseded текущим baseline;
- naming/docs/build/template wiring не противоречат текущему phase-1 contract;
- cleanup не ломает verify loop;
- `make verify` проходит.

## Риски
- broad cleanup легко превращается в бесконтрольную косметику, поэтому нужно удалять только подтверждённые stale/dead paths;
- удаление последнего compatibility shim меняет upgrade story для очень старых инсталляций, где внутренний auth Secret ещё не мигрировал на `machineUsername` / `machinePassword`.
