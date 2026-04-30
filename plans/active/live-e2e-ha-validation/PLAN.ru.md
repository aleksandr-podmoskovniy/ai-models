# Plan: live e2e/HA validation after rollout

## 1. Current phase

Phase 1/2 validation. Цель — доказать, что текущий publication/runtime baseline
готов к тяжёлому live-прогону и что новые доработки не остались только
unit/template-level evidence.

## 2. Orchestration

Режим: `full` при фактическом запуске.

Перед destructive HA actions нужны read-only reviews:

- `integration_architect` — live safety, placement, storage, node-cache,
  observability, rollback.
- `backend_integrator` — DMCR/direct-upload/mirror/upload/GC failure modes.
- `api_designer` — если в ходе run будут найдены CRD/status/capability
  расхождения.
- `reviewer` — если будут code/template fixes после e2e.

Текущий шаг — подготовка runbook, без live destructive actions.

## 3. Active bundle disposition

Keep active:

- `live-e2e-ha-validation` — канонический executable runbook для следующего
  live запуска. Historical evidence `k8s-apiac-20260429-001154` перенесён в
  `plans/archive/2026/live-e2e-ha-validation-legacy-evidence/`, потому что он
  относится к старому delivery contract и больше не является исполняемой
  матрицей.
- `observability-signal-hardening` — остаётся executable: pending Slice 4
  должен проверить live metrics/alerts и добить log field dictionary.
- `ray-a30-ai-models-registry-cutover` — archived: KubeRay-specific delivery
  больше не является целевой архитектурой. A30 проверка должна идти через
  generic PodTemplate CSI contract или plain Deployment/vLLM.

Archived:

- `capacity-cache-admission-hardening` — implementation завершён; live proof
  покрывается этим e2e runbook.
- `pre-rollout-defect-closure` — defect closure завершён и больше не является
  рабочей поверхностью.
- `public-docs-virtualization-style` — public docs slice завершён; следующие
  docs правки принадлежат feature-specific bundles.
- `source-capability-taxonomy-ollama` — Ollama registry implementation закрыт;
  live proof покрывается этим e2e runbook.
- `pod-placement-policy` — placement slice завершён и покрывается этим e2e
  runbook.

## 4. Start gate

Live run started after explicit user command on 2026-04-30. Scope for this run:
`k8s.apiac.ru`, managed node-cache enablement first, then plain
Deployment-based A30 validation for embedder, reranker and Whisper STT. KubeRay
resources are not used in this run: they are an integration target, not the
model-delivery contract.

Перед стартом зафиксировать:

- `git status -sb`;
- последний commit и образ/rollout, который реально стоит в `k8s.apiac.ru`;
- текущий `ModuleConfig ai-models`;
- текущие nodes/taints/labels;
- текущий список pods/deployments/events в `d8-ai-models`;
- текущие usage/capacity metrics, если доступны;
- текущие active `Model`/`ClusterModel` во всех namespaces.

Live mutation/config changes are allowed only inside this runbook scope:

- `ModuleConfig ai-models.spec.settings.nodeCache` may be enabled after SDS,
  `BlockDevice`, `LocalStorageClass` and target node checks;
- test namespace/resources must be labelled
  `ai.deckhouse.io/live-e2e=ha-validation`;
- workload manifests must be annotation-only from the ai-models contract point
  of view: no hand-written registry credentials, materialize init containers or
  internal artifact URIs.

## 5. Execution slices

### Slice 1. Rollout, placement and baseline

Цель:

- доказать, что новая сборка выкачена;
- проверить, что controller/upload-gateway/DMCR не держатся на
  master/control-plane после placement fix;
- зафиксировать baseline для последующего сравнения.

Проверки:

- `kubectl --context k8s.apiac.ru get module ai-models -o yaml`
- `kubectl --context k8s.apiac.ru -n d8-ai-models get deploy,pod -o wide`
- `kubectl --context k8s.apiac.ru get nodes --show-labels`
- `kubectl --context k8s.apiac.ru -n d8-ai-models get events --sort-by=.lastTimestamp`

