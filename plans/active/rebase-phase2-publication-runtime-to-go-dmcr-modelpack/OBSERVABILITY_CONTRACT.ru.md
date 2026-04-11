# Observability Contract For `ai-models`

## Зачем этот note

Нужен не generic wishlist по метрикам, а честный module-owned contract,
который позволяет:

- встроить `ai-models` в Deckhouse platform monitoring на том же уровне
  зрелости, что и `virtualization`;
- не плодить fake metrics, которые не дают operator action;
- не путать runtime-local debug signal с platform-level monitoring contract.

Этот note основан на прямом разборе live patterns из sibling
`virtualization` module:

- protected scrape shell:
  - `../virtualization/templates/virtualization-controller/service-monitor.yaml`
  - `../virtualization/templates/virtualization-controller/service-metrics.yaml`
  - `../virtualization/templates/rbac-to-us.yaml`
- alert rules:
  - `../virtualization/monitoring/prometheus-rules/*.yaml`
- dashboard queries:
  - `../virtualization/monitoring/grafana-dashboards/*`
- logging and controller integration:
  - `../virtualization/images/virtualization-artifact/pkg/logger/*`
  - `../virtualization/images/virtualization-artifact/pkg/eventrecord/eventrecorderlogger.go`
- object-state metrics:
  - `../virtualization/images/virtualization-artifact/pkg/monitoring/metrics/*`

## Что реально делает `virtualization`

### 1. Scrape shell жёстко отделён от product metrics

`virtualization` сначала решает не "что мерить", а "как безопасно отдавать".
Pattern такой:

- metrics bind only on localhost inside Pod;
- наружу `/metrics` отдаёт только `kube-rbac-proxy`;
- `ServiceMonitor` идёт по HTTPS;
- для scrape используется platform machine identity;
- target discovery и RBAC оформлены как module-owned deployment shell.

Это правильный baseline. Это не optional polish, а обязательная часть module
integration.

### 2. Alerts в основном symptom-based, а не debug-counter-based

Повторяющийся паттерн в `virtualization`:

- `TargetDown`
- `TargetAbsent`
- `PodIsNotReady`
- `PodIsNotRunning`
- storage capacity risk для module-owned PVC

Отдельные business alerts появляются только там, где есть прямой operator
action и понятная remediation:

- outdated VM firmware;
- node shutdown blocked by running VMs.

То есть `virtualization` не строит alerting на noisy internal counters типа:

- reconcile errors total;
- handler retries total;
- upload/download requests total;
- transient per-step failures.

Это важно перенести один-в-один.

### 3. Product metrics строятся от публичной object truth

Самая важная идея `virtualization`:
dashboards и часть product logic строятся не от логов и не от ephemeral pod
metrics, а от explicit object-state metrics:

- `<object>_status_phase`
- `<object>_info`
- отдельные actionable booleans
- sizes / capacities / timestamps только там, где они реально нужны

То есть платформа видит:

- сколько объектов в каком состоянии;
- какие из них готовы;
- какие занимают сколько ресурса;
- какие дополнительные свойства влияют на операционное решение.

Это гораздо полезнее, чем histogram-ы вокруг каждой функции контроллера.

### 4. Logging в `virtualization` унифицирован как infrastructure primitive

У `virtualization` logging не размазан по стилям:

- один root logger;
- он bridged в `slog`, `controller-runtime` и `klog`;
- reconcile logger получает `name` / `namespace`;
- code-level handlers добавляют стабильные attrs:
  - `controller`
  - `handler`
  - `step`
  - `collector`
- events можно писать одновременно в K8s Events и в log stream.

То есть log line уже сама по себе содержит контекст, а не требует повторного
grep по косвенным кускам сообщения.

### 5. Runtime-local progress metrics не подменяют platform contract

В `virtualization` есть import/progress metrics и termination-message reports,
но они используются прежде всего:

- чтобы обновить `status.progress`;
- чтобы разобрать final result;
- чтобы дать runtime-local introspection.

