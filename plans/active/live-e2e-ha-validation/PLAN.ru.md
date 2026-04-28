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

- `live-e2e-ha-validation` — keep active. Это канонический executable runbook
  для следующего live запуска.
- `observability-signal-hardening` — keep active. Первый code slice уже даёт
  `collector_up` / scrape duration / last-success metrics и normalized `err`
  logs; live e2e должен проверить их после rollout, а alert wiring остаётся
  следующим observability slice.
- `pod-placement-policy` — archived to
  `plans/archive/2026/pod-placement-policy`; placement slice завершён и теперь
  покрывается этим e2e runbook.

## 4. Start gate

Начинать live run только после явной команды пользователя.

Перед стартом зафиксировать:

- `git status -sb`;
- последний commit и образ/rollout, который реально стоит в `k8s.apiac.ru`;
- текущий `ModuleConfig ai-models`;
- текущие nodes/taints/labels;
- текущий список pods/deployments/events в `d8-ai-models`;
- текущие usage/capacity metrics, если доступны;
- текущие active `Model`/`ClusterModel` во всех namespaces.

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
  `sourceFetchMode`, `sessionID` или direct-upload state reference.
- missing collector health metric or `collector_up=0` is a stop condition
  unless it is explained by a controlled negative test.

### Slice 3. HF Direct publication matrix

Цель:

- проверить current default `sourceFetchMode=Direct`;
- покрыть `Model` и `ClusterModel`;
- собрать capabilities/status evidence.

Matrix:

- small Safetensors text/chat model;
- small GGUF model;
- small public cluster-scoped Gemma 4-compatible source, если доступен;
- representative models for embeddings, rerank, STT, TTS, CV, multimodal,
  image/video generation and tool-calling metadata. Если publication layout не
  поддержан, ожидается explicit unsupported-layout failure.

Pass:

- supported artifacts reach `Ready`;
- unsupported artifacts fail clearly before heavy byte path;
- `status.resolved.format`, `supportedEndpointTypes`, `supportedFeatures`,
  footprint/provenance match source facts;
- workload delivery works for at least one namespaced `Model` and one
  `ClusterModel`.

### Slice 4. HF Mirror mode

Цель:

- временно включить `aiModels.artifacts.sourceFetchMode=Mirror`;
- доказать durable mirror snapshot/resume/cleanup.

Проверки:

- source mirror manifest/state objects created under `raw/.mirror`;
- worker logs show mirror download/upload phases and resume-safe state;
- publication reads from mirror object source;
- delete/GC removes backend artifact and source mirror prefix.

Rollback:

- вернуть `sourceFetchMode=Direct` сразу после mirror scenario.

Pass:

- mirror path reaches `Ready` or recoverable failure with durable state;
- no orphan mirror prefix after cleanup/GC.

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
- multi-model annotation with stable env/path aliases;
- transition from one model to another;
- deletion of model while workload exists;
- MaterializeBridge baseline;
- SharedDirect only when node-cache preflight passes.

SharedDirect preflight:

- `ModuleConfig` has `nodeCache.enabled=true`;
- SDS/local storage CRDs exist and runtime PVCs are bound;
- node-cache runtime pods ready on selected nodes;
- ready node labels/taints allow target workload placement.

Pass:

- workloads start only when selected delivery mode is actually ready;
- SharedDirect CSI mount waits for ready digest and never schedules onto
  unprepared nodes;
- multi-model paths are deterministic and vLLM-compatible enough for future
  `ai-inference` consumption.

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
- `sourceFetchMode` and capacity settings restored;
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
