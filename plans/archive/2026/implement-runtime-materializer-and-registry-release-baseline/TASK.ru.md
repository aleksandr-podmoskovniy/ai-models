# Runtime Materializer And Registry Release Baseline

## Контекст

В репозитории уже есть phase-2 baseline для:

- `Model` / `ClusterModel`;
- source-first publication из `HuggingFace`, narrow `HTTP` и `Upload`;
- публикации `ModelPack` в OCI registry через текущий implementation adapter;
- delete cleanup опубликованных OCI artifacts;
- corrective refactor controller towards `domain / application / ports / adapters`.

Но release baseline пока неполный:

- runtime delivery path в `ai-inference` ещё не реализован;
- public/API/docs всё ещё содержат остатки старой backend-neutral/object-storage
  модели, хотя live path уже OCI-first;
- в репозитории ещё остаются legacy wording и seams, которые мешают строить
  production-ready materializer path;
- `mlflow` phase-1 runtime остаётся рядом, но не должен участвовать в
  publication/runtime contract.

Пользовательский целевой сценарий теперь явный:

1. source-first publication остаётся как есть;
2. canonical artifact plane — published `ModelPack`/`ModelKit` в OCI registry;
3. runtime всегда получает модель через init-container/materializer и работает
   только с локальным путём на volume/PVC;
4. backend, лежащий под registry plane, не влияет на public или runtime
   contract: как и в virtualization, потребитель всегда видит только OCI ref;
5. старые неверные ветки и лишняя patchwork-структура должны быть вычищены;
6. `mlflow` не трогаем, он просто продолжает стоять рядом.

## Постановка задачи

Довести phase-2 implementation до release-oriented baseline для registry path:

1. вычистить старые API/docs/contracts, которые расходятся с текущим OCI-first
   publication path;
2. ввести явный runtime materialization contract для init-container based
   delivery;
3. реализовать module/runtime wiring для v0 init-adapter based
   materialization;
4. подготовить shared-volume/PVC handoff so that runtime sees only local model
   path;
5. не трогать phase-1 `mlflow` runtime, кроме случаев, где текущие docs надо
   просто синхронизировать.
6. не расширять `openapi/config-values.yaml` runtime/materializer-specific
   knobs; follow the same split as virtualization.

## Scope

- `api/core/v1alpha1/*`
- generated `crds/*`
- `images/controller/internal/*` around runtime/materialization
- controller bootstrap and module templates
- `openapi/*`
- `docs/CONFIGURATION*`
- active bundle docs for this workstream

## Non-goals

- Не менять phase-1 `mlflow` deployment/runtime logic.
- Не проектировать заново publication source flows, если их можно reuse'ить.
- Не вводить новый public API для pod injection в этом же bundle.
- Не делать сразу node-local cache plane; v0 baseline может идти через shared
  volume / PVC.
- Не переписывать сразу текущий implementation adapter на raw custom
  `ModelPack` encoder; concrete implementation должна оставаться сменяемой.
- Не вытаскивать adapter-specific runtime settings в public module contract.

## Затрагиваемые области

- `api/`
- `crds/`
- `images/controller/`
- `templates/controller/*`
- `openapi/*`
- `docs/CONFIGURATION*`
- `plans/active/implement-runtime-materializer-and-registry-release-baseline/*`

## Критерии приёмки

- Public API и docs больше не рекламируют старую `ObjectStorage` artifact shape
  там, где live path уже OCI-only.
- В controller есть явный runtime materialization contract, не смешанный с
  publish/store internals.
- Runtime contract не различает `PVC`/`S3` или другие storage backend details
  под registry plane: для него всегда существует только immutable OCI ref from
  registry.
- Есть v0 init-adapter runtime wiring contract:
  - immutable OCI ref;
  - digest verification inputs;
  - shared volume/PVC unpack path;
  - main runtime gets local path only.
- `ModelPack` contract не зашит на `KitOps`: текущие `kit` / `kitops-init`
  остаются только v0 adapters behind ports.
- Старые неправильные или мёртвые seams, мешающие этой модели, вычищены или
  зафиксированы как explicit follow-up.
- Релевантные docs и OpenAPI/values согласованы с новой runtime direction.
- `openapi/config-values.yaml` остаётся коротким user-facing module contract,
  а runtime/materializer specifics живут только в `openapi/values.yaml`,
  templates/helpers и code.

## Риски

- Слишком рано потащить runtime pod mutation в public API и снова размыть
  boundaries.
- Смешать release baseline с будущим node-local cache plane и расползтись по
  scope.
- Оставить старые `ObjectStorage`/backend-neutral хвосты и получить снова
  противоречивую контрактную модель.
- Случайно протащить в runtime contract детали backend storage под `DVCR`
  вместо virtualization-style `registry target only`.