Они не становятся центральным platform contract для alerting и dashboards.

Это для `ai-models` тоже критично:
не надо путать upload/publish byte-path debug metrics с module monitoring
contract.

## Что уже есть в `ai-models`

### Уже сделано правильно

- protected metrics shell already exists:
  - controller metrics bind only on `127.0.0.1`;
  - controller `/metrics` exposed through `kube-rbac-proxy`;
  - DMCR does the same;
  - backend already has its own `ServiceMonitor` with machine-only auth.
- root controller logger already bridges:
  - `slog`
  - `controller-runtime/pkg/log`
  - `k8s.io/klog/v2`
- append-only audit seam already exists via controller-owned `Kubernetes Events`.

### Чего пока нет

- нет module-owned `PrometheusRule` для controller / DMCR / backend health;
- нет module-owned product metrics for `Model` / `ClusterModel`;
- нет ai-models dashboards;
- runtime binaries `publish-worker` / `upload-gateway` / `artifact-cleanup`
  не используют тот же structured logging shell, что controller;
- audit events не mirrored в logs как один unified operator trail;
- нет explicit storage-risk alert для DMCR PVC mode;
- нет честно зафиксированного списка "эти метрики нужны", "эти не нужны".

## Философия метрик для `ai-models`

### Что считать source of truth

Source of truth для platform observability должны быть:

- `Model.status`
- `ClusterModel.status`
- kube-state metrics для module-owned Pods / PVC / workload health

Не должны быть source of truth:

- per-request upload gateway internals;
- per-step publish worker debug counters;
- log parsing;
- S3 multipart implementation details;
- hidden cleanup handles / session secrets.

### Что нам действительно нужно

Обязательны только метрики, на которые есть один из трёх ответов:

1. по ним оператор понимает, жив модуль или нет;
2. по ним можно построить platform dashboard по public model catalog;
3. по ним можно завести alert с прямым operator action.

Если метрика не даёт одного из этих трёх результатов, она не должна входить
в minimal contract.

### Что нам не нужно как first-class contract

Не нужны в первой обязательной волне:

- reconcile duration histograms по каждому handler;
- counters всех ошибок по всем code paths;
- per-session/per-model request counters с высококардинальными labels;
- byte progress metrics как cluster-wide scrape contract;
- generic `condition{reason=...}` с большой label churn;
- metrics c `artifactURI`, raw object key, upload session ID, repo revision,
  filename или HTTP URL как labels.

## Обязательный logging contract для `ai-models`

### 1. Один logging shell для всех Go binaries

Обязательны общие helpers для:

- `ai-models-controller`
- `ai-models-artifact-runtime publish-worker`
- `ai-models-artifact-runtime upload-gateway`
- `ai-models-artifact-runtime artifact-cleanup`

Минимум:

- один common logger factory;
- один format switch (`text` / `json`);
- no silent fallback to ad-hoc stderr printing;
- controller keeps bridge into `controller-runtime` and `klog`;
- runtime commands use the same field naming discipline.

### 2. Обязательные structured attrs

Минимальный стабильный словарь attrs:

- `component`
- `controller`
- `kind`
- `namespace`
- `name`
- `sourceType`
- `runtimeKind`
- `phase`
- `reason`

Дополнительно, где применимо:

- `sessionID`
- `workerPod`
- `jobName`
- `artifactKind`
- `artifactDigest`
- `rawBucket`
- `rawKeyPrefix`

Нельзя логировать:

- upload token;
- presigned URLs;
- auth headers;
- secret contents;
- full CA bundles;
- full raw object keys, если они несут user-derived sensitive path material.

### 3. Логировать только lifecycle edges и важные decisions

Нужно логировать:

- object accepted;
- upload session issued;
- probe accepted / rejected;
- multipart init / complete / abort;
- remote ingest started;
- raw staged;
- publication started;
- publication succeeded / failed;
- publish worker skipped because concurrency cap reached;
- cleanup job requested / completed / failed;
- DMCR GC requested / completed / failed.

