# Rebuild Controller Architecture And Publication Flow

## Контекст

В репозитории уже есть phase-2 baseline для `Model` / `ClusterModel`, CRD, module shell для controller и первый live `HuggingFace -> mlflow` путь. Но текущая структура controller drift-нула от ранее согласованных паттернов:

- public controller слишком много знает про execution details;
- import/publish/cleanup job boundaries смешаны;
- result handoff сейчас завязан на pod termination message;
- virtualization-style controller-owned auth/access flow ещё не реализован;
- dual backend path (`mlflow`/object storage и `payload-registry`/OCI) пока не оформлен как стабильный internal contract.

Пользовательский ожидаемый сценарий уже зафиксирован:

1. Пользователь выбирает `spec.source` по аналогии с virtualization (`HuggingFace`, `Upload`, `HTTP`).
2. Модуль публикует модель во внутренний artifact plane: либо object storage через internal backend, либо OCI в payload-registry.
3. Controller считает technical profile и обогащает `status`.
4. `Model` / `ClusterModel` доходят до `Ready`.
5. Runtime-потребитель работает через local materialization в PVC/shared volume.
6. Удаление CR должно приводить к cleanup сохранённого артефакта.

## Постановка задачи

Исправить структуру controller так, чтобы она соответствовала agreed production-ready direction, и продолжить реализацию publication flow поверх новой структуры, не ломая public DKP contract.

## Scope

- Переложить controller на явные bounded responsibilities:
  - lifecycle/status owner;
  - publication operation / worker boundary;
  - artifact inspection boundary;
  - delete cleanup owner.
- Сохранить `Model` / `ClusterModel` как public contract с `spec.source` и `status.artifact` / `status.resolved`.
- Подготовить internal contracts для двух artifact planes:
  - object storage через internal backend;
  - OCI через payload-registry.
- Подготовить controller-owned access/auth shape по аналогии с virtualization.
- Продвинуть live implementation минимум до рабочего `source -> published artifact -> resolved profile -> ready` baseline без дальнейшего усложнения public API.

## Non-goals

- Не реализовывать в этой задаче полный runtime-agent для `ai-inference` / `KubeRay`.
- Не делать полноценную security-hardening фазу с digest/signature verification.
- Не переизобретать phase-1 backend deployment shape.
- Не вводить наружу raw MLflow сущности как public UX.
- Не завершать в одном slice все source variants и все backends сразу, если это ломает архитектурную чистоту.

## Затрагиваемые области

- `api/core/v1alpha1/*`
- `crds/*`
- `images/controller/internal/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/controller/*`
- `docs/CONFIGURATION*`
- `docs/development/*` при необходимости
- `plans/active/rebuild-controller-architecture-and-publication-flow/*`

## Критерии приёмки

- Есть task bundle с зафиксированным corrective plan.
- Controller boundary больше не строится вокруг одного fat reconciler-а.
- Execution details publication flow вынесены из status-owning reconciler-а в отдельный bounded layer.
- Public `Model` / `ClusterModel` сохраняют source-first contract.
- Есть рабочий baseline хотя бы для одного source path, который не опирается на brittle architecture shortcuts.
- Удаление опубликованной модели остаётся controller-owned и не теряет cleanup semantics.
- Module shell controller остаётся HA/metrics/health ready.

## Риски

- Можно застрять в частичном rework и временно ухудшить уже работающий happy-path.
- Слишком раннее объединение `mlflow` и `payload-registry` execution paths может размыть architecture boundary.
- Попытка сразу сделать runtime materializer и publish controller в одном bundle приведёт к новому drift.
