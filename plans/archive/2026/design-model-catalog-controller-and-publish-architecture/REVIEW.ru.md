# Review

## Findings

- Bundle остаётся в фазовых границах: проектирует phase-2 public API и
  controller orchestration, но не тянет implementation внутрь phase-1 runtime.
- Public contract не протекает raw MLflow сущностями: `Model` / `ClusterModel`
  и их status опираются на generic `ociRef` / digest / metadata, а не на
  `Run`, `Logged Model`, `Workspace` или `PayloadRepositoryTag`.
- Canonical publish plane выбран последовательно: `ModelPack/ModelKit` в OCI
  registry через `payload-registry`, а internal backend переведён в роль
  secondary metadata/provenance mirror.
- Upload path сформулирован в DKP-манере: controller-owned staging contract,
  а не browser upload и не manual final push.
- Namespaced vs cluster-scoped access split выглядит defensible:
  - `Model` получает простой same-namespace default;
  - `ClusterModel` требует explicit access policy.

## Checks

- `git diff --check -- plans/active/design-model-catalog-controller-and-publish-architecture`
- review against:
  - `docs/development/TZ.ru.md`
  - `docs/development/PHASES.ru.md`
  - `docs/development/REVIEW_CHECKLIST.ru.md`
  - `plans/active/design-backend-isolation-and-storage-strategy/*`
  - `plans/active/evaluate-kitops-with-dkp-registry/*`
  - `plans/active/design-kuberay-rgw-s3-consumption/*`

## Residual risks

- `payload-registry` как OCI backend для `ModelKit` логичен по contract, но
  live smoke push/pull реального `ModelKit` ещё не подтверждён.
- `KubeRay` всё ещё остаётся adapter path, а не first-class integration; если
  это станет главным runtime на ближайшем горизонте, design придётся
  дополнительно упростить под него.
- Выбор canonical OCI artifact означает, что формулировку phase-2 sync с
  internal backend нужно трактовать как metadata/provenance sync, а не как
  обязательное дублирование published bytes в backend artifact store.
- Если later выяснится, что `payload-registry` namespace-root semantics и
  PVC-backed storage operationally неудобны для model growth, нужно будет
  заменить backend registry implementation за adapter boundary без смены public
  API.