Не нужно логировать:

- каждый poll loop;
- каждый reconcile без state change;
- каждый upload part;
- большие object dumps.

### 4. Events и logs должны идти одним semantic trail

Правильный pattern из `virtualization`:

- user-facing K8s Event остаётся;
- тот же semantic edge пишется и в log stream теми же reason/message class.

Для `ai-models` это особенно важно на переходах:

- `UploadSessionIssued`
- `RemoteIngestStarted`
- `RawStaged`
- `PublicationSucceeded`
- `PublicationFailed`

## Обязательный metrics contract для `ai-models`

Namespace метрик должен быть module-owned и DKP-consistent:

- `d8_ai_models_*`

### A. Component health metrics

Здесь custom metrics не нужны.
Используем:

- `up`
- `kube_pod_status_ready`
- `kube_pod_status_phase`
- `kubelet_volume_stats_*` для module-owned PVC

Сюда входят компоненты:

- controller
- backend
- DMCR

### B. Product state metrics для public catalog objects

Это обязательная часть first real observability baseline.

Нужны:

- `d8_ai_models_model_status_phase{name,namespace,uid,phase,source_type}`
- `d8_ai_models_clustermodel_status_phase{name,uid,phase,source_type}`

Эти метрики должны давать один-hot gauge по текущей фазе так же, как это
сделано в `virtualization`.

Фазы должны быть взяты из live public status, а не из внутреннего runtime
state. Для текущего API это значит:

- `Pending`
- `WaitForUpload`
- `Publishing`
- `Ready`
- `Failed`
- `Deleting`

### C. Минимальные actionable booleans

Отдельными gauge, а не generic mega-condition metric:

- `d8_ai_models_model_ready{name,namespace,uid}`
- `d8_ai_models_clustermodel_ready{name,uid}`
- `d8_ai_models_model_validated{name,namespace,uid}`
- `d8_ai_models_clustermodel_validated{name,uid}`

Почему это нужно:

- `Ready` отвечает на главный platform question "можно ли потреблять модель";
- `Validated` отделяет publication success от policy mismatch.

### D. Минимальная info metric

Нужны:

- `d8_ai_models_model_info{name,namespace,uid,resolved_source_type,format,task,framework,artifact_kind}`
- `d8_ai_models_clustermodel_info{name,uid,resolved_source_type,format,task,framework,artifact_kind}`

Это даёт operator-facing срезы по:

- source type;
- input / resolved format;
- task / framework;
- artifact backend kind.

Не надо вешать сюда:

- `artifactURI`
- `digest`
- `revision`
- `repoID`
- `license`
- `family`
- runtime compatibility matrix

Их можно добавить позже только если на них появится реальный dashboard use
case. В first wave это лишняя label density.

### E. Artifact size metric

Нужны:

- `d8_ai_models_model_artifact_size_bytes{name,namespace,uid}`
- `d8_ai_models_clustermodel_artifact_size_bytes{name,uid}`

Это минимально полезно для:

- capacity views;
- top-N heavy models;
- сопоставления роста published catalog с storage pressure.

### F. Что можно отложить

Можно отложить до отдельного slice:

- upload session expiry timestamp metrics;
- publication operation timestamp metrics;
- upload gateway request counters;
- publish worker duration histograms;
- raw object count / raw size metrics;
- cleanup lifecycle metrics.

Они не запрещены, но не должны блокировать first module-grade monitoring
baseline.

## Обязательный alerting contract для `ai-models`

### 1. Health alerts

Обязательны:

- `D8AiModelsControllerTargetDown`
- `D8AiModelsControllerTargetAbsent`
- `D8AiModelsControllerPodIsNotReady`
- `D8AiModelsControllerPodIsNotRunning`
- `D8AiModelsDMCRPodIsNotReady`
- `D8AiModelsDMCRPodIsNotRunning`
- `D8AiModelsBackendPodIsNotReady`
- `D8AiModelsBackendPodIsNotRunning`