Pass:

- module Ready;
- no growing restartCount;
- controller/upload-gateway rendered/live placement does not target or tolerate
  master/control-plane;
- no stale e2e resources from previous runs.

### Slice 1A. A30 node-cache enablement

Цель:

- включить managed node-cache только на A30 ноде с локальными дисками;
- доказать, что SDS/local-volume substrate создан и node-cache runtime готов;
- не затрагивать другие GPU/worker nodes.

Проверки:

- найти target node по фактическим GPU/SDS labels and allocatable resources;
- проверить `BlockDevice` на target node и label
  `ai.deckhouse.io/model-cache=true`;
- включить `nodeCache.enabled=true` с selector на target node;
- дождаться `LocalStorageClass`, `LVMVolumeGroupSet`, `LVMVolumeGroup`,
  runtime `PVC` and node-cache runtime Pod;
- проверить label `ai.deckhouse.io/node-cache-runtime-ready=true` на target
  node.

Pass:

- на target node есть ready node-cache runtime and bound PVC;
- на остальных nodes node-cache runtime не создан;
- controller logs не содержат cluster-scope Secret informer/RBAC errors.

Rollback:

- удалить test workloads;
- вернуть `nodeCache.enabled=false` in `ModuleConfig ai-models`;
- удалить только module-owned node-cache resources, если они остались
  orphaned and имеют labels/owner identity ai-models.

### Slice 2. Observability and log capture preflight

Цель:

- до создания моделей подготовить evidence capture;
- убедиться, что короткоживущие source-worker/upload artifacts можно прочитать
  после completion.
- проверить, что новый observability hardening реально виден в live scrape, а
  не только в unit tests.

Проверки:

- controller logs include reconcile owner/name/source mode;
- upload-gateway logs include session/reservation/publication ids without
  secrets;
- DMCR logs include maintenance/direct-upload/GC fields;
- metrics scrape reachable through module ServiceMonitor/kube-rbac-proxy;
- kube-rbac-proxy service accounts can create `TokenReview` and
  `SubjectAccessReview`, and Prometheus/scraper can `get`
  `deployments/prometheus-metrics` for controller and DMCR;
- `d8_ai_models_collector_up{collector="catalog-state"} == 1`;
- `d8_ai_models_collector_up{collector="runtime-health"} == 1`;
- `d8_ai_models_collector_up{collector="storage-usage"} == 1`;
- `d8_ai_models_collector_scrape_duration_seconds` exists for all three
  collectors and is bounded;
- `d8_ai_models_collector_last_success_timestamp_seconds` is non-zero after a
  successful scrape;
- controller/runtime and DMCR structured error logs use `err`, not `error`;
- runtime Pods/Secrets use e2e labels.

Pass:

- для каждого runtime event есть correlation fields:
  `model`, `namespace`/cluster scope, `uid`, `publicationID`,
  module-owned source policy, `sessionID` или direct-upload state reference.
- missing collector health metric or `collector_up=0` is a stop condition
  unless it is explained by a controlled negative test.

### Slice 3. HF Direct publication matrix

Цель:

- проверить current module-owned direct source policy;
- покрыть `Model` и `ClusterModel`;
- собрать capabilities/status evidence.

Matrix:

- small Safetensors text/chat model;
- small GGUF model;
- small Ollama registry GGUF model;
- larger Ollama registry GGUF model for raw-layer streaming and interruption
  replay;
- small public cluster-scoped Gemma 4-compatible source, если доступен;
- representative models for embeddings, rerank, STT, TTS, CV, multimodal,
  image/video generation and tool-calling metadata. Если publication layout не
  поддержан, ожидается explicit unsupported-layout failure.

Pass:

- supported artifacts reach `Ready`;
- unsupported artifacts fail clearly before heavy byte path;
- `status.resolved.format`, `supportedEndpointTypes`, `supportedFeatures`,
  footprint/provenance match source facts;
