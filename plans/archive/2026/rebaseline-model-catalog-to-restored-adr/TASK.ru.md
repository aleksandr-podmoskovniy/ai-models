# Rebaseline Model Catalog To Restored ADR

## 1. Контекст

В `internal-docs` восстановлен прежний ADR по каталогу моделей:

- public contract ориентирован на `Model` / `ClusterModel`;
- first iteration в ADR — `artifact-first`, `OCI-first`;
- ключевые поля в духе ADR: `spec.artifact`, `spec.usagePolicy`,
  `spec.launchPolicy`, `status.resolved`, `status.conditions`;
- каталог должен быть границей для `ai-inference`, а не набором raw backend
  сущностей;
- внутренний backend остаётся internal component.

При этом текущая phase-2 ветка в `ai-models` уже успела уехать в другую
архитектуру:

- `source-oriented` public API вместо `artifact-first`;
- `HuggingFace | Upload | OCIArtifact` как часть public contract;
- backend-neutral `status.artifact` вместо ADR-shape `status.resolved`;
- design bundle про controller-owned publish orchestration, staging upload,
  runtime delivery и managed backend switching;
- контроллер как module runtime пока не доведён до cluster baseline.

Нужно жёстко сверить текущее состояние с восстановленным ADR, зафиксировать drift
и начать выравнивание implementation path без повторного самовольного
передизайна public contract.

## 2. Постановка задачи

Сделать rebaseline phase-2 относительно восстановленного ADR и при этом
удержать в поле зрения user-level target:

- controller должен запускаться в кластере как полноценный module component;
- `Model` / `ClusterModel` должны стать реальными CRD модуля;
- дальше нужно прийти к working publication flow, где модель можно загрузить,
  обогатить metadata/status и подготовить к использованию;
- приоритетный practical path для ближайших фич — HF-first;
- deletion lifecycle должен чистить внешние следы модели;
- для runtime consumption нужен отдельный materializer/agent path, в том числе
  для `KubeRay`.

Но важное ограничение:

- сначала source of truth — restored ADR;
- любые расхождения между user target и ADR должны быть явно зафиксированы как
  gap/следующий ADR-step, а не тихо прошиты в public API.

## 3. Scope

- audit текущего phase-2 кода и design bundle против restored ADR;
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `api/core/v1alpha1/*`
- `images/controller/internal/*`
- `images/controller/cmd/*`
- module rollout paths, если они не конфликтуют с restored ADR:
  - `templates/*`
  - `images/hooks/*`
  - `.werf/stages/*`
  - `openapi/*`

## 4. Non-goals

- Не пытаться в одном slice одновременно закрыть:
  - rebaseline public API;
  - working HF import;
  - MLflow/S3 publish;
  - OCI/KitOps publish;
  - KubeRay materializer runtime.
- Не переписывать restored ADR под текущее implementation drift.
- Не скрывать противоречия между ADR и желаемым future flow.

## 5. Ожидания пользователя, которые нужно держать как целевую траекторию

### Operational baseline

- controller развёрнут модулем в кластере;
- controller HA-ready;
- health/metrics/ServiceMonitor присутствуют;
- `Model` / `ClusterModel` реально устанавливаются как CRD модуля.

### Publication baseline

- через CR можно инициировать модель publication flow;
- в приоритете practical path для HF;
- модель попадает либо во внутренний backend (`mlflow` + `s3`), либо в OCI path
  (`kitops` + `payload-registry`);
- metadata рассчитывается и обогащает CR;
- status/conditions объясняют lifecycle;
- удаление CR чистит внешние следы модели.

### Consumption baseline

- модель может быть использована runtime consumers;
- для `KubeRay` нужен отдельный agent/materializer path с локальной подготовкой
  модели в PVC/shared storage;
- digest / sha verification допустимо отложить на следующий phase step, если
  base flow уже работает и risk явно зафиксирован.

## 6. Критерии приёмки для текущего task bundle

- Есть жёсткий audit matrix: restored ADR vs current code/design bundle vs user
  expectations.
- Для каждого существенного drift есть решение:
  - `keep for now but out of contract`;
  - `rework now`;
  - `defer behind explicit next-step`.
- Реализация следующего slice не противоречит restored ADR.
- Если runtime/module rollout можно безопасно делать независимо от public API
  rework, он должен быть выведен в отдельный bounded slice и реализован.

## 7. Основные риски

- Если снова смешать public API redesign и runtime rollout, получится ещё один
  архитектурный ком.
- Если попытаться притянуть user target без фиксации ADR gaps, drift повторится.
- Если сделать cluster runtime без ясного owner-моделя CRD/API, контроллер
  останется process без понятного контракта.
