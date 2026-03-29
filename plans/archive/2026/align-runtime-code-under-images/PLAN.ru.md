# План работ: align runtime code under images

## Current phase

Этап 1: внутренний managed backend как компонент модуля `ai-models`.

## Slice 1. Зафиксировать целевой runtime layout

### Цель

Снять референсы из `virtualization` и `gpu-control-plane` и определить
канонический контракт: `api/` для platform API, `images/*` для executable
runtime code.

### Изменяемые области

- `plans/active/align-runtime-code-under-images/`

### Проверки

- чтение reference repo layouts
- инвентаризация текущих `images/`, `hooks/`, `controllers/`

### Артефакт

Понятно, какие каталоги остаются top-level, а какие должны жить под `images/*`.

## Slice 2. Перевести hooks и controller layout

### Цель

Убрать неверный structural contract и согласовать фактический кодовый layout с
reference-модулями.

### Изменяемые области

- `images/`
- `hooks/`
- `.werf/stages/`
- `werf.yaml`
- `Makefile`

### Проверки

- `make test`
- `make helm-template`

### Артефакт

Go hooks code лежит под `images/hooks`, а `images/controller/` зафиксирован как
будущий корень controller executable code.

## Slice 3. Выровнять docs и repo guidance

### Цель

Закрепить новый contract в repo docs и не оставлять старую guidance про
`controllers/` как кодовый корень.

### Изменяемые области

- `docs/development/`
- `README*.md`
- `AGENTS.md`

### Проверки

- `make verify`

### Артефакт

Repo docs и workflow guidance согласованы с layout `api/` + `images/*`.

## Rollback point

После Slice 1. Целевой contract определён, но физический перенос кода ещё не
начат.

## Final validation

- `make verify`