Это прямая калька с правильного `virtualization` pattern.

### 2. Storage risk alerts

Обязательны:

- `D8AiModelsDMCRInsufficientCapacityRisk`

Условие:

- только для PVC-backed publication storage;
- alert на "мало свободного места" по тому же operational смыслу, что у
  `DVCR`.

Опционально, если shared publication work volume переведён в PVC mode:

- `D8AiModelsPublicationWorkPVCInsufficientCapacityRisk`

### 3. Что не надо alert-ить

Не надо делать first-wave alerts на:

- любой `Model` в `Failed`;
- любой `ClusterModel` в `Failed`;
- upload session expiry;
- отдельные preflight rejects;
- cleanup pending;
- worker retries;
- upload gateway 4xx;
- individual user mistakes in source URL / format / quota.

Это либо noisy by design, либо требует product policy, которой сейчас ещё нет.

## Dashboards: минимально полезный набор

### Обязательный overview dashboard

Нужен один module overview dashboard с:

- count `Model` by phase;
- count `ClusterModel` by phase;
- ready vs failed vs wait-for-upload split;
- split by source type;
- split by resolved format;
- top artifact sizes;
- controller / backend / DMCR component health;
- DMCR PVC free space, если storage mode = PVC.

### Что пока не нужно

Не нужны сразу:

- per-upload traffic dashboards;
- per-worker CPU flame-like views;
- per-stage latency histograms;
- dashboard по raw S3 object churn.

Это можно сделать позже только если появится operator use case.

## Практический gap list для live `ai-models`

### Обязательно сделать

1. Добавить module-owned collectors для `Model` / `ClusterModel` state metrics
   по pattern `virtualization` cache-backed collectors.
2. Добавить `PrometheusRule` для controller / backend / DMCR health.
3. Добавить `PrometheusRule` для DMCR PVC capacity risk.
4. Завести один overview dashboard.
5. Протянуть unified structured logger в runtime binaries.
6. Сделать audit-event-to-log mirroring.

### Не надо делать в этой же волне

1. Не вводить большой generic condition metric.
2. Не плодить histograms вокруг каждого adapter step.
3. Не тащить S3 multipart internals в Prometheus labels.
4. Не alert-ить на user-generated model failures.
5. Не пытаться сразу построить SLO на publish latency, пока byte-path и
   native encoder ещё не стабилизированы.

## Рекомендуемый порядок реализации

### Slice O1. Logging shell first

- shared logger helpers for all runtime commands;
- structured attrs on lifecycle edges;
- audit events mirrored to logs.

### Slice O2. Product state metrics

- `Model` / `ClusterModel` collectors;
- state/ready/validated/info/size metrics;
- no extra debug counters.

### Slice O3. Alerts and dashboard

- health alerts;
- DMCR capacity alert;
- one overview dashboard.

### Slice O4. Optional later

- operation timestamps if stuck-state alerting becomes necessary;
- upload gateway request metrics if there is a proven operator use case;
- worker duration histograms only after publish stages become stable enough.

## Bottom line

`virtualization` teaches three things that are directly applicable here:

1. Сначала secure scrape shell.
2. Затем object-state metrics from public truth.
3. Alerts only on health and actionable risk, not on internal chatter.

Для `ai-models` это означает:

- module integration shell уже в целом правильный;
- product observability contract ещё почти не реализован;
- следующая правильная работа не "навалить побольше метрик", а
  "добавить маленький жёсткий набор state metrics, health alerts и
  unified lifecycle logging".
## Contract override (2026-04-11)

- DMCR is no longer user-configurable as `PersistentVolumeClaim`; the live
  module contract supports only the shared S3-compatible byte backend from
  `aiModels.artifacts`.
- Because of that, module-owned baseline alerts and overview dashboards must
  not promise PVC-capacity monitoring for DMCR.
