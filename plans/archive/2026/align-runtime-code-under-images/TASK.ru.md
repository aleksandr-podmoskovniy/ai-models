# Перевести runtime code layout под images/*

## Контекст

Текущий `ai-models` уже использует `images/backend/` для внутреннего backend,
но repo layout всё ещё расходится с паттернами `virtualization` и
`gpu-control-plane`:
- top-level `controllers/` создан как отдельный корень, хотя исполняемый
  controller code в DKP-модулях должен жить под `images/*`;
- Go code hooks лежит в `hooks/batch`, тогда как reference-модули держат такой
  код под `images/hooks` и импортируют артефакт в bundle;
- development docs пока описывают `controllers/` как кодовый корень и тем самым
  закрепляют неверный structural contract.

Пользователь явно требует перестроить layout так, чтобы controller был в
`images/` по аналогии с `virtualization` и `gpu-control-plane`.

## Постановка задачи

Нужно перестроить layout runtime code в модуле так, чтобы:
- top-level `api/` оставался только для будущего публичного DKP API;
- исполняемый controller code размещался под `images/controller/`;
- Go hooks code размещался под `images/hooks/`;
- `werf`, `make`, docs и repo guidance были согласованы с этим layout;
- не появилось фейковых runtime компонентов или незадействованных placeholder
  build stages.

## Scope

- `plans/active/align-runtime-code-under-images/`
- `images/`
- `hooks/`
- `.werf/stages/`
- `werf.yaml`
- `Makefile`
- `docs/development/`
- `README*.md`
- `AGENTS.md`

## Non-goals

- не реализовывать реальный phase-2 controller logic;
- не добавлять CRD/API generation раньше времени;
- не менять runtime semantics внутреннего backend;
- не добавлять лишние образные стадии без фактической надобности.

## Критерии приёмки

- top-level `controllers/` больше не используется как кодовый корень;
- в repo есть `images/controller/` как каноническое место для будущего
  controller executable code;
- Go hooks code перенесён под `images/hooks/`, а bundle/import path работает;
- docs явно фиксируют, что executable runtime code живёт под `images/*`;
- `make verify` проходит.

## Риски

- можно сломать werf import/build path для go hooks;
- можно оставить старые пути в docs и Makefile;
- можно создать пустой controller skeleton, который будет выглядеть как фейковая
  реализация вместо structural contract.
