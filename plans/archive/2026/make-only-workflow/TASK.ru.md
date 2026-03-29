# Перевести репозиторий на make-only workflow

## Контекст

В репозитории уже есть полноценный `Makefile`, через который идут все основные
проверки, сборка и локальные developer loops. При этом параллельно остаётся
`Taskfile` как второй entrypoint для тех же действий, а в части документации это
создаёт ненужную двусмысленность.

## Постановка задачи

Нужно закрепить `make` как единственный supported developer workflow для
проверок и локальных операций репозитория.

## Scope

- убрать `Taskfile`-based entrypoints из живой repo surface;
- удалить `Taskfile.yaml` и `Taskfile.init.yaml`, если они не нужны для CI или
  runtime;
- поправить живую документацию и policy так, чтобы она ссылалась только на
  `make`-команды.

## Non-goals

- не переписывать исторические task bundles в `plans/`;
- не менять сами команды `make`;
- не трогать CI, если он уже использует `make`.

## Затрагиваемые области

- `plans/make-only-workflow/`
- `AGENTS.md`
- `DEVELOPMENT.md`
- `Taskfile.yaml`
- `Taskfile.init.yaml`

## Критерии приёмки

- в живой repo surface для проверок и developer workflows используется только
  `make`;
- `Taskfile.yaml` и `Taskfile.init.yaml` удалены;
- `make verify` проходит.

## Риски

- если где-то скрыто используется `task`, удаление `Taskfile` может всплыть уже
  после cleanup;
- если оставить часть живых ссылок на `task`, workflow останется двусмысленным.
