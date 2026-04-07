# Rebase Phase-2 Publication Runtime To Go DVCR ModelPack Pattern

## Контекст

Текущий phase-2 publication path уже умеет публиковать `Model` / `ClusterModel`
из `HuggingFace`, narrow `HTTP` и `Upload` в OCI-backed
`ModelPack` artifacts. Но concrete data plane реализован плохо:

- слишком большая часть publication/runtime semantics уехала в
  `images/backend/scripts/*.py`;
- `KitOps` сейчас встроен не как replaceable implementation adapter, а как
  hard-coded CLI path внутри Python script;
- `ai-inference`-oriented resolved metadata в API уже предусмотрена, но
  текущий publish path считает только маленький subset;
- runtime/public contract уже договорён как `OCI from registry`, hidden backend
  under `DVCR`, `ModelPack` as contract, but current implementation drift still
  keeps phase-2 publication closer to ad-hoc backend scripts than to the
  virtualization pattern.

Пользователь прямо требует:

- убрать Python/script-centric phase-2 publication/runtime path;
- сделать controller/runtime path самодостаточным и Go-first;
- переиспользовать working patterns из virtualization, особенно DVCR/data-plane
  discipline;
- оставить module-owned domain только там, где это действительно наш продукт:
  `Model` / `ClusterModel`, `ModelPack` contract, inference-oriented metadata.
- зафиксировать простой public input contract:
  - `spec.source`
  - `spec.inputFormat`
  где `spec.source` сводится к `source.url` или `source.upload`, а
  `spec.inputFormat` можно не указывать при однозначном автоопределении
  при скрытом fixed output contract без лишнего `spec.publish`.

## Постановка задачи

Переосмыслить и постепенно перевести phase-2 publication/runtime implementation
на virtualization-style architecture:

- controller/runtime data plane в Go;
- hidden backend artifact plane reused by pattern, not reimplemented;
- `ModelPack` contract with replaceable implementations (`KitOps`, `Modctl`,
  future module-owned encoder) behind adapters;
- ai-inference-oriented resolved metadata becomes a first-class publication
  outcome, not an optional afterthought.

## Scope

- Создать canonical bundle для полного rebase phase-2 publication/runtime path.
- Зафиксировать explicit target architecture, ownership и slice order.
- Зафиксировать, какие текущие Python/shell paths считаются temporary debt, а
  какие остаются phase-1/backend-adjacent shell.
- Начать bounded implementation cuts в сторону Go-first publication/runtime
  path без возврата к старой patchwork structure.
- Синхронизировать repo memory, docs и controller structure rationale.

## Non-goals

- Не переписывать phase-1 MLflow runtime/deployment shell в этой задаче.
- Не менять публичный DKP contract без отдельного обоснования в slice.
- Не реализовывать сразу весь `ai-inference` mutation/runtime delivery path в
  одном mega-slice.
- Не писать собственный registry/storage backend с нуля.
- Не ломать текущий live publication path без replacement slice.

## Затрагиваемые области

- `images/controller/internal/*`
- `images/backend/scripts/*`
- `images/backend/werf.inc.yaml`
- `docs/*`
- `plans/active/*`
- `.agents/skills/*` если потребуется закрепить durable discipline

## Критерии приёмки

- Canonical architecture bundle явно фиксирует:
  - Go-first phase-2 data plane;
  - hidden backend under DVCR-style artifact plane;
  - `ModelPack` as contract, concrete tools only as adapters;
  - ai-inference metadata as required publication outcome.
- Ясно перечислены текущие implementation drifts и backlog cuts по каждому
  проблемному seam:
  - Python publication worker;
  - Python upload session server;
  - metadata calculation gap;
  - runtime delivery gap.
- Первый bounded corrective slice реально landed в код, а не только в docs.
- Изменение не ухудшает текущие controller quality gates и не создаёт новый
  patchwork bundle.

## Риски

- Можно увлечься большим redesign и зависнуть без первого landed slice.
- Можно перепутать phase-1 backend-adjacent scripts с phase-2 publication debt
  и удалить нужный shell раньше времени.
- Можно снова зашить public/runtime contract на конкретный tool brand.
