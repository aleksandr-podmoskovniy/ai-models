# Runbook: комплексный live e2e/HA

Этот файл — рабочая шпаргалка для старта по команде. Он не заменяет
`TASK.ru.md` и `PLAN.ru.md`; здесь только порядок выполнения и evidence gates.

## Перед стартом

- Подтвердить context: `k8s.apiac.ru`.
- Подтвердить, что local `main` соответствует build, выкаченному в кластер.
- Проверить, что нет активных пользовательских `Model`/`ClusterModel`, которые
  можно спутать с e2e.
- Создать/проверить namespace `ai-models-e2e`.
- Все test resources маркировать:
  `ai.deckhouse.io/live-e2e=ha-validation`.

## Минимальная модельная матрица

Подбирать публичные маленькие модели в момент запуска, потому что HF state
может меняться.

Обязательные классы:

- `Safetensors` text/chat.
- `GGUF` text/chat.
- Ollama registry GGUF: one small model and one larger model.
- `Diffusers` text-to-image.
- `Diffusers` text-to-video или image-to-video, если есть достаточно маленький
  публичный artifact.
- Embeddings.
- Rerank.
- STT.
- TTS.
- CV image classification/object detection/segmentation.
- Multimodal vision-language.
- Tool-calling capable chat template.

Если класс невозможно опубликовать из-за unsupported artifact layout, pass
только при явном отказе до тяжёлого byte path и корректном status reason.

## Evidence gates

Для каждого scenario фиксировать:

- resource YAML before/after;
- events;
- controller logs;
- source-worker/upload-gateway logs;
- DMCR logs;
- direct-upload state secret phase/stage без секретных значений;
- storage usage/capacity metrics or explicit absence;
- cleanup/GC result.

## Что проверять после rollout

Сначала проверяется не модель, а сама платформа модуля:

- module rollout: `Module/ai-models` Ready, Deployments ready, no growing
  restart count;
- placement: controller и upload-gateway не закреплены на
  master/control-plane; DMCR placement соответствует module policy;
- TLS/ingress: upload-gateway использует отдельные certificate/custom
  certificate templates, без inline secret material в Ingress;
- alerts: нет `Firing` alerts по legacy backend или false `TargetAbsent`;
- ServiceMonitor/service labels: controller и DMCR scrape targets совпадают.
- metrics RBAC: `kube-rbac-proxy` service accounts have authn/authz review
  rights, and `d8-monitoring:prometheus` / `d8-monitoring:scraper` can read
  `deployments/prometheus-metrics` for controller and DMCR.

Observability hardening:

- `d8_ai_models_collector_up{collector="catalog-state"} == 1`;
- `d8_ai_models_collector_up{collector="runtime-health"} == 1`;
- `d8_ai_models_collector_up{collector="storage-usage"} == 1`;
- `d8_ai_models_collector_scrape_duration_seconds` есть для всех collectors;
- `d8_ai_models_collector_last_success_timestamp_seconds` не равен `0` после
  успешного scrape;
- в controller/runtime/DMCR structured logs error attribute называется `err`;
- при controlled negative case collector health должен падать в `0`, но
  `/metrics` endpoint должен оставаться живым.

Storage/capacity:

- module-wide storage limit/used/reserved/free видны в metrics/status там, где
  limit задан;
- HF Direct reserve считается по сумме выбранных remote files до DMCR
  direct-upload;
- HF Mirror reserve считается как две module-owned копии: raw mirror +
  canonical DMCR artifact; отказ по capacity должен происходить до mirror
  transfer;
- upload с достаточным размером резервирует место до publish;
- upload сверх доступного места получает понятный отказ без worker BackOff;
- после delete/cleanup reservation/used корректно сходятся.

Publication/runtime:

- HF Direct: small Safetensors/GGUF `Model`;
- HF Direct: small `ClusterModel`;
- HF Mirror: durable mirror state, resume-safe publish, cleanup mirror prefix;
- Ollama Direct: small GGUF `Model` from `https://ollama.com/library/...`,
  proving registry manifest/config/model-layer path, not HTML scraping;
- Ollama Direct: larger GGUF `ClusterModel` to exercise raw-layer streaming,
  storage reservation and interruption replay;
- Ollama Mirror, if internal mirror mode is enabled in the tested build:
  durable source mirror state, resume-safe transfer and cleanup mirror prefix;
- Upload: multipart upload через gateway, replay после restart gateway или
  controller;
- Diffusers: image generation и video generation layout либо публикуется как
  `format=Diffusers`, либо получает explicit unsupported-layout reason до
  тяжёлого byte path;
- metadata/capabilities: chat, embeddings, rerank, STT, TTS, CV, multimodal,
  image/video generation, tool-calling без overclaim MCP.
- ai-inference handoff evidence:
  `sourceCapabilities.provider`, `format`, `family`, `parameterCount`,
  `quantization`, `contextWindowTokens`, `supportedEndpointTypes` and
  `supportedFeatures`; CR status must not contain concrete runtime selection
  such as `vllm`, `ollama` or `compatibleRuntimes`.

Workload delivery:

- single-model annotation;
- `ClusterModel` annotation;
- multi-model aliases and stable paths;
- model switch on existing workload;
- delete model while workload exists;
- MaterializeBridge baseline;
- SharedDirect только если node-cache/SDS/local storage preflight полностью
  готов;
- SharedDirect получает scheduling gate, если сумма `sizeBytes` всех aliases
  больше per-node shared cache capacity или size неизвестен.

