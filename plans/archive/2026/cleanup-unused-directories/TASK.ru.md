# Почистить лишние каталоги после structural reshuffle

## Контекст

После последних перестроений layout в репозитории остались каталоги, которые
больше не входят в текущий structural contract:
- пустые top-level `controllers/` и `hooks/`;
- пустые legacy skill directories;
- завершённые bundles в `plans/active/`, которые уже не являются активными.

Пользователь просит убрать ненужные папки и не оставлять structural мусор в
production-репозитории.

## Постановка задачи

Нужно удалить или перенести только те каталоги, которые действительно больше не
нужны текущему repo contract, без затрагивания локальных tool/cache директорий
и без ломки `make verify`.

## Scope

- `plans/active/cleanup-unused-directories/`
- `.agents/skills/`
- `controllers/`
- `hooks/`
- `plans/active/`
- `plans/archive/2026/`

## Non-goals

- не чистить пользовательские локальные cache/tool directories вроде `.cache/`
  и `.bin/`;
- не удалять каталоги, которые участвуют в `werf`, `make` или docs contract;
- не переписывать runtime layout заново.

## Критерии приёмки

- empty structural directories, не входящие в current contract, удалены;
- завершённые bundles больше не висят в `plans/active/`;
- `make verify` проходит.

## Риски

- можно удалить каталог, который ещё упоминается в docs или workflow;
- можно перепутать active и archive bundle state.
