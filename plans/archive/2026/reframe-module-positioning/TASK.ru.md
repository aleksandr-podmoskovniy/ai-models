# Переписать user-facing позиционирование модуля ai-models

## Контекст

В репозитории уже есть рабочий DKP module root, phase-1 конфигурация managed `MLflow`, build shell и документация по этапам разработки. При этом часть user-facing текстов до сих пор описывает `ai-models` как `shell`, `foundation` или `in progress`, что делает модуль временной заготовкой даже там, где нужно уже фиксировать нормальный продуктовый контракт.

## Постановка задачи

Нужно переписать публичные описания модуля так, чтобы `ai-models` везде читался как конечный DKP-модуль для registry и каталога AI/ML-моделей, а текущий phase-1 managed `MLflow` объяснялся как текущая runtime-реализация модуля, а не как "временная основа" или "bootstrap shell".

## Scope

- обновить module metadata и disable message;
- переписать top-level README и docs overview/configuration;
- выровнять OSS/module copy и OpenAPI description под один product-level narrative;
- сохранить честное указание на текущий phase-1 scope без обещания уже готовых `Model` / `ClusterModel`.

## Non-goals

- не менять архитектурные этапы из `docs/development/*`;
- не менять значения OpenAPI schema, templates или runtime behavior;
- не переводить модуль из `Preview` в другой stage;
- не добавлять phase-2 API раньше реальной реализации.

## Затрагиваемые области

- `module.yaml`
- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `openapi/config-values.yaml`
- `oss.yaml`

## Критерии приёмки

- в user-facing текстах модуль больше не называется `shell`, `foundation` или `in progress`;
- `ai-models` описан как DKP-модуль для реестра и каталога AI/ML-моделей;
- current phase объясняется как текущая runtime-реализация модуля на базе managed `MLflow`;
- wording согласован между `module.yaml`, README, docs и OpenAPI description;
- repo-level проверки проходят.

## Риски

- если переписать тексты слишком "финально", можно создать ложное впечатление, что `Model` / `ClusterModel` уже доступны;
- если оставить слишком много phase wording, модуль снова будет выглядеть как временная заготовка.
