# Публичная документация в DKP-стиле

## Контекст

После упрощения `ModuleConfig` пользовательская документация `ai-models`
осталась неполной: есть `README` и `CONFIGURATION`, но нет нормального
разделения на обзор, администрирование, пользовательские сценарии, CRD,
примеры и FAQ. В `virtualization` и `gpu-control-plane` такая структура уже
используется и лучше подходит для сопровождения DKP-модуля.

## Задача

Оформить полноценный public docs surface для `ai-models`:

- обновить overview;
- добавить guide для администраторов;
- добавить guide для пользователей;
- добавить CRD page;
- добавить examples;
- добавить FAQ;
- синхронизировать RU/EN документы;
- не менять skills/agents без отдельной governance-задачи.

## Scope

- `docs/README*.md`;
- `docs/ADMIN_GUIDE*.md`;
- `docs/USER_GUIDE*.md`;
- `docs/CR*.md`;
- `docs/EXAMPLES*.md`;
- `docs/FAQ*.md`;
- точечные ссылки в `docs/CONFIGURATION*.md`.

## Non-goals

- не менять `AGENTS.md`, `.codex/*`, `.agents/skills/*`, `.codex/agents/*`;
- не менять CRD/API;
- не менять templates или runtime behavior;
- не обещать Ollama publication как готовый путь, пока loader fail-closed.

## Критерии приёмки

- docs имеют DKP-разбивку по паттерну virtualization/gpu-control-plane;
- в admin docs есть минимальный `ModuleConfig`, secret contract, capacity,
  node-cache/SDS setup, observability и RBAC;
- в user docs есть `Model`, `ClusterModel`, upload, workload delivery,
  multi-model delivery и статусы;
- `CR*.md` остаются entrypoint для generated CRD docs по DKP-паттерну:
  только front matter и `<!-- SCHEMA -->`, без ручного дублирования схемы;
- examples дают применимые YAML snippets без ручного DMCR/materialize wiring;
- FAQ закрывает типовые вопросы по удалённым knobs, SDS, cache, capacity,
  Ollama/Diffusers и рестартам;
- `make verify` или релевантные docs/render checks проходят.

## Риски

- docs могут начать обещать ещё не реализованное поведение;
- дублирование с `CONFIGURATION` может создать второй source of truth.
