# План работ: realign module layout and CI

## Current phase

Этап 1: внутренний managed backend inside the `ai-models` module.

## Slice 1. Зафиксировать целевой layout

### Цель

Снять референсы из `virtualization`, определить целевые границы `templates/`,
`api/`, `controllers/`, `hooks/` и CI shell для `ai-models`.

### Изменяемые области

- `plans/active/realign-module-layout-and-ci/`

### Проверки

- инвентаризация текущего repo layout
- чтение референсов из `virtualization`

### Артефакт

Понятен целевой layout без patchwork-решений.

## Slice 2. Перестроить repo layout и templates

### Цель

Сделать layout чище и подготовить его к backend/controller development.

### Изменяемые области

- `templates/`
- `api/`
- `controllers/`
- `docs/development/`
- `README*.md`
- `AGENTS.md`
- `DEVELOPMENT.md`

### Проверки

- `make helm-template`

### Артефакт

Component-oriented template layout и зафиксированные repo boundaries.

## Slice 3. Выровнять GitHub и GitLab CI shell

### Цель

Сделать CI naming и workflow structure ближе к DKP module pattern и убрать вид
временной обвязки, а GitHub Actions выровнять по референсу
`gpu-control-plane`.

### Изменяемые области

- `.github/workflows/`
- `.gitlab-ci.yml`

### Проверки

- `make verify`

### Артефакт

GitHub и GitLab CI выглядят как системный module shell, а не как случайный
набор workflow/job файлов; GitHub использует pair `build.yaml` / `deploy.yaml`
по образцу `gpu-control-plane`.

## Rollback point

После Slice 1. Целевой layout и CI scope уже понятны, но физический перенос
файлов ещё не начат.

## Final validation

- `make verify`
