# Rebaseline Publication Plane To Backend Artifact Plane

## Контекст

Текущий phase-2 controller drift-нул в сторону двух artifact planes:

- `payload-registry` как OCI publish plane;
- `mlflow` как live publication backend для `HuggingFace` и `HTTP`.

Это больше не соответствует выбранному направлению.

Новая опорная модель:

1. Пользователь задаёт только `spec.source` по аналогии с virtualization:
   - `HuggingFace`
   - `HTTP`
   - `Upload`
2. Модуль публикует модель только в backend-owned artifact plane.
3. Текущая implementation artifact plane строится по reference-паттернам virtualization, но в current code/docs не фиксируется product-specific naming.
4. `MLflow` остаётся поднятым рядом как внутренний backend phase-1, но в publication pipeline phase-2 не участвует.
5. Public contract по-прежнему остаётся `Model` / `ClusterModel` с `status.artifact` и `status.resolved`.
6. Runtime consumption должен в будущем идти через local materialization в PVC/shared volume, а не через прямую работу рантайма с backend-специфичным URI.

При этом важно не потерять ideологию ADR:

- source-first contract;
- controller-owned upload;
- calculated technical profile;
- controller-owned cleanup при удалении;
- namespaced и cluster scope, как у virtualization.

## Постановка задачи

Жёстко ребейзнуть controller runtime и internal publication contracts под backend-owned artifact plane, убрать central wiring вокруг `payload-registry` и `mlflow publication`, и подготовить безопасную основу для следующего live slice `source -> backend artifact -> resolved profile -> ready`.

## Scope

- Завести новый phase-2 task bundle под backend artifact plane.
- Зафиксировать и реализовать первый corrective slice:
  - controller runtime больше не конфигурируется через `payload-registry`, `managed backend publication` и legacy cleanup/publication workers;
  - publication operation contracts больше не тащат `managedbackend`/`mlflow` semantics и legacy cleanup state;
  - delete lifecycle должен использовать backend-neutral cleanup handle и backend cleanup entrypoint вместо `mlflow`-shaped phase-2 contracts;
  - internal identity/scope должны быть готовы к `namespaced` и `cluster` ownership без привязки к registry path layout;
  - docs controller-а должны описывать backend artifact plane direction без product-specific naming;
  - foundation должна допускать простую future replacement текущей backend implementation на `payload`;
  - legacy phase-2 packages удалены там, где они больше не нужны runtime или следующему implementation slice.
- Сохранить existing public `Model` / `ClusterModel` source-first contract как временную базу.
- Реализовать первые live source slices поверх module-owned object storage contract:
  - `HuggingFace -> backend artifact -> resolved -> Ready`;
  - `HTTP -> backend artifact -> resolved -> Ready`.
- Исправить execution contract между controller publication Job и backend worker, чтобы live slices были реально исполняемыми, а не только unit-tested.

## Non-goals

- Не реализовывать в этом slice `Upload`.
- Не переносить сейчас весь runtime-materializer/agent для `ai-inference`.
- Не трогать phase-1 deployment `MLflow`, кроме удаления publication coupling из phase-2 controller.
- Не делать в этом slice финальный cleanup path для saved artifact.

## Затрагиваемые области

- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/app/*`
- `images/controller/internal/publication/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `images/controller/internal/artifactbackend/*`
- `images/controller/internal/*`
- `images/controller/README.md`
- `templates/controller/*` при необходимости
- `docs/CONFIGURATION*`
- `plans/active/rebaseline-publication-plane-to-backend-artifact-plane/*`

## Критерии приёмки

- В репозитории есть новый task bundle под backend artifact plane pivot.
- Controller runtime больше не требует `registry-host`, `registry-namespace` и `managed-backend-*` для phase-2 publication flow.
- Internal publication contracts не содержат `mlflow`/`managedbackend` publication semantics и не завязаны на старый cleanup worker path.
- Delete lifecycle использует backend-neutral cleanup handle и generic backend cleanup command.
- `Model` / `ClusterModel` lifecycle controller может продолжать работать через bounded operation contract без registry-specific identity.
- Для `spec.source.type=HuggingFace` controller создаёт backend publication Job, получает durable backend result и доводит object до `status.artifact`, `status.resolved` и `phase=Ready`.
- Для `spec.source.type=HTTP` controller создаёт backend publication Job, получает durable backend result и доводит object до `status.artifact`, `status.resolved` и `phase=Ready`.
- Backend publication worker и controller Job используют согласованный CLI/API contract для artifact URI, source-specific args и durable result handoff.
- Документация controller-а больше не описывает `payload-registry`, `mlflow/S3` или прошлые publication directions как активный phase-2 path.
- В controller module появился нейтральный `artifactbackend` boundary для будущей замены текущей backend implementation на `payload`.
- Legacy packages `payloadregistry`, `registrypath`, `managedbackend`, `modelimportjob`, `uploadsession` и связанные старые helpers не остаются в active phase-2 surface.
- Узкие тесты controller-а проходят.

## Риски

- Часть старых contracts, особенно вокруг upload/runtime delivery, ещё требует следующего rebasing под backend artifact plane.
- Если попытаться прямо сейчас полностью вырезать старую реализацию, можно сломать cleanup и lifecycle сильнее, чем нужно для первого corrective шага.
- После live `HuggingFace` + `HTTP` baseline phase-2 publication всё ещё остаётся неполной: `Upload`, runtime materialization, auth projection и live cleanup execution ещё впереди.
