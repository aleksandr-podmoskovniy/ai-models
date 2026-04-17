# Техническое задание: модуль ai-models

## Цель

Сделать модуль DKP `ai-models`, который добавляет платформе capability
"единый каталог опубликованных моделей" и держит controller-owned publication,
distribution и runtime delivery path без зависимости на исторический
backend shell.

## Продуктовый результат

В результате платформа должна получить:
- `Model` и `ClusterModel` как platform-facing contract;
- единый путь публикации моделей;
- канонические OCI `ModelPack` artifacts во внутреннем `DMCR`;
- возможность принимать результаты из JupyterHub, Airflow и других training/experiment контуров;
- устойчивую основу для дальнейшей интеграции с `ai-inference`.

## Архитектурная рамка

- `ai-models` — модуль платформы, а не namespaced application для команд.
- Публичный контракт платформы — `Model` и `ClusterModel`.
- Канонический publication contract — controller-owned OCI artifact flow.
- Исторический backend не должен оставаться архитектурным центром
  или user-facing baseline.
- Любой пользовательский UX поверх моделей должен строиться через DKP API,
  runtime delivery и catalog semantics, а не через прямую зависимость от
  старого backend UI/API.

## Этапы

### Этап 1. Phase reset к ai-models-owned publication baseline

Нужно получить рабочий модуль, который:
- публикует модели через controller-owned path;
- хранит канонические OCI artifacts во внутреннем `DMCR`;
- оформлен в DKP-стиле: module values, OpenAPI, templates, Helm library, werf;
- не зависит от live historical-backend runtime shell или его auth/workspace
  semantics.

На этом этапе не нужно пытаться завершить весь runtime distribution stack.
Главная цель — получить чистый и рабочий publication baseline без legacy
backend центра.

### Этап 2. Distribution и runtime topology

Нужно добавить:
- `DMZ` registry scenario;
- node-local cache / mount delivery;
- status/conditions и observability для distribution/runtime topology;
- bounded large-model UX.

### Этап 3. Упрочнение

Нужно добавить:
- hardening и supply-chain практики;
- controlled patching/rebasing remaining runtime code;
- distroless для собственного кода;
- дополнительные security checks и quality gates.

## Что важно не делать

- Не держать dual-stack fallback на historical backend или `KitOps` после
  landing ai-models-owned replacement.
- Не пытаться в первом же шаге сделать весь `DMZ` / node-cache / lazy-loading
  UX в одном diff.
- Не смешивать public model contract с backend/runtime implementation knobs.
- Не переходить к тяжёлому hardening, пока нет работающего publication baseline.

## Требования к качеству

- Каждая задача проходит через task bundle.
- Каждый этап имеет понятный working baseline.
- Изменения укладываются в текущий этап и не тянут преждевременно будущие части решения.
- Модуль остаётся поддерживаемым и понятным без знания внутренних обсуждений.
- Любой новый крупный шаг сопровождается актуализацией docs/development.
