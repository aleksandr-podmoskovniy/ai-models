# Rebase Phase-2 Publication Runtime To Go DMCR ModelPack Pattern

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
  under `DMCR`, `ModelPack` as contract, but current implementation drift still
  keeps phase-2 publication closer to ad-hoc backend scripts than to the
  virtualization pattern.

Пользователь прямо требует:

- убрать Python/script-centric phase-2 publication/runtime path;
- сделать controller/runtime path самодостаточным и Go-first;
- переиспользовать working patterns из virtualization, особенно DMCR/data-plane
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

### Current contract override (2026-04-11)

- internal phase-1 backend remains MLflow-based and keeps PostgreSQL as a
  required dependency for metadata/auth storage;
- shared S3-compatible `artifacts` storage is now the only supported byte
  backend for:
  - MLflow artifacts;
  - controller-owned raw ingest;
  - internal DMCR publication bytes;
- inline object-storage credentials are not supported anymore; module users
  must provide an existing Secret reference;
- PVC-backed internal publication storage is no longer part of the live
  user-facing contract.

Переосмыслить и постепенно перевести phase-2 publication/runtime implementation
на virtualization-style architecture:

- controller/runtime data plane в Go;
- hidden backend artifact plane reused by pattern, not reimplemented;
- module-local internal DMCR becomes the actual publication backend instead of
  an externally configured OCI registry;
- internal publication backend supports the two storage modes required by the
  target architecture:
  - S3-compatible object storage;
  - `PersistentVolumeClaim`;
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
- Держать phase-2 publish path независимым от internal `MLflow` backend и не
  превращать backend metadata plane в обязательную часть platform publication
  lifecycle.
- Убрать external-registry-centric module contract и заменить его на
  module-owned internal publication plane contract.
- Выровнять module build/deploy shell с DKP module patterns из
  `gpu-control-plane` и `virtualization` там, где это относится к
  phase-2 runtime path:
  - module-local distroless layer для собственного Go runtime;
  - chart-level `deckhouse_lib_helm` dependency вместо repo-local helper fork;
  - явное разделение между phase-2 controller runtime shell и phase-1 backend
    packaging shell.
- Синхронизировать repo memory, docs и controller structure rationale.
- Зафиксировать module-owned observability contract для phase-2 runtime path:
  - какие logging patterns обязательны;
  - какие product/state metrics действительно нужны;
  - какие alerts должны быть у модуля на уровне platform integration;
  - что нельзя копировать из virtualization blindly и не нужно тащить в
    `ai-models`.
- Добить текущие structural hotspots внутри уже landed runtime tree без
  выдумывания новых fake boundaries:
  - `catalogcleanup`
  - `dmcr-cleaner`
- Вернуть в public contract только те policy-поля, у которых есть живая
  runtime/publication semantics:
  - `spec.modelType`
  - `spec.usagePolicy`
  - `spec.launchPolicy`
  - `spec.optimization`
- Убрать текущий synchronous `uploadsession` smell, где один ephemeral pod
  сначала принимает большие байты, а потом в том же critical path сам делает
  publication.
- Убрать `KitOps` CLI как runtime dependency, если replacement slice можно
  сделать без потери OCI publication contract и без новой fake abstraction.
- Сжать текущие жирные controller hotspots in place:
  - `sourcefetch`
  - `modelformat`

## Non-goals

- Не переписывать phase-1 MLflow runtime/deployment shell в этой задаче.
- Не переводить backend Python runtime в distroless до отдельного hardening
  решения этапа 3.
- Не менять публичный DKP contract без отдельного обоснования в slice.
- Не реализовывать сразу весь `ai-inference` mutation/runtime delivery path в
  одном mega-slice.
- Не писать собственный registry/storage backend с нуля.
- Не ломать текущий live publication path без replacement slice.
- Не удалять phase-1 MLflow/backend path, пока внутренний publication backend
  не wired end-to-end и не закрывает текущий phase-2 path.
- Не возвращать в public API старую ADR-спеку буквально, если конкретное поле
  не подтверждено живой controller semantics.
- Не тащить speculative policy blocks как пустые placeholder knobs без
  validation и observable effect в status/conditions.

## Затрагиваемые области

- `images/controller/internal/*`
- `images/distroless/*`
- `images/backend/scripts/*`
- `images/backend/werf.inc.yaml`
- `werf.yaml`
- `Chart.yaml`
- `Chart.lock`
- `openapi/*`
- `templates/*`
- `docs/*`
- `plans/active/*`
- `.agents/skills/*` если потребуется закрепить durable discipline
- platform observability contract and monitoring integration notes derived from
  the sibling `virtualization` module patterns

## Критерии приёмки

- Canonical architecture bundle явно фиксирует:
  - Go-first phase-2 data plane;
  - hidden backend under a real module-local DMCR-style artifact plane;
  - `ModelPack` as contract, concrete tools only as adapters;
  - ai-inference metadata as required publication outcome.