- Ollama status exposes source/provider and registry-derived facts without
  claiming a concrete runtime (`vllm`, `ollama`, `llama.cpp`);
- workload delivery works for at least one namespaced `Model` and one
  `ClusterModel`.

### Slice 4. Source mirror evidence

Цель:

- доказать, что source mirror semantics не потеряны после удаления public
  source-fetch mode knob.

Проверки:

- проверить unit/integration evidence для mirror manifest/state, resume-safe
  state и cleanup;
- в live e2e не менять `ModuleConfig`: mirror больше не является
  user-facing cluster toggle.

Pass:

- нет live-runbook шага, который требует public source-fetch mode override;
- mirror evidence остаётся в code-level tests или отдельном internal harness.

### Slice 5. Upload publication and capacity reservation

Цель:

- проверить upload gateway identity, multipart session, reservation and publish;
- проверить понятный отказ при нехватке module-wide capacity.

Scenarios:

- successful small GGUF/Safetensors upload;
- successful Diffusers archive upload if selected test artifact is small enough;
- rejected upload with declared `sizeBytes` greater than available capacity;
- missing/unknown size where capacity limit requires declared size.

Pass:

- successful upload reserves, commits, publishes and releases/settles usage;
- rejected upload returns clear HTTP/status reason, not worker BackOff;
- capacity metrics/status expose limit/used/free/reserved enough for operator;
- no leaked staging multipart/session objects after abort/delete.

### Slice 6. Diffusers and media capabilities

Цель:

- проверить new `Diffusers`, `ImageGeneration`, `VideoGeneration`,
  `VideoInput`, `VideoOutput` support without overclaiming serving readiness.

Scenarios:

- text-to-image Diffusers;
- image-to-image or inpainting Diffusers if small public sample exists;
- text-to-video or image-to-video Diffusers if small public sample exists;
- upload archive variant for at least one Diffusers layout.

Pass:

- valid Diffusers layout publishes as `format=Diffusers`;
- endpoint/features come from `model_index.json`/pipeline metadata;
- invalid mixed layout is rejected explicitly;
- status does not claim MCP; `ToolCalling` remains only model capability.

### Slice 7. Workload delivery matrix

Цель:

- проверить delivery не только для одной модели.

Scenarios:

- single model annotation;
- cluster model annotation;
- multi-model annotations with stable model-name paths;
- transition from one model to another;
- deletion of model while workload exists;
- blocked delivery when neither SharedDirect nor implemented SharedPVC is
  available;
- SharedDirect only when node-cache preflight passes.

SharedDirect preflight:

- `ModuleConfig` has `nodeCache.enabled=true`;
- SDS/local storage CRDs exist and runtime PVCs are bound;
- workload metadata declares only the model annotation; controller injects
  node-cache CSI volumes for requested model names;
- user workload or ai-inference sets nodeSelector/affinity/tolerations for a
  node where node-cache runtime and local storage are actually ready.

Pass:

- workloads start only when selected delivery mode is actually ready;
- SharedDirect CSI mount waits for ready digest and never schedules onto
  unprepared nodes;
- multi-model paths are deterministic and vLLM-compatible enough for future
  `ai-inference` consumption.

### Slice 7A. Manual A30 vLLM SharedDirect drill

Цель:

- отдельно от RayService/GitOps проверить, что обычный `Deployment` с одной
  annotation получает node-cache CSI volume, resolved artifact attributes,
  mounts и env от ai-models;
- отладить CSI/node-cache mount на A30/MIG ноде с локальными дисками;
- доказать, что vLLM видит модель по стабильному пути, а workload не содержит
  ручной `materialize-artifact`, PVC, DMCR credentials или cache mkdir.

Факты текущего preflight на `k8s.apiac.ru`:

