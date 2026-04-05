# Design Model / ClusterModel Controller And Publish Architecture

## 1. Контекст

У `ai-models` уже есть phase-1 baseline с внутренним managed backend, SSO,
workspaces, import jobs и S3-backed artifact path. На практике стало видно, что:

- raw MLflow surface слишком сложен как пользовательский контракт платформы;
- для inference/distribution текущий `MLflow + S3 prefix + bucket policy` даёт
  слабый UX и неудобный RBAC;
- для `KServe` лучше выглядит `KitOps/ModelPack + OCI registry`;
- для `KubeRay` прямой `KitOps` path не first-class, но возможен через
  init-container unpack, а direct `S3 bucket_uri` остаётся fallback;
- локальный upload с ноутбука нельзя строить на browser upload больших файлов;
- `payload-registry` в `dkp-registry` выглядит как реалистичный DKP-native OCI
  backend, но требует отдельной governance-модели из-за namespace-root semantics.

Нужно сформировать полноценный phase-2 target design, по которому потом можно
реализовывать `Model`, `ClusterModel`, контроллеры публикации и publish flow.

## 2. Постановка задачи

Спроектировать публичный DKP API и целевую архитектуру `ai-models` для phase 2:

- определить роль `Model` и `ClusterModel`;
- определить controller/runtime boundaries;
- определить publish contract для разных источников:
  - `Hugging Face`;
  - локальный upload с компьютера;
  - расширяемый путь для training outputs;
- определить, как `KitOps/ModelPack` используется для packaging и publication;
- определить, как published artifacts живут в `payload-registry`;
- определить, как организовать RBAC и access grants для publish/pull;
- определить роли пользователей, controller-owned actions и user journeys;
- определить место внутреннего backend в целевой архитектуре.

## 3. Scope

- Целевая phase-2/phase-2.1 архитектура public catalog API.
- `Model` / `ClusterModel` API contract на уровне spec/status/conditions.
- Controller state machine и ownership model.
- Publish pipeline через `KitOps/ModelPack`.
- Registry layout и access model для `payload-registry`.
- Upload contract по аналогии с virtualization/DVCR.
- User flows:
  - HF import;
  - локальный upload;
  - consumption from KServe;
  - consumption from KubeRay;
  - delete / cleanup.
- Чёткая рекомендация, что является canonical storage/publish form.

## 4. Non-goals

- Не реализовывать сейчас сами CRD, контроллер или jobs.
- Не переделывать phase-1 backend runtime.
- Не внедрять live `KitOps` publish в кластер.
- Не выбирать финально long-term registry product вместо `payload-registry`
  и `Harbor`; для этого дизайна нужен implementable target, а не procurement
  decision.
- Не проектировать browser upload wizard.
- Не делать inference orchestration частью `ai-models`.

## 5. Затрагиваемые области

- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `docs/development/*` только как reference, без изменения contract на этом шаге
- локальные reference repos:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/dkp-registry/payload-registry/*`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/*`

## 6. Критерии приёмки

- Есть единый design bundle, который:
  - объясняет, что является public contract;
  - объясняет, что является internal backend boundary;
  - описывает source->package->publish->consume flow;
  - даёт implementable spec/status shape для `Model` и `ClusterModel`;
  - описывает registry path layout и RBAC model;
  - описывает upload flow с локального компьютера без browser data plane;
  - описывает KServe и KubeRay consumer paths;
  - объясняет delete/cleanup semantics.
- Решение не выводит наружу raw MLflow entities как платформенный UX.
- Решение не завязывает public API на `payload-registry` internal virtual
  objects; backend specifics остаются implementation detail.
- Решение укладывается в phase-2 и честно отмечает phase-2.1/phase-3 topics.

## 7. Риски

- `payload-registry` operationally image-centric и может дать шероховатости на
  `ModelPack` media types или tooling.
- `KubeRay` не имеет first-class `KitOps` integration; единый publish artifact
  придётся адаптировать отдельным runtime path.
- Если сделать `payload-registry` canonical storage, текущий MLflow artifact
  contract нужно осознанно сузить до metadata/provenance role, иначе получится
  двойное владение published artifacts.
- Upload path через временные staging repos требует аккуратной TTL/cleanup
  discipline.
