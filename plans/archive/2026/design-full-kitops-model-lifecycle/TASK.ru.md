# Full KitOps Model Lifecycle Design

## 1. Контекст

В репозитории уже есть phase-2 baseline для:

- `Model` / `ClusterModel`;
- controller-owned publication;
- `KitOps`-based OCI publication path для `HuggingFace`, narrow `HTTP` и
  `Upload(HuggingFaceDirectory)`;
- delete cleanup published OCI artifacts.

Но рабочая картина всё ещё неполная:

- нет полной target-схемы от `spec.source` до запуска модели в runtime;
- нет зафиксированного временного решения для runtime materialization;
- нет цельной security-модели вокруг publication, verification, unpack и delete;
- нет чёткого rationale, почему platform path должен быть построен вокруг
  `KitOps`, а не вокруг raw storage URLs или ad hoc runtime adapters.

Пользовательский запрос сейчас шире очередного code slice:

1. использовать upstream `kit init` container как временный runtime-delivery
   primitive;
2. зафиксировать, зачем платформа вообще использует `KitOps`;
3. включить полную цепочку проверок для безопасной публикации и запуска модели;
4. прорисовать end-to-end схему от upload/source до запуска и удаления модели.

## 2. Постановка задачи

Подготовить полный design bundle для target lifecycle `Model` /
`ClusterModel` на базе `KitOps`:

1. source-first publication:
   - `HuggingFace`;
   - `HTTP`;
   - `Upload` по паттерну virtualization/DVCR;
2. packaging/publish в OCI через `KitOps`;
3. metadata/profile enrichment в `status.resolved`;
4. runtime delivery через upstream `kit init` container как v0 adapter;
5. future path к module-owned materializer / cache plane;
6. destructive cleanup и delete guards;
7. security gates и integrity/signature/policy checks на всём пути.

## 3. Scope

- Сформулировать rationale for `KitOps` как platform packaging/delivery plane.
- Зафиксировать target public/runtime boundaries:
  - что видит `Model` / `ClusterModel`;
  - что делает controller;
  - что делает runtime materializer/init container;
  - что остаётся internal-only.
- Описать v0 runtime path через upstream `kit init` container.
- Описать v1+ path для module-owned materializer image/agent, cache и hardening.
- Описать full lifecycle:
  - source acceptance;
  - upload/publication;
  - profile extraction;
  - ready/readiness;
  - runtime pull/unpack into PVC;
  - delete guards and cleanup.
- Описать security controls и явно разделить:
  - что умеет сам `KitOps`;
  - что должны делать мы;
  - что нельзя честно обещать как «безопасность модели».

## 4. Non-goals

- Не реализовывать в этом bundle новый runtime code.
- Не патчить прямо сейчас upstream `kit init` image.
- Не делать сейчас distroless/rebase/hardening implementation.
- Не менять в этом bundle CRD или controller code.
- Не переоткрывать уже принятый source-first public UX.

## 5. Затрагиваемые области

- `plans/active/design-full-kitops-model-lifecycle/*`
- при необходимости ссылки на:
  - `api/core/v1alpha1/*`
  - `images/controller/internal/*`
  - `docs/CONFIGURATION*`
  - archival design bundles в `plans/archive/2026/*`

## 6. Критерии приёмки

- Есть отдельный design bundle с:
  - rationale;
  - target architecture;
  - user flows;
  - security model;
  - phased implementation order.
- В bundle явно зафиксирован v0 runtime path через upstream `kit init`.
- В bundle явно разведены:
  - publication plane;
  - runtime delivery plane;
  - future cache/materializer plane.
- Зафиксировано, почему `KitOps` подходит платформе и где его границы.
- Зафиксировано, какие проверки обязательны до `Ready` и до runtime start.
- Зафиксировано, какие delete guards нужны перед очисткой опубликованного
  артефакта и локальных materializations.

## 7. Риски

- Слишком рано объявить upstream `kit init` final architecture, хотя это только
  v0 adapter.
- Перепутать «целостность и provenance артефакта» с «полной безопасностью
  модели» как ML-объекта.
- Смешать в одном документе publication, runtime materialization и future cache
  plane без чётких stage boundaries.
