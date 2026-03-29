# Выравнивание phase-1 shell для managed MLflow в модуле ai-models

## Контекст

Репозиторий уже систематизирован по фазам и правилам работы, но текущая module shell обвязка ещё частично отражает будущий этап с `Model` / `ClusterModel`, а не реальный stage 1 baseline. Перед переносом сборки и патчей `MLflow` нужно привести в согласованное состояние docs, metadata, OpenAPI/values и минимальные templates/verify-правила.

## Постановка задачи

Нужно выровнять текущую обвязку `ai-models` под этап "managed MLflow inside the module" и подготовить устойчивый shell для последующего переноса сборки `MLflow` и runtime wiring по паттернам из `n8n-d8`.

## Scope

- привести root/docs/module metadata к stage 1 формулировкам;
- ввести user-facing `config-values` для managed `MLflow`, storage, ingress/https и auth baseline;
- отделить runtime/internal values от user-facing schema в `openapi/values.yaml`;
- убрать фейковый registry secret flow и заменить его на реальный fallback к `global.modulesImages.registry.dockercfg`;
- подтянуть локальные verify-правила к текущему shell, чтобы docs и templates проверялись согласованно.

## Non-goals

- переносить сам upstream `MLflow` код, Dockerfile или patches;
- собирать рабочий `Deployment` / `Service` / `Ingress` для `MLflow`;
- вводить `Model`, `ClusterModel`, контроллеры или publish flow;
- делать hardening, distroless или deep patching.

## Затрагиваемые области

- `plans/align-managed-mlflow-shell/`
- `README*`, `docs/`, `module.yaml`
- `openapi/`
- `templates/`
- `fixtures/`
- `Makefile`

## Критерии приёмки

- docs и metadata описывают stage 1 как managed `MLflow`, а не текущий публичный catalog API;
- `config-values.yaml` и `values.yaml` задают понятный baseline для `MLflow` bootstrap по аналогии с `n8n-d8`;
- registry secret больше не рендерится с фейковым `.dockerconfigjson`;
- локальные команды проверки не расходятся с тем, что реально проверяется в shell;
- `make lint`, `make helm-template`, `make kubeconform` и `make verify` проходят.

## Риски

- слишком рано зафиксировать values, которые потом окажутся неудобны для реального `MLflow` runtime;
- случайно протащить в stage 1 semantics будущего catalog API;
- сделать values/OpenAPI слишком абстрактными и не пригодными для следующего шага с переносом сборки.
