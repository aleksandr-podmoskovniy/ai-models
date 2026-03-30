# Исправить giterminism allowlist для werf в GitHub CI

## Контекст

GitHub Actions build падает на стадии `deckhouse/modules-actions/build@v4`, потому
что `werf` запрещает чтение environment variable
`WERF_ENABLE_HOST_GO_PKG_CACHE` из template logic в `images/hooks/werf.inc.yaml`.
Это чистый giterminism/config issue: CI не доходит до фактической сборки
модуля.

## Постановка задачи

Нужно привести `werf-giterminism.yaml` в соответствие с фактическими
`env(...)`-вызовами в текущем `werf` config, чтобы GitHub build мог загрузить и
отрендерить конфигурацию без нарушения reproducibility contract.

## Scope

- `plans/active/fix-werf-giterminism-env-allowlist/`
- `werf-giterminism.yaml`
- при необходимости `images/**/werf.inc.yaml` или `.werf/stages/*.yaml`

## Non-goals

- не менять runtime semantics образов;
- не перестраивать CI shell;
- не чинить следующие возможные сбои, пока не пройден config-loading stage.

## Критерии приёмки

- все `env(...)` из текущего werf config либо разрешены в giterminism, либо
  убраны из template logic;
- локальный YAML/config sanity check проходит;
- `make verify` проходит.

## Риски

- можно разрешить слишком широкий набор env variables;
- можно оставить следующий скрытый blocker после первого исправления.
