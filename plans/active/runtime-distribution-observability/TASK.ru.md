# Runtime/distribution observability continuation

## 1. Заголовок

Первый production-grade observability slice для managed node-cache runtime
plane

## 2. Контекст

В live phase-2 baseline уже landed:

- controller-owned publication в native OCI `ModelPack`;
- managed node-cache substrate;
- stable per-node `node-cache-runtime` Pod + PVC;
- bounded runtime-side desired-artifact extraction from live managed `Pod`.

Но observability surface всё ещё асимметрична:

- module уже экспортирует public catalog metrics по `Model` / `ClusterModel`;
- при этом отдельный shared runtime plane для node-local cache почти не даёт
  machine-readable health signal;
- оператору приходится идти в raw `Pod` / `PVC`, вместо того чтобы видеть
  bounded module-owned runtime metrics.

Пользователь требует продолжать проект как prod-ready module, а не оставлять
новый runtime plane без эксплуатационного сигнала.

## 3. Постановка задачи

Нужно добавить первый bounded observability cut поверх уже landed
`node-cache-runtime` plane:

- ввести отдельную monitoring boundary для runtime-plane health, а не
  раздувать `catalogmetrics`;
- экспортировать Prometheus metrics только по managed `node-cache-runtime`
  ресурсам;
- зафиксировать минимальный, defendable health contract для:
  - desired selected nodes vs managed/ready runtime resources;
  - runtime `Pod` phase/readiness;
  - shared cache `PVC` bind state;
  - requested shared-cache volume size;
- подключить collector в controller bootstrap и синхронизировать runtime docs.

## 4. Scope

- новый compact bundle `plans/active/runtime-distribution-observability/*`;
- `images/controller/internal/monitoring/*`;
- `images/controller/internal/bootstrap/*`;
- `images/controller/README.md`;
- `images/controller/STRUCTURE.ru.md`;
- `images/controller/TEST_EVIDENCE.ru.md`.

## 5. Non-goals

- не вводить сейчас `DMZ` distribution lag metrics;
- не добавлять новый public status/condition в `Model` / `ClusterModel`;
- не тянуть runtime-local counters из `node-cache-runtime` process logs;
- не вводить alerting rules, Grafana dashboards или новый monitoring API;
- не смешивать public catalog metrics и runtime-plane health в один collector
  package.

## 6. Затрагиваемые области

- controller monitoring boundary;
- bootstrap registration of module-owned collectors;
- docs/evidence around current runtime ownership shell.

## 7. Критерии приёмки

- Появляется отдельная monitoring boundary для runtime-plane health, а не ещё
  один mixed collector inside `catalogmetrics`.
- Collector читает только managed `node-cache-runtime` `Pod` / `PVC` по
  ai-models-owned labels внутри runtime namespace и не притворяется source of
  truth для всего cluster storage/runtime.
- Экспортируются проверяемые metrics для:
  - desired selected nodes;
  - managed/ready runtime `Pod` count;
  - managed/bound runtime `PVC` count;
  - `Pod` phase;
  - `Pod` readiness;
  - `PVC` bind state;
  - requested shared-cache volume size.
- Bootstrap регистрирует новый collector рядом с catalog metrics без
  architectural drift.
- `images/controller` docs/evidence описывают новый live observability seam.
- Проходят:
  - `cd images/controller && go test ./internal/monitoring/... ./internal/bootstrap`
  - `make verify`
  - `git diff --check`

## 8. Риски

- можно засунуть runtime health в `catalogmetrics` и снова смешать public object
  truth с internal runtime plane;
- можно начать экспортировать слишком много storage/runtime detail и раздуть
  observability contract быстрее, чем runtime maturity;
- можно завязать metrics на fragile naming вместо module-owned labels и
  получить brittle collector.