- target node: `k8s-w3-gpu.apiac.ru`;
- taint: `dedicated.apiac.ru=w-gpu:NoExecute`;
- group label: `node.deckhouse.io/group=w-gpu-mig`;
- local disks discovered by `sds-node-configurator`:
  `dev-0b257c2c37d39bec8279efd49d0e7d315fddfdfe` and
  `dev-2c4adac3cce4860ce6686d249ef6a96bf268a2ea`, both `150Gi`,
  `consumable=true`;
- ready ClusterModels already exist:
  `a30-user-bge-m3`, `a30-bge-reranker-v2-m3-en-ru`,
  `a30-whisper-medium`;
- current `ModuleConfig ai-models` has `nodeCache` disabled, so SharedDirect
  cannot be tested until node-cache is enabled after rollout.
- 2026-04-30 live attempt with controller image
  `sha256:2182b460a7e5691f6ebb69080ac4056e3f3933ffc85b67095ae1375430454494`
  failed closed before substrate creation: `workloaddelivery.normalizeOptions`
  dropped `DeliveryAuthKey`, so controller crashed with
  `runtime delivery auth key must not be empty when managed node-cache delivery
  is enabled`. `nodeCache` was rolled back to `enabled=false`; repeat this
  slice only after the fixed image is rolled out.

Detailed commands and manifests live in
`plans/active/live-e2e-ha-validation/A30_VLLM_SHARED_DIRECT.ru.md`.
Plain Deployment manifests for embedder/reranker/Whisper live in
`plans/active/live-e2e-ha-validation/a30-shared-direct-workloads.yaml`.

Pass:

- node and selected BlockDevices are labelled with
  `ai.deckhouse.io/model-cache=true`;
- after enabling node-cache, ai-models creates managed
  `LVMVolumeGroupSet`, `LocalStorageClass`, per-node runtime PVC and runtime
  Pod;
- `k8s-w3-gpu.apiac.ru` gets
  `ai.deckhouse.io/node-cache-runtime-ready=true`;
- manual vLLM `Deployment` contains only user-owned intent:
  model annotation, vLLM image/command, GPU scheduling and probes;
- controller mutates PodTemplate into SharedDirect:
  controller-created CSI volume with resolved artifact attributes, stable
  `/data/modelcache` mount, resolved model annotations/env, no materializer
  init container;
- Pod lands on `k8s-w3-gpu.apiac.ru`, consumes `gpu.deckhouse.io/a30-mig-1g6`
  if that resource is available, and serves embeddings through vLLM;
- node-cache runtime logs show digest prefetch/ready and CSI publish, without
  restarting on transient materialization errors.

Stop/fix:

- workload requires manually written ai-models internals;
- mutation leaves the workload blocked instead of SharedDirect while
  node-cache is ready;
- workload schedules onto a node without ready node-cache runtime and then
  hangs/fails on CSI mount instead of being rejected by scheduler/placement
  policy;
- CSI mount hangs without clear condition/log reason;
- vLLM downloads the model from HF instead of reading the matching
  `/data/modelcache/models/<model-name>` path;
- GPU extended resource is absent on the target node after GPU module checks.

### Slice 8. Controlled interruption replay

Цель:

- доказать idempotency/replay under real failures.

Interruptions:

- delete one controller pod during HF Direct publication;
- delete one upload-gateway pod during active upload;
- delete one DMCR pod during direct-upload;
- delete active source-worker pod during large raw-layer upload;
- if node-cache enabled, restart one node-cache runtime pod during workload
  mount/materialization.

Pass:

- public status stays `Publishing`/recoverable for replayable interruption;
- no false terminal `Failed` for `context canceled`/terminated worker;
- direct-upload state secret shows resume/committed progress;
- final publication reaches `Ready`;
- logs clearly show interruption and resume decision.

### Slice 9. Delete, cleanup and DMCR GC

Цель:

- проверить all resource lifecycles after success and after interrupted run.

Проверки:

