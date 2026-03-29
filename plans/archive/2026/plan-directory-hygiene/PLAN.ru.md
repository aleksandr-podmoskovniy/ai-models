# План работ: plan directory hygiene

## Current phase

Этап 1: внутренний managed backend inside the `ai-models` module.

## Slice 1. Зафиксировать новую структуру plans/

### Цель

Описать явную policy для active/archive bundles и определить, какие каталоги надо
перенести из корня `plans/`.

### Изменяемые области

- `plans/active/plan-directory-hygiene/`

### Проверки

- поиск по repo на `plans/<slug>` и `plans/`
- инвентаризация текущего содержимого `plans/`

### Артефакт

Понятен целевой layout и migration scope.

## Slice 2. Перенести bundles и обновить policy/docs

### Цель

Сделать `plans/` устойчивым каталогом: активные bundles отдельно, архив отдельно,
живые workflow-ссылки обновлены.

### Изменяемые области

- `plans/`
- `AGENTS.md`
- `DEVELOPMENT.md`
- `README*.md`
- `docs/development/`
- `.agents/skills/`
- `.codex/agents/task-framer.toml`

### Проверки

- поиск по repo на `plans/<slug>`
- проверка содержимого `plans/`

### Артефакт

Живой workflow переведён на `plans/active/<slug>/`, исторические bundles убраны
из корня каталога.

## Rollback point

После Slice 1. Новая policy уже определена, но migration ещё не начат.

## Final validation

- `make verify`
