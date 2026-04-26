## 1. Current phase

Этап 2. Каноническая публикация в `DMCR` уже зафиксирована. Текущий workstream
закрывает именно runtime topology: единственный целевой узловой путь доставки
модели и уход от текущего per-workload materialize bridge.

## 2. Orchestration

`full`

Причина:

- задача одновременно затрагивает runtime delivery, storage substrate,
  controller wiring, values/OpenAPI, эксплуатационные сигналы и документацию;
- здесь легко снова размыть границы между публикацией, узловым кэшем и
  доставкой модели в прикладной объект;
- финальная форма должна быть defendable как production-ready единая topology,
  а не как семейство переходных materialize-режимов.

Read-only reviews, обязательные перед большими implementation slices:

- `repo_architect`
  - проверить, что новый путь не превращает `images/controller` в giant mixed
    runtime tree;
- `integration_architect`
  - проверить границы между storage substrate, узловым runtime plane,
    доставкой в прикладной объект и наблюдаемостью;
- `api_designer`
  - проверить, что runtime/storage policy не протаскивается в публичный
    `Model` / `ClusterModel` контракт.

## 3. Slices

### Slice 1. Перефиксировать workstream и заморозить единственную целевую схему

Цель:

- оставить один канонический active bundle, который ясно описывает:
  - текущее состояние;
  - единственную целевую схему;
  - инварианты, которые нельзя потом размыть в коде.

Файлы/каталоги:

- `plans/active/node-local-cache-runtime-delivery/*`

Проверки:

