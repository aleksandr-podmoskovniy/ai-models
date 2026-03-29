# Этап 1: поднять MLflow как внутренний managed backend модуля ai-models

## Контекст

Репозиторий уже оформлен как DKP module shell, но в нём ещё нет рабочего backend для каталога моделей. Следующий необходимый шаг — получить рабочий MLflow внутри модуля, подключённый к PostgreSQL и S3-compatible storage, чтобы затем уже строить поверх него `Model` и `ClusterModel`.

## Постановка задачи

Нужно сделать первый рабочий этап модуля `ai-models`: развернуть MLflow как внутренний managed компонент модуля в DKP-манере, подключить его к зависимостям платформы и обеспечить базовую эксплуатацию.

## Scope

- values и OpenAPI для настроек MLflow backend;
- templates и wiring в стиле DKP module;
- подключение PostgreSQL и artifact storage;
- рабочий Service/Ingress/UI;
- базовые monitoring/logging integration points;
- воспроизводимая сборка и deploy path.

## Non-goals

- `Model` / `ClusterModel` и их контроллеры;
- publish flow и синхронизация с MLflow API;
- distroless и глубокий patching MLflow;
- inference integration.

## Затрагиваемые области

- `module.yaml`
- `openapi/`
- `templates/`
- `images/mlflow/`
- `werf.yaml`, `base_images.yml`
- `docs/` и `docs/development/`
- CI/verify scripts при необходимости

## Критерии приёмки

- модуль рендерится и проходит валидацию шаблонов;
- MLflow deployment shape описан и воспроизводим;
- PostgreSQL и artifact storage wiring задокументированы и включены в конфигурацию модуля;
- есть понятный путь проверки доступности UI;
- есть базовая observability story;
- этап можно развивать дальше без слома структуры репозитория.

## Риски

- попытка тащить сразу весь каталог моделей;
- смешение upstream baseline и DKP-style шаблонов в одном хаотичном шаге;
- неочевидная граница между временными решениями и целевой архитектурой.
