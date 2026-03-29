# План работ: align base_images and agent ecosystem

## Current phase

Этап 1: managed MLflow inside the DKP module.

## Slice 1. Собрать референсы и целевой baseline

### Цель

Понять, что именно в `virtualization` является source-of-truth для `base_images`, и какие роли/skills в `ai-models` действительно нужны как reusable baseline.

### Изменяемые области

- `plans/align-base-images-and-agent-ecosystem/`

### Проверки

- ручная сверка `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- ручная сверка `.agents/` и `.codex/`
- read-only subagent review по обоим направлениям

### Артефакт

Есть понятный целевой baseline по base images и ecosystem skills/subagents.

## Slice 2. Перевести base_images на DKP pattern

### Цель

Использовать полный Deckhouse image map как источник правды и сохранить автоматический отсев только реально используемых образов во время `werf` build.

### Изменяемые области

- `base_images.yml` или новый каталог с full image map
- `.werf/stages/base-images.yaml`
- `werf.yaml`
- при необходимости docs вокруг build baseline

### Проверки

- `make helm-template`
- `make verify`

### Артефакт

Base images wired по DKP pattern без локальной ручной урезки source-of-truth.

## Slice 3. Систематизировать skills/subagents

### Цель

Свести repo-local ecosystem к более объективному и reusable baseline для новых DKP modules.

### Изменяемые области

- `.agents/skills/`
- `.codex/agents/`
- `.codex/README.md`
- при необходимости `AGENTS.md`

### Проверки

- ручная сверка текстов и ролей
- `make verify`

### Артефакт

Есть более чистый, понятный и reuse-friendly набор skills/subagents.

## Rollback point

После Slice 1. Целевой baseline уже понятен, но рабочее дерево ещё можно вернуть без structural changes.

## Final validation

- `make verify`
