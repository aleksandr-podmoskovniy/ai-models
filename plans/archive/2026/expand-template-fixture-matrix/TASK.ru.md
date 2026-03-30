# Расширить fixture matrix для template rendering и kubeconform

## Контекст

Сейчас `make helm-template` и `make kubeconform` проверяют только один
happy-path render из `fixtures/module-values.yaml`. Это подтверждает базовый
render path, но не даёт достаточного coverage для phase-1 DKP module:
managed/external PostgreSQL, inline artifacts credentials,
global HA и auth/https combinations.

## Постановка задачи

Нужно превратить template checks в небольшой fixture matrix:
- выделить несколько render scenarios;
- рендерить каждый сценарий в отдельный `helm-template-*.yaml`;
- прогонять `kubeconform` по всему набору render outputs;
- по возможности использовать fixtures так, чтобы проверялись и repo defaults,
  а не только полностью задублированные explicit values.

## Scope

- `plans/active/expand-template-fixture-matrix/`
- `fixtures/*`
- `tools/helm-tests/helm-template.sh`
- при необходимости `DEVELOPMENT.md`

## Non-goals

- не менять runtime behavior модуля;
- не проектировать e2e tests;
- не добавлять отдельные CI jobs вне текущего `make verify` loop.

## Затрагиваемые области

- render fixtures;
- local render/kubeconform tooling;
- developer workflow docs.

## Критерии приёмки

- есть несколько fixture scenarios для ключевых phase-1 combinations;
- `make helm-template` рендерит каждый сценарий в отдельный
  `tools/kubeconform/renders/helm-template-*.yaml`;
- `make kubeconform` валидирует весь набор render outputs без доп. ручных шагов;
- `make verify` проходит;
- fixture matrix покрывает минимум:
  - managed PostgreSQL + inline artifacts credentials;
  - managed PostgreSQL + virtual-hosted S3 addressing;
  - external PostgreSQL + inline artifacts credentials;
  - HA-enabled render;
  - Dex/HTTPS-enabled render.

## Риски

- легко сделать слишком хрупкие fixtures, если в них дублировать все defaults;
- неаккуратный naming scenario files ухудшит читаемость render outputs;
- если рендер-сценарии будут неортогональны, coverage станет шумным, но не
  полезным.