Manual A30/vLLM SharedDirect drill:

- до подтверждения нового rollout не включать `nodeCache` и не применять
  workload;
- target cluster: `k8s.apiac.ru`;
- target node: `k8s-w3-gpu.apiac.ru`;
- target model: `ClusterModel/a30-user-bge-m3`;
- target image: `rayproject/ray-llm:2.54.0-py311-cu128`;
- workload manifest должен содержать только annotation
  `ai.deckhouse.io/model-refs: model=ClusterModel/a30-user-bge-m3`, vLLM
  command/resources/probes and GPU scheduling;
- запрещены ручные `materialize-artifact`, PVC `model-cache-pvc`, DMCR Secret
  env and cache mkdir;
- detailed script:
  `plans/active/live-e2e-ha-validation/A30_VLLM_SHARED_DIRECT.ru.md`.

HA/interruption:

- delete controller pod during publication;
- delete upload-gateway pod during upload;
- delete DMCR pod during direct-upload;
- delete active source-worker during raw-layer upload;
- delete active source-worker during larger Ollama raw-layer upload;
- restart node-cache runtime during mount/materialization when SharedDirect is
  enabled.

Delete/GC:

- finalizers released only after cleanup evidence;
- cleanup-state/upload/session/source-worker secrets disappear;
- DMCR GC request goes queued -> armed -> done;
- `deletedRegistryBlobCount` or equivalent structured evidence is captured;
- no e2e-labeled pods/secrets/jobs/leases remain.

## RBAC matrix gate

RBAC проверяется до destructive HA шагов и повторяется после rollout, если
шаблоны менялись. Сравнение с virtualization:

- module-specific `user-authz` правила должны быть только delta-фрагментами с
  `user-authz.deckhouse.io/access-level`;
- inheritance между `User` -> `PrivilegedUser` -> `Editor` -> `Admin` и
  cluster-level ролями обеспечивает Deckhouse `user-authz`, а не копирование
  одних и тех же правил в каждом module fragment;
- пустой fragment допустим только если для этого уровня нет нового
  module-owned действия.

Проверить first-version `user-authz`:

- `User`: allow `get/list/watch` `models` and `clustermodels`; deny
  `create/update/patch/delete`, `status`, `finalizers`, `secrets`, `pods/log`,
  `pods/exec`, `pods/attach`, `pods/portforward`, internal runtime objects.
- `PrivilegedUser`: same module delta as `User`; no extra module-local Secret
  or pod subresource access.
- `Editor`: allow write for namespaced `models`; deny write for
  `clustermodels`, deny sensitive paths.
- `Admin`: same module delta as `Editor`; no service-object delete surface is
  exposed by `ai-models`.
- `ClusterEditor`: allow write for `clustermodels`; also verify effective
  persona keeps namespaced model write through Deckhouse inheritance.
- `ClusterAdmin`: same module delta as `ClusterEditor`; no extra
  module-owned service-object surface is exposed.
- `SuperAdmin`: no module-specific fragment; verify effective global persona
  can operate while namespaceSelector/limitNamespaces semantics are not
  bypassed by module resources.

Проверить `rbacv2`:

- `d8:use:capability:module:ai-models:view`: allow read `models`; deny
  `clustermodels` and all sensitive paths.
- `d8:use:capability:module:ai-models:edit`: allow write `models`; deny
  `clustermodels`, `moduleconfigs`, sensitive paths.
- `d8:manage:permission:module:ai-models:view`: allow read
  `models`, `clustermodels` and `ModuleConfig/ai-models`; deny sensitive paths.
- `d8:manage:permission:module:ai-models:edit`: allow write
  `models`, `clustermodels` and `ModuleConfig/ai-models`; deny status,
  finalizers, Secrets and internal runtime objects.

Evidence commands:

- static/render:
  `python3 -m unittest tools/helm-tests/validate_renders_test.py`,
  `make helm-template`, `make kubeconform`;
- render/static: `make helm-template`;
- live matrix: `kubectl auth can-i --as=<e2e-subject> ...`;
- service-account boundary:
  `kubectl auth can-i --as=system:serviceaccount:d8-ai-models:ai-models-controller ...`
  and `--as=system:serviceaccount:d8-ai-models:dmcr ...`;
- record every allow/deny row in `NOTES.ru.md`.

## Stop conditions

Остановить прогон и перейти к fix-slice, если:

- public status получает terminal `Failed` на replayable interruption;
- upload reservation допускает превышение capacity;
- cleanup удаляет runtime state без request-scoped backend cleanup evidence;
- controller/upload-gateway после rollout остаются на master/control-plane;
- SharedDirect workload попадает на ноду без ready node-cache runtime;
- any human-facing role allows Secret, pod log/exec/attach/port-forward,
  `*/status`, `*/finalizers` or internal runtime objects unexpectedly;
- namespaced `use` grants `ClusterModel`, or cluster personas cannot operate
  `ClusterModel` through the intended path;
- logs не позволяют связать model -> worker -> direct-upload -> DMCR artifact.

## Cleanup

После каждого scenario удалять test resources, если следующий scenario не
использует их как dependency. После всего run:

- удалить namespace `ai-models-e2e`;
- удалить cluster-scoped test `ClusterModel`;
- вернуть временные `ModuleConfig` изменения;
- дождаться DMCR GC done или зафиксировать deliberate delay;
- проверить отсутствие e2e-labeled pods/secrets/jobs/leases.
