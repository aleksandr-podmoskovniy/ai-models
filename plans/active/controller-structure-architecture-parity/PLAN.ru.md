### 1. Current phase

Этап 2 runtime topology: node-cache/CSI путь уже появился в live tree, поэтому
architecture docs должны отражать текущие boundaries.

### 2. Orchestration

`full`

Причина:

- задача затрагивает controller/runtime architecture;
- пользователь явно попросил подключить архитектора;
- нужно сверить границы с Deckhouse/virtualization patterns и не внести
  docs-монолит вместо архитектуры.

Read-only reviews перед implementation:

- `repo_architect`: проверить package topology, anti-monolith risks, stale
  boundaries and documentation shape.
- `integration_architect`: проверить runtime/storage/CSI/control-plane
  boundaries against virtualization-style integration patterns.

### 3. Slices

#### Slice 1. Снять фактическую карту controller tree

Цель:

- получить current package/file map;
- найти расхождения между `STRUCTURE.ru.md` и кодом.

Файлы:

- read-only `images/controller/**`

Проверки:

- `find images/controller -maxdepth 4 -type d`
- `find images/controller -maxdepth 4 -type f`
- `rg` по stale vocabulary in `STRUCTURE.ru.md`

#### Slice 2. Архитектурный review и решение по documentation shape

Цель:

- зафиксировать, какие детали должны попасть в `STRUCTURE.ru.md`;
- отделить durable architecture map от неустойчивого per-function inventory.

Файлы:

- `plans/active/controller-structure-architecture-parity/PLAN.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- manual consistency with subagent outputs.

#### Slice 3. Update `STRUCTURE.ru.md`

Цель:

- обновить live package map и descriptions;
- убрать statements, которые называют уже реализованный CSI/shared-direct путь
  будущим;
- добавить новые entrypoints and dataplane boundaries.

Файлы:

- `images/controller/STRUCTURE.ru.md`

Проверки:

- `rg -n "future|будущ|ai-models-node-cache-runtime|nodecachecsi|nodecacheruntime|cleanupstate|archiveio|publishplan" images/controller/STRUCTURE.ru.md`

#### Slice 4. Узкий cleanup, если аудит найдёт очевидный drift

Цель:

- исправить только small, low-risk architecture drift без product/API change.

Файлы:

- `images/controller/internal/controllers/nodecacheruntime/options.go`
- `images/controller/internal/controllers/nodecacheruntime/options_test.go`
- `images/controller/internal/controllers/nodecacheruntime/reconciler.go`
- `images/controller/cmd/ai-models-controller/config.go`
- `images/controller/cmd/ai-models-controller/config_test.go`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/webhook.yaml`
- `tools/helm-tests/validate-renders.py`
- `tools/helm-tests/validate_renders_test.py`

Проверки:

- targeted `go test` по затронутым packages.

Решение:

- `nodeCache.maxSize` остаётся storage-substrate budget для SDS thin-pool.
- Runtime eviction budget не должен превышать per-node shared PVC, поэтому
  `NodeCacheRuntime.MaxTotalSize` берётся из `nodeCache.sharedVolumeSize`, а
  runtime options валидируют `MaxTotalSize <= SharedVolumeSize`.
- Workload delivery webhook должен быть fail-closed для annotated workloads:
  иначе annotated Pod template может уйти без model delivery mutation.
- `sharedVolumeSize <= maxSize` проверяется и в controller startup, и в Helm
  render validation, чтобы не отдавать заведомо невалидный manifest в кластер.
- Empty `internal/application/publishplan` удаляется как placeholder без
  use-case contract.

#### Slice 5. Зафиксировать follow-up долги, которые нельзя смешивать с этим patch

Цель:

- не прятать крупные архитектурные долги внутри docs cleanup;
- оставить next slices явными.

Follow-up:

- split `upload-gateway` из controller Deployment в отдельный Deployment,
  Service, Ingress и ServiceAccount с узким namespace-scoped доступом к
  upload-session Secrets и object-storage credentials;
- split publication worker identity: не использовать controller ServiceAccount
  для short-lived worker Pods;
- заменить expensive runtimehealth scrape over all Pods/workloads на
  controller-maintained or indexed narrow status source;
- если desired-artifact projection между `k8s/nodecacheruntime` и
  `k8s/modeldelivery` начнёт расти, вынести projection type в отдельный shared
  contract вместо adapter-to-adapter импорта.

### 4. Rollback point

После Slice 3 можно оставить только docs update без code cleanup: это безопасно
и не меняет runtime behavior.

### 5. Final validation

- `git diff --check && git diff --cached --check`
- targeted `go test` if code changed
- `make verify` if code/templates changed or if final state is close to release.

### 6. Subagent findings

`repo_architect`:

- package topology is mostly defensible, but `STRUCTURE.ru.md` was stale and
  must describe boundaries, not every file/function;
- empty application placeholders are not architecture and should be removed;
- current cross-adapter desired-artifact projection is acceptable only while it
  remains narrow.

`integration_architect`:

- node-cache SDS/CSI/runtime split is close to virtualization-style layering;
- upload-gateway and publication-worker identities are still too broad and need
  separate slices;
- workload delivery mutation should fail closed for annotated workloads;
- runtime cache budget must not exceed the shared node-cache volume.

### 7. Reviewer follow-up

Reviewer findings resolved in this slice:

- full repository diff contains unrelated pre-existing dirty hunks in the same
  files (`docs/CONFIGURATION*`, `openapi/*`, `templates/controller/webhook.yaml`);
  this bundle owns only the node-cache size semantics, `STRUCTURE` parity,
  webhook fail-closed policy and render/test guardrails added here;
- `sharedVolumeSize <= maxSize` is now render-time fail-fast in
  `templates/_helpers.tpl`, not only controller startup validation;
- `STRUCTURE.ru.md` current-state verdict no longer contains a date;
- focused render validator now checks fail-closed workload delivery webhook and
  non-empty `caBundle`;
- targeted delivery tests now include `workloaddelivery` and `modeldelivery`.
