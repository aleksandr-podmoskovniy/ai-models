# GPU-direct runtime delivery target design

## Контекст

Текущий production-safe workload delivery contract движется к двум режимам:

- `SharedDirect`: node-local cache через ai-models CSI, `nodeCache.enabled=true`;
- `SharedPVC`: controller-owned RWX filesystem для кластеров без local disks.

Появилась идея заменить `SharedPVC` и ускорить cold start через прямую загрузку
модели из `DMCR` / object storage / registry в память GPU workload'а, возможно
через RDMA, NVMe-oF, GPUDirect Storage или аналогичные механизмы. При наличии
локального кэша предлагается параллельно прогревать node-local cache, чтобы
повторные рестарты читали уже с локального источника.

## Задача

Спроектировать production target topology для accelerated model delivery,
не протаскивая экспериментальные runtime-specific механизмы в текущий
Kubernetes workload contract.

## Scope

- Определить, какие части остаются в `ai-models`, а какие принадлежат будущему
  `ai-inference` runtime adapter.
- Разделить безопасный filesystem delivery baseline и будущий accelerator-direct
  optimization path.
- Зафиксировать security model для digest-scoped read grants без выдачи DMCR
  credentials в workload namespace.
- Зафиксировать HA/retry/cache-warmup требования.
- Сформировать slices для proof-of-concept и production hardening.

## Non-goals

- Не менять текущий `Model` / `ClusterModel` API в этом design slice.
- Не удалять `SharedPVC` без runtime proof, потому что он остаётся
  универсальным POSIX-filesystem fallback для runtime'ов, которым нужен path.
- Не обещать generic S3/registry-to-GPU path как DKP/Kubernetes primitive.
- Не добавлять workload mutation, sidecars или credentials в code без
  отдельного implementation bundle.

## Затрагиваемые области

- `docs/development/*` или future ADR;
- `plans/active/csi-workload-ux-contract`;
- `plans/active/modelpack-efficient-compression-design`;
- будущий `ai-inference` integration contract.

## Критерии приёмки

- Есть чёткий verdict: что production-safe сейчас, что future/experimental.
- Есть byte path для cold start, cache hit and write-through warmup.
- Есть security boundary: no raw DMCR/S3 credentials in workload namespace.
- Есть scheduler-facing capability model: filesystem, node-cache, GDS/RDMA/NIXL
  как свойства runtime/node, а не свойства модели.
- Есть rollout slices с проверяемыми доказательствами.

## Риски

- Runtime-specific optimization может протечь в public `Model.spec`.
- Direct-to-GPU может сломать изоляцию: controller/CSI не владеет памятью GPU
  пользовательского процесса.
- Без range/resume/chunk integrity cold-start optimization станет менее HA, чем
  обычный filesystem delivery.
- RDMA/GDS support сильно зависит от hardware, kernel, filesystem, driver and
  inference runtime.
