## 2026-04-25. SDS/CSI render guardrail

### Наблюдение

- `nodeCache` уже строит managed `LVMVolumeGroupSet` и `LocalStorageClass`
  через `storage.deckhouse.io` CRD.
- Pre-cutover workload bridge уже мог опираться на managed
  `LocalStorageClass`, но оставался workload-local materialize path.
- Но включение `nodeCache` без `sds-node-configurator` / `sds-local-volume` и
  без node/block-device selectors раньше обнаруживалось только после rollout,
  на controller startup.

### Решение

- Вынести prerequisites в Helm validation:
  - required modules: `sds-node-configurator-crd`,
    `sds-node-configurator`, `sds-local-volume-crd`, `sds-local-volume`;
  - `nodeCache.nodeSelector` must be non-empty;
  - `nodeCache.blockDeviceSelector` must be non-empty.
- Добавить render fixture с включённым `nodeCache`, SDS modules и selectors.
- Дублировать non-empty selector guard в controller config parser, чтобы
  прямой запуск через env/flags не обходил Helm validation.
- Обновить docs: текущий CSI-backed bridge не считать финальным
  `SharedDirect`; финальный cutover остаётся отдельным slice.

## 2026-04-25. Workload-facing SharedDirect cutover

### Наблюдение

- Cross-namespace PVC не может быть корректным workload-facing contract для
  managed node-cache: workload namespace не должен получать module-local
  storage object или DMCR read Secret только ради потребления уже опубликованной
  модели.
- Managed default path должен быть node-owned: workload сообщает immutable
  artifact identity, а фактическое чтение DMCR и заполнение shared store делает
  node-cache runtime plane.
- Explicit workload-provided cache volumes остаются legacy bridge path, потому
  что это пользовательский storage contract и его нельзя тихо заменить.

### Решение

- Managed workload без явного `/data/modelcache` volume получает inline CSI
  volume `node-cache.ai-models.deckhouse.io` с artifact URI/digest/family
  attributes и стабильным mount `/data/modelcache/model`.
- Controller больше не проецирует DMCR read auth/CA и runtime imagePullSecret в
  workload namespace для managed SharedDirect path; stale projections удаляются
  при следующем reconcile.
- `nodeCache.nodeSelector` переносится в managed workload pod template, а
  конфликтующие selectors fail-close до rollout.
- `SharedDirect` / `NodeSharedRuntimePlane` annotations становятся живым
  desired-artifact contract для node-cache runtime live Pod discovery.

## 2026-04-25. Node-cache runtime hardening

### Наблюдение

- Узловой CSI/runtime plane ещё использует общий `controllerRuntime` image, хотя
  по responsibility это отдельный privileged node daemon.
- Ошибка скачивания одного digest сейчас может завершить весь runtime loop и
  вызвать restart Pod'а.
- Workload placement проверяет только статический `nodeCache.nodeSelector`, но
  не факт готовности runtime Pod/socket/cache PVC на конкретной ноде.

### Решение

- Выделить внутренний dedicated `nodeCacheRuntime` image поверх того же
  `images/controller` source/build context и distroless base; публичный values
  knob не добавлять.
- Перенести retry/backoff в `internal/nodecache`: ошибка одного digest должна
  логироваться, откладываться до следующего allowed attempt и не мешать другим
  digest'ам или maintenance.
- Контроллер runtime plane должен выставлять/снимать managed Node label
  готовности, а workload delivery обязан добавлять этот label в nodeSelector
  managed SharedDirect workload'ов.
- Не создавать отдельный CRD/intent plane и не переносить placement policy в
  CSI node server.
