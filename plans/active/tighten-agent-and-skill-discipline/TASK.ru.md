# Tighten Agent And Skill Discipline

## Контекст

В репозитории уже есть `AGENTS.md`, `.codex/README.md`, локальные skills в
`.agents/skills/*` и repo-local agent profiles в `.codex/agents/*`, но
фактическая работа всё ещё уезжает в несистемные повторы:

- planning и review правила размазаны между несколькими документами;
- skills местами слишком общие и допускают “почти подходит” вместо жёсткой
  границы применения;
- orchestration expectations для agents описаны неравномерно;
- нет одного короткого governance pass, который бы жёстко выравнивал skills,
  agent roles и review discipline друг с другом.
- отсутствует достаточно жёсткий reusable doctrine для:
  - boundary discipline
  - long-context resilience
  - systematic testing methodology
  - portability в другие DKP module repos

Пользователь явно требует поправить skills и agent guidance и отревьювить их
жёстко, потому что текущая работа недостаточно системна.

## Постановка задачи

Провести governance refactor локального workflow surface как reusable baseline
для DKP module repos:

- пересмотреть `AGENTS.md`, `.codex/README.md` и `.agents/skills/*`;
- пересмотреть `.codex/agents/*`;
- убрать противоречия, размытые места и слабые правила;
- усилить требования к planning, orchestration, review и bounded slices;
- зафиксировать системный engineering doctrine, который не теряется на длинном
  контексте и не зависит от текущего chat history;
- выровнять reusable core vs project overlays так, чтобы skills и agents можно
  было переносить в другие DKP module repos;
- оставить только те skills и agent expectations, которые реально помогают
  работать системно и воспроизводимо.

## Scope

- `AGENTS.md`
- `.codex/README.md`
- `.agents/skills/*`
- `.codex/agents/*`
- `docs/development/CODEX_WORKFLOW.ru.md`
- `docs/development/TASK_TEMPLATE.ru.md`
- `docs/development/REVIEW_CHECKLIST.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `plans/README.md`
- task bundle для этой задачи

## Non-goals

- не менять product/runtime/API code модуля;
- не редизайнить весь Codex platform behavior за пределами repo-local rules;
- не плодить новые skills/agent roles без необходимости, если проблему можно решить
  tightening existing ones.

## Затрагиваемые области

- `AGENTS.md`
- `.codex/README.md`
- `.agents/skills/*.md`
- `.codex/agents/*.toml`
- `docs/development/*.md`
- `plans/README.md`
- `plans/active/tighten-agent-and-skill-discipline/*`

## Критерии приёмки

- правила в `AGENTS.md`, `.codex/README.md` и relevant skills больше не
  противоречат друг другу;
- orchestration expectations стали жёстче и конкретнее:
  - когда planning обязателен
  - когда delegation обязателен
  - когда review обязателен
  - как фиксировать findings в bundle
- reusable doctrine для engineering/systematic testing/long-context hygiene
  стал явным и переносимым;
- skills не дублируют друг друга без явного split of responsibility;
- agent profiles не противоречат skills и не маскируют governance work как
  incidental wording;
- есть machine-checkable inventory/lint для ключевых governance invariants,
  а не только manual reread;
- `docs/development` больше не расходится с `AGENTS.md`, `.codex/README.md`
  и reusable workflow doctrine;
- есть финальный жёсткий review по самим инструкциям, а не только summary.

## Риски

- можно усилить правила настолько, что workflow станет непрактичным;
- можно оставить дублирующие instructions и только увеличить объём текста;
- можно поправить wording, но не закрыть реальные причины несистемной работы.
