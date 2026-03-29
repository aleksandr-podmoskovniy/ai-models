# Навести порядок в plans/ и ввести active/archive policy

## Контекст

Сейчас `plans/` используется как обязательный workflow-след для нетривиальных
задач, но все bundles складываются прямо в корень каталога. Из-за этого папка
неограниченно растёт, в ней уже лежат завершённые bundles за предыдущие задачи,
а policy по архивированию отсутствует.

При этом важно сохранить две вещи:
- task bundles должны оставаться в git как инженерный след и контекст решений;
- bundles не должны попадать в module bundle/release path.

## Постановка задачи

Нужно превратить `plans/` из неструктурированного каталога в управляемую
иерархию с активными и архивными bundles.

## Scope

- ввести структуру `plans/active/` и `plans/archive/<year>/`;
- перенести текущие завершённые bundles из корня `plans/` в архив;
- перевести живую workflow-документацию и skill guidance на
  `plans/active/<slug>/`;
- удалить мусор вроде `plans/.DS_Store`.

## Non-goals

- не переписывать содержимое исторических bundles без необходимости;
- не убирать `plans/` из git;
- не включать `plans/` в module bundle/release artifacts.

## Затрагиваемые области

- `plans/`
- `AGENTS.md`
- `DEVELOPMENT.md`
- `README*.md`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `.agents/skills/`
- `.codex/agents/task-framer.toml`

## Критерии приёмки

- корень `plans/` содержит только `README.md`, `active/` и `archive/`;
- новые задачи направляются в `plans/active/<slug>/`;
- завершённые bundles перенесены в архив и больше не захламляют корень;
- `plans/` по-прежнему не попадает в module bundle;
- `make verify` проходит.

## Риски

- можно оставить часть живых ссылок на старый путь `plans/<slug>/`;
- можно случайно переместить текущий активный bundle не туда;
- можно сломать навигацию, если policy не будет явно описана в docs.