- manual consistency review against:
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/internal/controllers/workloaddelivery/*`
  - `images/controller/internal/controllers/nodecacheruntime/*`

Артефакт результата:

- compact bundle с одной целевой картиной, где текущий materialize path
  описан только как временный bridge.

### Slice 2. Вычистить vocabulary drift и остаточные ложные seams

Цель:

- привести код, docs и bundle к одному словарю:
  - `узловой общий кэш`;
  - `стабильный workload-facing runtime-контракт`;
  - `переходный materialize bridge`;
- удалить или локализовать surfaces, которые больше не участвуют в живом пути
  и только создают архитектурный шум.

Файлы/каталоги:

- `images/controller/internal/nodecache*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `rg -n "materialize-artifact|nodecacheintent|shared mount|AI_MODELS_MODEL_PATH|bridge|fallback" images/controller docs/CONFIGURATION.ru.md`
- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/controllers/nodecacheruntime ./internal/nodecache`

Артефакт результата:

- один словарь и один объяснимый contract surface без старых полумёртвых
  narrative seams.

### Slice 3. Довести единственную целевую topology через узловой общий кэш

Цель:

- сделать так, чтобы при доступном узловом кэше прикладной объект получал
  модель из node-owned shared plane, а не через отдельную полную раскладку в
  собственный том.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/nodecache/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime ./internal/nodecache ./cmd/ai-models-artifact-runtime`

Артефакт результата:

- workload-facing shared delivery path, который переиспользует одну и ту же
  модель на ноде и не требует per-workload full materialization при готовом
  узловом кэше.

### Slice 4. Подготовить cutover и удаление долгоживущего bridge narrative

Цель:

- убрать из target surface представление о per-workload materialization как о
  втором supported mode и подготовить controlled cutover к единственной
  topology.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/cmd/ai-models-controller/*`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `make helm-template`

Артефакт результата:

- target docs, values и controller surfaces больше не описывают переходный
  materialize bridge как равноправную product topology.

### Slice 5. Дотянуть сигналы наблюдаемости и эксплуатационный контур

Цель:

- дать оператору ясный ответ:
  - в каком состоянии узловой runtime plane;
  - можно ли считать целевую topology реально готовой на конкретной ноде;
  - остались ли ещё workload'и, живущие на временном materialize bridge.

Файлы/каталоги:

- `images/controller/internal/monitoring/runtimehealth/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

Проверки:

- `cd images/controller && go test ./internal/monitoring/runtimehealth ./internal/controllers/workloaddelivery ./internal/controllers/nodecacheruntime`

Артефакт результата:

- production-readable signals по состоянию узлового слоя и cutover readiness.

### Slice 6. Синхронизировать docs, values и repo surface с итоговой схемой

Цель:

- привести values, OpenAPI, docs и controller structure к одному финальному
  описанию без обещаний того, чего ещё нет, без legacy wording drift и без
  narrative про постоянный переходный режим.

Файлы/каталоги:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `make helm-template`
- `make kubeconform`

Артефакт результата:

- repo-local surfaces совпадают с тем, что реально делает код.

## 4. Rollback point

После Slice 2 можно безопасно остановиться:

- bundle уже канонически фиксирует цель;
- словарь и boundaries вычищены;
- текущий живой materialize bridge ещё не ломался;
- быстрый путь ещё не частично внедрён.

Текущий landed шаг по Slice 2:

- live delivery mode/reason переименованы из `PerPodFallback` /
  `ManagedFallbackVolume` в `MaterializeBridge` / `ManagedBridgeVolume`;
- current shared PVC path больше не притворяется настоящим `SharedDirect`:
  live controller теперь держит его как отдельный `SharedPVCBridge`, а
  будущий `SharedDirect` остаётся зарезервированным для node-owned shared
  delivery;
- `node-cache-runtime` больше не должен трактовать current shared PVC bridge
  как workload-facing shared plane для `prefetch`;
- managed ephemeral volume и docs теперь описываются как переходный
  materialize bridge, а не как полноценный fallback-режим;
- live runtime semantics при этом не менялись: per-workload
  `materialize-artifact` остаётся текущим bridge до следующего cutover slice.

Это последняя безопасная точка перед изменением live runtime semantics.

## 5. Final validation

- узкие `go test` по затронутым пакетам после каждого slice;
- `make helm-template`;
- `make kubeconform`;
- `make verify`.

## 6. Continuation 2026-04-25: SDS/CSI render guardrail

Current slice is a bounded correctness guardrail before changing live runtime
semantics.

### Slice 2a. Явно зафиксировать SDS-backed CSI prerequisites

Цель:

- не давать включить `nodeCache` в render без Deckhouse SDS modules, которые
  создают нужные `storage.deckhouse.io` CRD и CSI-backed `LocalStorageClass`;
- не откладывать пустые `nodeSelector` / `blockDeviceSelector` до controller
  startup crash-loop;
- зафиксировать, что pre-cutover workload bridge уже опирался на
  SDS-backed `LocalStorageClass`, но это ещё не было финальным
  workload-facing shared-direct mount.

Файлы/каталоги:

- `templates/_helpers.tpl`
- `templates/module/validation.yaml`
- `fixtures/render/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- текущий `plans/active/node-local-cache-runtime-delivery/NOTES.ru.md`

Проверки:

- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`

Артефакт результата:

- render fails fast for invalid node-cache SDS prerequisites;
- node-cache enabled fixture renders the intended SDS/CSI-backed bridge
  surface;
- docs do not overclaim final shared-direct delivery.

Статус 2026-04-25:

- implemented in templates/docs/render validation;
- implemented the same non-empty selector guard in controller flag/env config
  parsing, so non-Helm starts fail before bootstrap too;
- `make helm-template` passed;
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
  passed;
- `python3 tools/helm-tests/validate_renders_test.py` passed;
- `cd images/controller && go test ./cmd/ai-models-controller` passed;
- `git diff --check && git diff --cached --check` passed;
- `make lint-codex-governance` passed;
- `make kubeconform` was attempted, but hung inside
  `kubeconform -schema-location default` without output and was terminated.

## 7. Continuation 2026-04-25: workload-facing SharedDirect cutover

Current slice changes live default semantics for managed `nodeCache` delivery.

### Slice 3a. Перевести managed workload path на inline CSI SharedDirect

Цель:

- при `nodeCache.enabled=true` больше не inject'ить прежний per-workload
  bridge storage как default managed workload cache;
- inject'ить workload-facing inline CSI volume with digest/artifact attributes
  and stable `AI_MODELS_MODEL_PATH=/data/modelcache/model`;
- проставлять `SharedDirect` / `NodeSharedRuntimePlane` annotations so the
  node-cache runtime sees live managed Pods as desired artifacts;
- не проецировать DMCR read auth/CA and runtime image pull secret into workload
  namespaces for SharedDirect, because pulling happens in the node-cache plane;
- propagate configured node-cache `nodeSelector` into managed workloads, and
  fail on conflicting selectors instead of scheduling onto unsupported nodes.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/node-cache-runtime/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check && git diff --cached --check`

Артефакт результата:

- managed workload default becomes `SharedDirect`;
- legacy per-workload materialize bridge remains only for explicitly supplied
  workload cache volumes until the removal slice.

Статус 2026-04-25:

- implemented managed workload cutover to inline CSI
  `node-cache.ai-models.deckhouse.io` with artifact URI/digest/family
  attributes;
- implemented cleanup/pruning for stale workload-namespace registry
  projection and runtime imagePullSecret on SharedDirect reconcile;
- implemented node-cache `nodeSelector` propagation with fail-closed selector
  conflict handling;
- implemented refresh of existing managed CSI volume attributes so model
  digest changes cannot leave stale inline CSI attributes in the pod template;
- rendered `CSIDriver` identity with `volumeLifecycleModes: [Ephemeral]`;
- removed `nodeCache.fallbackVolumeSize` from controller flags/env and
  values/OpenAPI surfaces;
- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
  passed;
- `python3 tools/helm-tests/validate_renders_test.py` passed;
- `make helm-template` passed;
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
  passed;
- stale-term scan for fallback/generic-ephemeral/future-shared-direct wording
  passed.
- `git diff --check && git diff --cached --check` passed;
- `make lint-codex-governance` passed.

Review-gate residual risks:

- This slice implements controller/template/docs cutover to the workload-facing
  inline CSI contract, but it does not yet implement the kubelet-facing CSI
  node plugin socket/registration and bind-mount server. A live rollout with
  `nodeCache.enabled=true` still requires that runtime slice before managed
  Pods can mount `node-cache.ai-models.deckhouse.io`.
- `make kubeconform` / `make verify` were not rerun in this slice because the
  previous `make kubeconform` attempt hung in `kubeconform -schema-location
  default`; narrow Helm render validation was used instead.

## 8. Continuation 2026-04-25: kubelet-facing CSI node runtime

Current slice closes the critical residual from Slice 3a. It is bounded to the
node-cache runtime entrypoint and the controller-owned per-node runtime Pod.

### Slice 3b. Реализовать CSI node socket/registration/bind-mount server

Цель:

- existing controller-owned per-node node-cache runtime Pod must expose the
  kubelet-facing CSI socket for `node-cache.ai-models.deckhouse.io`;
- runtime Pod must follow Deckhouse/SDS CSI node registration pattern:
  `node-driver-registrar` sidecar, `/var/lib/kubelet/csi-plugins/<driver>`,
  `/var/lib/kubelet/plugins_registry`, privileged node container and
  bidirectional kubelet mount;
- CSI NodePublish must bind-mount the ready digest store directory from the
  shared node-cache PVC into kubelet target path, so workloads see stable
  `/data/modelcache/model`;
- if the digest is not ready yet, NodePublish returns transient
  `UNAVAILABLE`, leaving kubelet retry/requeue to cover the gap while the
  runtime prefetch loop materializes the artifact;
- public `Model` / `ClusterModel` API stays unchanged, and storage details do
  not leak into user-facing spec.

Файлы/каталоги:

- `images/controller/internal/nodecache/*`
- `images/controller/internal/dataplane/nodecachecsi/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/_helpers.tpl`
- `templates/controller/deployment.yaml`
- `tools/helm-tests/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/nodecachecsi ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check && git diff --cached --check`
- `make lint-codex-governance`

Артефакт результата:

- `nodeCache.enabled=true` has an actual kubelet-facing node plugin path rather
  than only the workload template cutover;
- runtime delivery remains a single node-cache topology, not a reintroduced
  per-workload materialization mode.

Статус 2026-04-25:

- implemented `node-cache.ai-models.deckhouse.io` CSI Identity/Node server in
  the node-cache runtime binary;
- implemented read-only digest bind-mount in CSI `NodePublishVolume` with
  transient `UNAVAILABLE` while the digest is not ready;
- added fail-closed publish authorization through kubelet `podInfoOnMount`:
  requesting Pod UID/node/digest must match a managed `SharedDirect`
  desired-artifact Pod;
- shaped the controller-owned runtime Pod with Deckhouse/SDS-style CSI node
  registration: `node-driver-registrar`, kubelet plugin dir,
  plugin-registry dir, privileged runtime container and bidirectional kubelet
  mount;
- added controller config and Helm wiring for the Deckhouse common
  `csiNodeDriverRegistrar` image;
- documented the runtime contract and RBAC evidence for pod `get/list` without
  granting Secrets, pod exec/attach/portforward, status or finalizers;
- `cd images/controller && go test ./internal/dataplane/nodecachecsi ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
  passed;
- `python3 tools/helm-tests/validate_renders_test.py` passed;
- `make helm-template` passed;
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
  passed;
- `git diff --check && git diff --cached --check` passed;
- `make lint-codex-governance` passed;
- `make verify` passed.

Review-gate outcome:

- critical findings: none;
- residual live-rollout risk: existing node-cache runtime Pods must be
  recreated after applying this version, otherwise kubelet will still not have
  the registrar/socket path on already running runtime Pods;
- operational residual: `NodePublishVolume` intentionally returns transient
  `UNAVAILABLE` while a digest is not ready, so first workload mount can wait
  for the node-cache prefetch loop instead of falling back to per-workload
  materialization.

### Slice 3c. Исправить привязку runtime Pod к ноде для local LVM PVC

Цель:

- убрать прямой `spec.nodeName` у node-cache runtime Pod;
- оставить строгую привязку runtime Pod к целевой ноде через scheduler-visible
  node affinity по `kubernetes.io/hostname`;
- не ломать `WaitForFirstConsumer` у managed `LocalStorageClass`, потому что
  локальный LVM volume должен выбираться планировщиком по Pod+PVC вместе.

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime`
- `git diff --check && git diff --cached --check`

Статус 2026-04-25:

- removed direct `spec.nodeName` from the node-cache runtime Pod;
- added required node affinity by `kubernetes.io/hostname`, using the actual
  node label value when present and falling back to the Node object name;
- kept runtime identity as the Kubernetes Node object name through
  `AI_MODELS_NODE_NAME`;
- documented SDS-disabled / missing-local-disk failure modes;
- `cd images/controller && go test ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime`
  passed;
- `git diff --check && git diff --cached --check` passed.

Review-gate outcome:

- critical findings: none for the `WaitForFirstConsumer` fix;
- residual scheduling guardrail: managed workloads still inherit
  `nodeCache.nodeSelector`, not a dynamic "node-cache runtime ready" node
  label. If the selector includes nodes without matching local disks, workload
  Pods can still be scheduled there and then fail/hang on CSI mount because no
  node-cache runtime socket is registered on that node. The current intended
  operational contract is that `nodeCache.nodeSelector` selects only nodes that
  can satisfy `nodeCache.blockDeviceSelector`; a later hardening slice should
  add explicit runtime-readiness scheduling gates if that assumption is not
  acceptable.

### Slice 3d. Harden node-cache runtime image, retry and placement gate

Цель:

- отделить node-cache CSI/runtime plane от общего `controllerRuntime` image как
  внутренний dedicated image без нового user-facing values knob;
- оставить код внутри `images/controller`, без нового top-level module или
  shadow runtime tree;
- заменить падение всего node-cache runtime loop на ошибке одного digest на
  per-digest retry/backoff внутри `internal/nodecache`;
- добавить controller-owned readiness label на Node только когда runtime Pod
  реально Running/Ready, а workload delivery должен требовать этот label
  вместе с configured `nodeCache.nodeSelector`;
- не вводить новый CRD, зеркальный desired-state plane или публичный retry
  policy.

Read-only review:

- `repo_architect` completed before implementation:
  - dedicated image must move build, controller config, Helm wiring and tests
    together;
  - retry/backoff belongs in `internal/nodecache`, not CLI/K8s adapters;
  - placement policy belongs on controller/workloaddelivery side, not CSI node
    server;
  - do not add public node-cache image/retry/placement knobs.
- `integration_architect` was started before implementation and returned during
  the slice; findings confirmed the same boundaries: retry must stay inside the
  digest loop, the image split must not retarget publication workers, and
  placement readiness must be computed from live Pod/PVC/Node state rather than
  CSI server or metrics.

Файлы/каталоги:

- `images/controller/werf.inc.yaml`
- `images/controller/cmd/ai-models-node-cache-runtime/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/nodecache/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/controller/deployment.yaml`
- `fixtures/module-values.yaml`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/nodecache ./cmd/ai-models-node-cache-runtime ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check && git diff --cached --check`
- `make lint-codex-governance`

Артефакт результата:

- node-cache runtime runs from a dedicated distroless internal image;
- transient pull/materialization errors do not restart the node-cache runtime
  Pod and do not block other ready/missing digests;
- managed SharedDirect workloads are schedulable only to nodes that match the
  configured node-cache selector and carry the dynamic node-cache runtime ready
  label.

Статус 2026-04-25:

- added a dedicated `cmd/ai-models-node-cache-runtime` entrypoint and
  `nodeCacheRuntime` distroless image; managed node-cache runtime Pods use this
  image, while `controllerRuntime` stays scoped to publication/upload/legacy
  materialize commands;
- removed the node-cache command path from `ai-models-artifact-runtime`, so the
  shared publication/materialize runtime image no longer serves the privileged
  node-cache plane;
- added per-digest in-memory retry/backoff inside `internal/nodecache`; a
  failed digest no longer aborts the whole runtime loop and no longer blocks
  other digest prefetches;
- added a controller-owned Node readiness label
  `ai.deckhouse.io/node-cache-runtime-ready=true`, based on a Running/Ready
  runtime Pod on that node and a Bound shared cache PVC;
- managed SharedDirect workload templates now require the configured
  `nodeCache.nodeSelector` plus the dynamic runtime-ready label and keep the
  ai-models scheduling gate while no ready node exists;
- fixed the per-node runtime Pod compare to ignore scheduler-owned
  `spec.nodeName`, preventing churn after switching to scheduler-visible node
  affinity;
- kept node-cache runtime RBAC narrow; only the controller RBAC gained `patch`
  on Nodes to maintain the readiness label.

Проверки:

- `cd images/controller && go test ./internal/nodecache ./internal/dataplane/nodecacheruntime ./cmd/ai-models-node-cache-runtime ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/nodecacheruntime ./internal/controllers/nodecacheruntime ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./cmd/ai-models-controller`
  passed;
- `python3 tools/helm-tests/validate_renders_test.py` passed;
- `make helm-template` passed;
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
  passed;
- `make kubeconform` passed;
- `git diff --check && git diff --cached --check` passed;
- `make lint-codex-governance` passed;
- `make verify` passed.

Review-gate notes:

- no public `ModuleConfig`/OpenAPI knob was added for node-cache runtime image,
  retry policy or placement policy;
- no new CRD, mirrored intent plane or Prometheus-driven control path was
  introduced;
- remaining operational limitation: the readiness label proves runtime Pod +
  shared PVC readiness, not per-digest free-capacity admission. Free-space-aware
  scheduling is a larger future slice if needed.
