# Ужесточить и упростить orchestration сабагентов в ai-models

## Контекст

В репозитории уже есть skills, agent profiles и описанная orchestration-модель,
но на практике сабагенты используются редко и непоследовательно:
- правила их вызова есть, но они слишком общие;
- не зафиксировано, когда delegation обязательна, а когда лишняя;
- связь между skills и agent roles недостаточно явная;
- development docs описывают workflow, но не дают короткого decision matrix для
  ежедневной работы.

Пользователь хочет понять, нужны ли сабагенты вообще, и если нужны, то
оптимизировать систему так, чтобы ею реально пользовались без лишних ошибок.

## Постановка задачи

Нужно выровнять repo rules, `.codex` context и development docs так, чтобы:
- было явно прописано, когда delegation обязательна;
- было явно прописано, когда сабагенты не нужны;
- orchestration была согласована между `AGENTS.md`, `.codex/README.md`,
  `CODEX_WORKFLOW.ru.md` и agent profiles;
- final review discipline не оставалась опциональной формальностью.

## Scope

- `plans/active/optimize-subagent-orchestration/`
- `AGENTS.md`
- `.codex/README.md`
- `.codex/config.toml`
- `.codex/agents/*.toml`
- `docs/development/CODEX_WORKFLOW.ru.md`
- при необходимости `docs/development/REVIEW_CHECKLIST.ru.md`

## Non-goals

- не вводить обязательную многопоточную оркестрацию для мелких задач;
- не создавать десятки новых agent profiles без реальной роли;
- не менять phase model или runtime architecture модуля;
- не привязывать workflow к специфике одной IDE.

## Критерии приёмки

- в repo есть явный decision matrix: когда использовать сабагентов, каких и в
  каком порядке;
- мелкие задачи имеют явно разрешённый no-delegation path;
- финальный review формализован как обязательный шаг для существенных задач;
- `AGENTS.md`, `.codex/README.md`, `CODEX_WORKFLOW.ru.md` и agent profiles не
  противоречат друг другу;
- `make verify` проходит.

## Риски

- можно переусложнить workflow и сделать его слишком тяжёлым для повседневной
  работы;
- можно оставить правила красивыми на бумаге, но неисполняемыми на практике;
- можно задокументировать orchestration, которая не соответствует реальным
  возможностям инструмента.
