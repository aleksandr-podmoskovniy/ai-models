# План работ: optimize subagent orchestration

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Зафиксировать пробелы в текущей orchestration-модели

### Цель

Понять, почему при наличии skills и agent profiles сабагенты не становятся
естественной частью рабочего цикла.

### Изменяемые области

- `plans/active/optimize-subagent-orchestration/`

### Проверки

- чтение `AGENTS.md`, `.codex/README.md`, `CODEX_WORKFLOW.ru.md`,
  `.codex/agents/*.toml`
- read-only review через subagents

### Артефакт

Понятны конкретные gaps: trigger rules, mandatory delegation, no-delegation
path, final review discipline, mapping skills-to-agents.

## Slice 2. Выровнять repo rules и Codex context

### Цель

Сделать orchestration policy короткой, явной и согласованной между основными
repo-facing документами.

### Изменяемые области

- `AGENTS.md`
- `.codex/README.md`
- `.codex/config.toml`
- `.codex/agents/*.toml`

### Проверки

- точечная сверка текстов и role matrix

### Артефакт

Репозиторий явно задаёт, когда вызывать сабагентов и когда этого не делать.

## Slice 3. Выровнять development workflow docs и финальный review gate

### Цель

Превратить orchestration из "общего совета" в рабочий процесс для ежедневных
задач.

### Изменяемые области

- `docs/development/CODEX_WORKFLOW.ru.md`
- при необходимости `docs/development/REVIEW_CHECKLIST.ru.md`

### Проверки

- `make verify`

### Артефакт

Codex workflow и review discipline согласованы с repo rules и пригодны для
практической работы.

## Rollback point

После Slice 1. Пробелы и решения понятны, но документы ещё не менялись.

## Final validation

- `make verify`