- Chart/runtime shell больше не требует внешнего `publicationRegistry`
  endpoint/credentials contract; controller publication workers всегда смотрят
  в module-local internal registry service.
- Module config задаёт storage semantics внутреннего publication backend в
  терминах модуля (`S3` или `PersistentVolumeClaim`), а не как внешний registry
  wiring.
- Ясно перечислены текущие implementation drifts и backlog cuts по каждому
  проблемному seam:
  - Python publication worker;
  - Python upload session server;
  - metadata calculation gap;
  - runtime delivery gap.
- `spec.modelType`, `spec.usagePolicy`, `spec.launchPolicy` и
  `spec.optimization` либо отсутствуют, либо имеют живую semantics:
  validation against calculated profile, immutability rules, and condition
  impact visible through `Validated` / `Ready`.
- Upload path больше не заставляет один и тот же runtime pod держать
  пользовательский upload stream и publication pipeline в одной synchronous
  critical section.
- Object-storage upload target масштабируется через shared upload gateway и
  short-lived session records, а не через one-Pod-per-upload shell.
- Upload session state keeps controller-owned resumability data instead of
  relying only on `multipartUploadID`: persisted multipart part manifest and
  explicit session phases must exist at least through the raw-ingest stage.
- Для multi-TB target scenario cluster footprint при десятках concurrent
  uploads растёт session state и S3 multipart uploads, а не линейным числом
  uploader Pods.
- phase-2 publish path не требует `MLflow` run/model/version lifecycle для
  normal operation;
- raw ingest, publish status и final OCI reference остаются controller-owned
  concerns under `CRD.status`, а не backend entity model.
- Internal audit/provenance hooks, if present, remain append-only and
  non-authoritative:
  initial implementation may use controller-owned `Kubernetes Events` and
  minimal internal raw provenance (`RawURI`, object count, total size) without
  expanding public `status` or creating a second lifecycle engine.
- Primary preflight checks before full ingest explicitly cover only:
  authz, quota, declared size/format allowlist and lightweight source probes;
  future malware/content scanners stay a later async stage and are not
  misrepresented as first-release guarantees.
- Same object-storage backend may be reused for raw ingest and `DMCR`, but
  logical separation between `raw/`, optional `audit/`, and `dmcr/` storage
  areas is explicit.
- Phase-2 runtime больше не зависит от external `KitOps` CLI binary inside the
  runtime image, если native replacement slice landed safely.
- `sourcefetch` и `modelformat` уменьшаются как hotspots через explicit
  ownership split, а не через generic util dumps.
- Первый bounded corrective slice реально landed в код, а не только в docs.
- Изменение не ухудшает текущие controller quality gates и не создаёт новый
  patchwork bundle.
- Controller/runtime build shell использует module-local distroless image, а не
  прямой `base/distroless`.
- Helm chart использует normal DKP dependency path для `deckhouse_lib_helm`,
  без repo-local fork как primary source of truth.
- Helm render, kubeconform и targeted werf build подтверждают, что module shell
  по-прежнему корректно собирает и рендерит controller runtime path.
- Current structural hotspots are reduced in place instead of being hidden
  behind new generic packages:
  - `catalogcleanup` no longer keeps one monolithic controller I/O file for
    observation, mutation, status update and GC-request lifecycle;
  - `dmcr-cleaner` command package becomes a thin CLI shell and moves runtime
    garbage-collection lifecycle logic into an explicit internal implementation
    seam under `images/dmcr`.
- Controller structure docs stay aligned with the live runtime tree:
  shared worker/session adapters consume one direct `publishop.Request`,
  and legacy backend-centric package names do not survive once the live
  boundary becomes controller-owned.
- Observability contract is explicit and bounded:
  - module scrape shell stays protected through `ServiceMonitor` and
    `kube-rbac-proxy` style access;
  - alerts focus on target availability, pod readiness/running state and
    module-owned storage risk, not on noisy internal counters;
  - product metrics are anchored in `Model` / `ClusterModel` status truth and
    a small number of actionable booleans/sizes, not in per-step debug churn;
  - runtime-local progress/debug metrics may exist for worker internals, but
    are not misrepresented as the platform monitoring contract.

## Риски

- Можно увлечься большим redesign и зависнуть без первого landed slice.
- Можно перепутать phase-1 backend-adjacent scripts с phase-2 publication debt
  и удалить нужный shell раньше времени.
- Можно снова зашить public/runtime contract на конкретный tool brand.
- Можно вернуть в `spec` красивые, но пустые policy knobs и снова получить
  ложный contract drift.
- Можно попытаться выкинуть synchronous upload path без честного intermediate
  storage/ownership seam и сломать большие upload scenarios хуже, чем сейчас.
