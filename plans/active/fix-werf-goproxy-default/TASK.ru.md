# Исправить дефолт GOPROXY для локального werf build

## Контекст

После снятия giterminism blockers локальный `werf build --dev` доходит до
`go-hooks-artifact/install`, но падает из-за secret `GOPROXY` со значением
`<no value>`. Это ломает `go mod download` даже при корректном git-backed tree.

## Постановка задачи

Нужно задать нормальный reproducible default для `GOPROXY` в `werf` config,
чтобы локальный build не требовал обязательного внешнего env и не передавал
`<no value>` в build secrets.

## Scope

- `plans/active/fix-werf-goproxy-default/`
- `werf.yaml`
- при необходимости `images/**/werf.inc.yaml`
- при необходимости `DEVELOPMENT.md`

## Non-goals

- не менять build graph модуля;
- не убирать secret-механизм для `GOPROXY`, если он ещё нужен в CI;
- не чинить следующие возможные build blockers заранее.

## Критерии приёмки

- `GOPROXY` получает корректный default для local/dev build;
- `go-hooks-artifact/install` больше не падает на `invalid proxy URL missing scheme`;
- `make verify` проходит.

## Риски

- можно выбрать неподходящий default для CI или закрытых сетей;
- можно разойтись с паттернами reference-модулей.
