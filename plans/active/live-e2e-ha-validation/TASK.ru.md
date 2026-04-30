# Live e2e/HA validation after rollout

## 1. Заголовок

Комплексно проверить новую сборку `ai-models` в `k8s.apiac.ru`: публикация,
доставка в workload, storage accounting, capabilities/profile metadata,
placement, node-cache/SharedDirect, delete/GC и восстановление после
прерываний.

## 2. Контекст

Предыдущие live-прогоны уже нашли и закрыли несколько дефектов:

- upload handoff после successful source-worker;
- false terminal `Failed` при удалении active source-worker во время
  direct-upload;
- отсутствие request-scoped DMCR GC для in-flight direct-upload session;
- недостаточную GC UX evidence без `deletedRegistryBlobCount`;
- master/control-plane placement для controller и upload-gateway.

После текущих доработок нужно не просто повторить happy path, а доказать, что
вся Phase 1/2 runtime baseline ведёт себя устойчиво под реальными сбоями и
даёт понятные статусы/логи/метрики.

## 3. Постановка задачи

Подготовить и по отдельной команде выполнить live e2e runbook для
`k8s.apiac.ru`. До команды на старт ничего destructive не запускать.

Проверить:

- rollout и placement module-owned Pods;
- HF `Direct` и HF `Mirror` publication;
- Upload publication через gateway;
- `Model` и `ClusterModel`;
- Safetensors, GGUF, Diffusers image/video layouts;
- metadata/capabilities для chat, embeddings, rerank, STT, TTS, CV,
  multimodal, image/video generation и tool-calling;
- storage capacity reporting/reservation/admission failure;
- workload delivery: multi-model annotations, stable model-name paths and
  SharedDirect только если node-cache реально включён и готов;
- controller/DMCR/upload-gateway/source-worker interruption replay;
- delete/finalizer/cleanup/DMCR GC lifecycle;
- RBAC deny paths и service-account boundaries;
- полноту логов и метрик, достаточную для диагностики без ручного угадывания.

## 4. Scope

- Работать только с `k8s.apiac.ru`.
- Использовать временный namespace `ai-models-e2e` и label
  `ai.deckhouse.io/live-e2e=ha-validation`.
- Перед destructive actions собрать baseline и preflight.
- Для каждой публикации сохранить:
  - YAML `Model` / `ClusterModel`;
  - related runtime Secrets/Pods/Events;
  - controller, upload-gateway, source-worker, node-cache runtime и DMCR logs;
  - relevant metrics snapshots;
  - итоговый cleanup/GC evidence.
- Если найден воспроизводимый дефект текущей реализации, остановить run,
  локализовать root cause и исправить отдельным узким slice.

## 5. Non-goals

- Не лечить unrelated cluster noise вне влияния на `ai-models`.
- Не менять публичный API/CRD во время live run без отдельного API task.
- Не включать node-cache/SharedDirect в live кластере, если SDS/local storage
  preflight не проходит.
- Не менять storage quota policy по namespace: текущий run проверяет только
  module-wide capacity/reservation baseline.
- Не выполнять destructive cleanup за пределами e2e namespace и module-owned
  e2e resources.
- Не считать MCP поддержанным только по `ToolCalling`: MCP остаётся
  runtime/host capability будущего `ai-inference`.

## 6. Затрагиваемые области

- Live cluster:
  - `k8s.apiac.ru`;
  - namespace `d8-ai-models`;
  - namespace `ai-models-e2e`;
  - временные `Model` / `ClusterModel` / workloads;
  - `ModuleConfig` только для заранее описанных reversible checks.
- Repo areas при bugfix:
  - `api/`, `crds/`, `openapi/`;
  - `templates/`;
  - `images/controller/internal/controllers/*`;
  - `images/controller/internal/adapters/*`;
  - `images/controller/internal/dataplane/*`;
  - `images/dmcr/internal/*`;
  - docs/test evidence по необходимости.

## 7. Критерии приёмки

- Rollout: module `Ready`; controller, upload-gateway и DMCR ready; controller
  и upload-gateway не запланированы на master/control-plane.
- HF Direct: минимум одна Safetensors/GGUF модель доходит до `Ready`, workload
  получает модель и не уходит в stale BackOff.
- HF Mirror: mirror state durable, публикация доходит до `Ready`, cleanup
  удаляет mirror prefix.
- Upload: multipart upload, reservation, publish and cleanup проходят; при
  прерывании gateway/controller/worker состояние replayable.
- ClusterModel: cluster-scoped source без namespace-local shortcuts доходит до
  `Ready` и доставляется в workload через `ai.deckhouse.io/clustermodel`.
- Capabilities: для выбранной матрицы моделей status.resolved содержит
  ожидаемые `format`, `supportedEndpointTypes`, `supportedFeatures`,
  footprint и provenance без backend leakage.
- Diffusers: image/video Diffusers layout публикуется как `format=Diffusers`
  или даёт явный supported-layout failure без silent misclassification.
- Storage: metrics/status показывают used/free/limit где limit задан; upload
  reservation не допускает превышение capacity и возвращает понятную ошибку.
- SharedDirect: если node-cache включён, workload стартует только на ready
  node-cache nodes и CSI mount получает готовый digest; если node-cache не
  включён и SharedPVC ещё не реализован, workload получает понятный blocked
  state без скрытого materialize/PVC fallback.
- HA: controlled delete/restart controller, upload-gateway, DMCR pod и active
  source-worker не создаёт ложный terminal failure и не теряет direct-upload
  progress/state.
- Delete/GC: удаление всех test resources завершает finalizers, runtime state,
  cleanup request and DMCR GC with structured deleted-blob evidence.
- RBAC: user-facing roles не получают Secret/exec/attach/port-forward/status/
  finalizers/internal runtime objects; service-account permissions остаются
  module-local.
- После rollback нет test `Model`/`ClusterModel`/workloads, stale leases,
  runtime secrets, upload sessions или restart loops.

## 8. Риски

- Некоторые публичные HF модели могут быть gated, удалены или rate-limited.
- Mirror mode требует временного изменения `ModuleConfig` и rollback.
- Storage capacity test требует аккуратно выбрать лимит, чтобы не заблокировать
  production publication.
- SharedDirect зависит от SDS/local storage readiness и может быть skipped
  только с явным preflight evidence.
- DMCR GC имеет protected window; финальная проверка может занять десятки
  минут.