- finalizers removed only after cleanup decision;
- cleanup state/request secrets have phase queued/armed/done where applicable;
- DMCR maintenance quorum and `deletedRegistryBlobCount` captured;
- source mirror prefixes and upload staging objects are gone;
- no stale leases/runtime pods/secrets/jobs remain.

Pass:

- all e2e resources deleted;
- no request-scoped GC evidence missing;
- if GC delay is active, NOTES records queued timestamp and final done
  timestamp.

### Slice 10. RBAC and API smoke

Цель:

- доказать, что e2e не требует SuperAdmin-only behavior.
- доказать, что module-local RBAC следует Deckhouse/virtualization pattern:
  module adds only explicit access-level deltas, while base persona inheritance
  stays in `user-authz`.
- доказать, что текущие rendered templates соответствуют hardening guardrails,
  а не только live cluster случайно имеет старые/ручные роли.

Проверки:

- user-authz ClusterRoles render for all Deckhouse levels:
  `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`,
  `ClusterAdmin`; `SuperAdmin` stays a Deckhouse/global persona and must not
  need a module-specific fragment;
- compare with virtualization pattern: all module fragments use
  `user-authz.deckhouse.io/access-level`; empty delta roles are acceptable only
  when the module has no extra action for that access level;
- rbacv2 `use` and `manage` roles render with aggregate labels and intended
  scope;
- `kubectl auth can-i --as=<temporary subject>` matrix for:
  `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`,
  `ClusterAdmin`, `SuperAdmin`;
- rbacv2 direct binding matrix for `d8:use:capability:module:ai-models:*`
  and `d8:manage:permission:module:ai-models:*`;
- ClusterModel access uses intended cluster persona/manage path; namespaced
  `use` must not accidentally grant ClusterModel;
- no human role grants Secrets, exec/attach/port-forward, status/finalizers or
  internal runtime objects.
- render validation must pass with RBAC fixture tests:
  `python3 -m unittest tools/helm-tests/validate_renders_test.py`,
  `make helm-template`, `make kubeconform`.

Pass:

- RBAC evidence is explicit in `NOTES.ru.md`;
- every persona has an explicit allow/deny table, not prose;
- service-account permissions remain internal/module-local.

### Slice 11. Final cleanup and decision

Цель:

- вернуть кластер в исходное состояние;
- решить, можно ли считать build готовым к дальнейшим тестам/rollout.

Pass:

- namespace `ai-models-e2e` removed;
- temporary capacity settings restored;
- no active e2e `Model`/`ClusterModel`;
- deployments ready and restartCount stable;
- all findings classified:
  - blocker code defect;
  - architecture follow-up;
  - cluster/environment issue;
  - accepted limitation with evidence.

## 6. Evidence format

All live findings go to `plans/active/live-e2e-ha-validation/NOTES.ru.md` with:

- timestamp;
- scenario id;
- exact resource names;
- status/log/metric snippets;
- command references;
- pass/fail decision;
- cleanup result.

Do not paste secrets or credentials.

## 7. Rollback point

Every e2e object must carry label `ai.deckhouse.io/live-e2e=ha-validation`.

Rollback:

- delete namespace `ai-models-e2e`;
- delete owner-labelled cluster-scoped test objects;
- restore `ModuleConfig` changes made for Mirror/capacity/node-cache checks;
- wait for `d8-ai-models` deployments readiness;
- capture leftover resources if cleanup is not immediate.

## 8. Final validation commands

- `kubectl --context k8s.apiac.ru -n d8-ai-models get deploy,pod,events`
- `kubectl --context k8s.apiac.ru get models.ai.deckhouse.io -A`
- `kubectl --context k8s.apiac.ru get clustermodels.ai.deckhouse.io`
- `kubectl --context k8s.apiac.ru -A get pods -l ai.deckhouse.io/live-e2e=ha-validation`
- `kubectl --context k8s.apiac.ru -A get secrets -l ai.deckhouse.io/live-e2e=ha-validation`
- targeted repo checks if any code/template change is made.
