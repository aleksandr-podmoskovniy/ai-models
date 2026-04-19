## 1. Current phase

Этап 2. Public `Model` / `ClusterModel` source intent не меняется. Это
implementation continuation для node-local cache substrate и будущего runtime
delivery поверх уже опубликованных OCI artifacts.

## 2. Orchestration

`full`

Причина:

- задача меняет runtime/config/RBAC/docs surface;
- она одновременно затрагивает storage substrate, controller wiring и repo
  structure;
- по правилам репозитория сюда просится read-only delegation, но в текущей
  сессии она не используется, поэтому архитектурная дисциплина фиксируется
  прямо в bundle и финальном review.

## 3. Slices

### Slice 1. Зафиксировать continuation bundle и substrate contract

Цель:

- перевести architecture predecessor в implementation bundle без смешения с
  current `phase2-runtime-followups`.

Файлы/каталоги:

- `plans/active/node-local-cache-runtime-delivery/*`
- `plans/active/phase2-model-distribution-architecture/*`

Проверки:

- manual consistency review

Артефакт результата:

- compact bundle с explicit managed-substrate-first scope.

### Slice 2. Добавить managed node-cache substrate config и controller wiring

Цель:

- ввести cluster-level config и bootstrap/RBAC surface для managed substrate.

Файлы/каталоги:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/bootstrap/*`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller ./internal/bootstrap`

Артефакт результата:

- controller получает validated substrate options and wiring.

### Slice 3. Реализовать managed substrate controller и K8s adapter

Цель:

- дать ai-models owner controller над `LVMVolumeGroupSet` и
  `LocalStorageClass`.

Файлы/каталоги:

- `images/controller/internal/controllers/nodecachesubstrate/*`
- `images/controller/internal/adapters/k8s/nodecachesubstrate/*`

Проверки:

- `cd images/controller && go test ./internal/controllers/nodecachesubstrate ./internal/adapters/k8s/nodecachesubstrate`

Артефакт результата:

- ai-models держит managed substrate CRs без render-time guessing.

### Slice 4. Синхронизировать docs, structure и evidence

Цель:

- честно описать live state после landed substrate slice и не выдать его за
  уже готовый node-cache runtime.

Файлы/каталоги:

- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- docs/evidence совпадают с landed substrate-first shape.

### Slice 5. Добавить managed local fallback volume для current workload delivery

Цель:

- убрать требование к annotated workload заранее приносить local cache mount,
  если ai-models already owns node-local `LocalStorageClass`.

Файлы/каталоги:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`

Артефакт результата:

- current init-container fallback умеет auto-inject managed local ephemeral
  volume on `/data/modelcache` while preserving user-provided cache topology.

### Slice 6. Вынести shared node-cache contract из command-local materialize shell

Цель:

- перестать держать cache layout, marker и coordination внутри
  `cmd/ai-models-artifact-runtime`, чтобы будущий node-cache runtime и eviction
  logic опирались на одну reusable boundary.

Файлы/каталоги:

- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/nodecache`

Артефакт результата:

- `materialize-artifact` становится thin entrypoint, а shared cache-root
  behavior живёт в отдельном `internal/nodecache` package tree.

### Slice 7. Добавить bounded node-cache scan и eviction planning surface

Цель:

- подготовить future node-cache runtime к idle cleanup без вывода мёртвого
  public knob и без смешения planner logic с CLI/runtime entrypoint.

Файлы/каталоги:

- `images/controller/internal/nodecache/*`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/nodecache`

Артефакт результата:

- node-cache tree уже умеет систематически читать cache-root state и строить
  eviction candidates по size/age without pretending that cluster cleanup is
  already active.

### Slice 8. Добавить controller-owned node-cache runtime plane

Цель:

- посадить первый реальный per-node runtime поверх managed `LocalStorageClass`,
  чтобы bounded maintenance loop уже исполнялся отдельным cache plane, а не
  оставался только library contract.

Файлы/каталоги:

- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/bootstrap/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/support/resourcenames/*`
- `templates/controller/rbac.yaml`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/nodecache ./cmd/ai-models-artifact-runtime ./internal/controllers/nodecacheruntime ./internal/adapters/k8s/nodecacheruntime ./internal/bootstrap ./cmd/ai-models-controller`

Артефакт результата:

- ai-models держит bounded per-node maintenance plane поверх local PVC, а
  shared cache cleanup и planning больше не зависят от будущего mount service
  или от `materialize-artifact` fallback.

### Slice 9. Разрезать shared store и consumer current-link contract

Цель:

- убрать оставшееся смешение node-wide digest store и workload-local
  `current` symlink surface внутри `internal/nodecache`, чтобы будущий mount
  service не наследовал single-model assumption от current fallback path.

Файлы/каталоги:

- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/nodecache ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/modeldelivery ./internal/adapters/modelpack/oci`

Артефакт результата:

- shared digest store contract и consumer materialization layout получили
  отдельные names/seams, а node-cache runtime больше не выглядит так, будто
  global `current` symlink является canonical store truth.

### Slice 10. Нормализовать node-cache runtime plane к module-owned DaemonSet

Цель:

- убрать custom per-node runtime controller/PVC ownership shell и перевести
  node-cache maintenance plane на стандартный `DaemonSet` с generic ephemeral
  volume поверх ai-models-owned `LocalStorageClass`, чтобы следующий CSI slice
  уже опирался на нормальную node-agent форму, а не на самодельный reconcile.

Файлы/каталоги:

- `plans/active/node-local-cache-runtime-delivery/*`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `templates/node-cache-runtime/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/bootstrap/*`
- `images/controller/internal/support/resourcenames/*`
- `tools/helm-tests/*`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller ./internal/bootstrap ./internal/support/resourcenames`
- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `make kubeconform`

Артефакт результата:

- node-cache runtime plane больше не создаётся controller-owned per-node Pod/PVC
  loop'ом;
- module render держит один `DaemonSet` с generic ephemeral volume и bounded
  maintenance env contract;
- service-volume request у runtime plane больше не съедает весь node-cache
  thin-pool budget и не конфликтует по умолчанию с current fallback path;
- controller bootstrap/RBAC вычищены от node-runtime-only ownership, а bundle
  не держит legacy custom controller surface перед следующим CSI-like step.

### Slice 11. Добавить internal node-cache intent plane и shared-store prefetch

Цель:

- дать node-agent plane первый реальный desired-artifact contract, чтобы
  `node-cache-runtime` уже не обслуживал только maintenance loop над пустым
  store, а prefetch'ил published artifacts по реально запланированным managed
  workload pod'ам без нового public API.

Файлы/каталоги:

- `plans/active/node-local-cache-runtime-delivery/*`
- `images/controller/internal/nodecacheintent/*`
- `images/controller/internal/adapters/k8s/nodecacheintent/*`
- `images/controller/internal/controllers/nodecacheintent/*`
- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/bootstrap/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/support/resourcenames/*`
- `templates/controller/*`
- `templates/node-cache-runtime/*`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `tools/helm-tests/*`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

Проверки:

- `cd images/controller && go test ./internal/nodecacheintent ./internal/adapters/k8s/nodecacheintent ./internal/controllers/nodecacheintent`
- `cd images/controller && go test ./internal/nodecache ./cmd/ai-models-artifact-runtime`
- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/bootstrap ./cmd/ai-models-controller`
- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `make kubeconform`

Артефакт результата:

- managed workload pod template теперь несёт immutable published artifact
  identity, пригодную для downstream node-local cache intent extraction;
- ai-models держит отдельный internal controller-owned per-node intent plane
  вместо implicit discovery inside the node agent;
- `node-cache-runtime` получает реальный prefetch path в shared digest store;
- eviction planner умеет защищать currently desired digests от idle/size
  pressure eviction по умолчанию;
- `nodeCache` contract получает live shared-store volume budget вместо прежнего
  внутреннего service-volume placeholder.

## 4. Rollback point

После Slice 2 можно безопасно остановиться: config и wiring уже видны, но
managed substrate controller ещё не влияет на внешние storage CR.

После Slice 3 можно откатиться без поломки workload delivery: новый substrate
plane уже существует, но runtime mount path всё ещё остаётся на текущем
`materialize-artifact` fallback.

После Slice 5 можно откатиться без потери substrate plane: managed fallback
volume injection поверх current workload delivery уже живёт отдельно от
node-cache runtime service и не требует нового public `Model.spec`.

После Slice 7 можно откатиться без потери current delivery UX: shared
node-cache contract уже вынесен в reusable boundary, но cluster-level cache
daemon/CSI mount service всё ещё не обязательны для текущего fallback path.

После Slice 8 можно откатиться без ломки current delivery path: первый runtime
plane уже существует, но workload-facing shared mount contract всё ещё не
заменяет текущий init-container fallback.

После Slice 9 можно откатиться без потери landed runtime plane: cleanup
затронет только boundary names и internal split shared-store vs consumer-link
surfaces.

После Slice 10 можно откатиться без ломки substrate/fallback contract: сменится
только форма node maintenance plane, а shared cache semantics и current
workload fallback останутся прежними.

После Slice 11 можно откатиться без ломки current workload delivery path:
shared-store prefetch и node intent plane уйдут, но substrate, `DaemonSet`
shape и current fallback volume contract останутся рабочими.

## 5. Final validation

- `cd images/controller && go test ./cmd/ai-models-controller ./internal/bootstrap ./internal/controllers/nodecachesubstrate ./internal/adapters/k8s/nodecachesubstrate`
- `cd images/controller && go test ./cmd/ai-models-controller ./internal/bootstrap ./internal/controllers/nodecachesubstrate ./internal/adapters/k8s/nodecachesubstrate ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`
- `cd images/controller && go test ./cmd/ai-models-artifact-runtime ./internal/nodecache`
- `cd images/controller && go test ./internal/nodecache ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/modeldelivery ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./cmd/ai-models-controller ./internal/bootstrap ./internal/support/resourcenames`
- `cd images/controller && go test ./internal/nodecacheintent ./internal/adapters/k8s/nodecacheintent ./internal/controllers/nodecacheintent`
- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `make kubeconform`
- `make verify`
- `git diff --check`
