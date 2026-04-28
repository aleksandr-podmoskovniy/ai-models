# Plan: live e2e/HA validation after rollout

## 1. Current phase

Задача относится к Phase 1/2 boundary: publication/runtime baseline уже
выкатился, теперь нужно доказать устойчивость HF/Upload publication paths,
runtime delivery и delete/GC lifecycle перед следующими архитектурными
изменениями.

## 2. Orchestration

Режим: `full`.

Причина: задача затрагивает live runtime, storage/object-store integration,
DMCR GC, controller replay, HA и user-facing API behavior.

Read-only reviews перед активными прерываниями:

- `integration_architect` — проверить live e2e/HA сценарий на рискованные
  действия и missing observability.
- `backend_integrator` — проверить DMCR/upload/GC-specific failure modes.

## 3. Active bundle disposition

- `model-metadata-contract` — keep. Это отдельный executable workstream про
  metadata/profilecalc и будущий `ai-inference`; текущий live e2e не должен
  смешиваться с ним.
- `live-e2e-ha-validation` — keep active. Первая live-проверка нашла
  reproducible upload handoff defect, поэтому controlled interruption slice
  не закрывается на текущем образе: сначала нужна новая сборка с fix, затем
  повторный HF/Upload HA replay. Bundle остаётся executable, а не архивируется,
  пока post-fix replay не выполнен.

## 4. Slices

### Slice 1. Baseline and fixtures

Цель:

- повторно зафиксировать live state после rollout;
- создать временный namespace и test fixtures;
- выбрать маленькие HF/Upload artifacts, которые не перегружают cluster/S3.

Файлы:

- `plans/active/live-e2e-ha-validation/NOTES.ru.md`
- временные Kubernetes объекты в `ai-models-e2e`.

Проверки:

- `kubectl --context k8s.apiac.ru -n d8-ai-models get pods,deploy,events`
- `kubectl --context k8s.apiac.ru get models.ai.deckhouse.io -A`

Артефакт:

- baseline status и список test resources.

### Slice 2. HF publication and workload delivery

Цель:

- создать HF-backed `Model`;
- дождаться publication или корректного recoverable status;
- раскатить тестовый workload с model annotation;
- собрать worker/controller/DMCR логи.

Проверки:

- `kubectl get model -n ai-models-e2e -o yaml`
- logs по source/publish worker pods;
- events по workload.

Артефакт:

- HF path evidence.

### Slice 3. Upload publication

Цель:

- проверить upload session/gateway path;
- загрузить маленький model artifact;
- дождаться publication;
- собрать upload/publish logs.

Проверки:

- upload session/model status;
- gateway/controller/worker logs;
- DMCR events.

Артефакт:

- Upload path evidence.

### Slice 4. Controlled interruptions

Цель:

- во время active publication/reconcile контролируемо перезапустить controller;
- перезапустить один DMCR pod;
- удалить active worker pod/job;
- проверить replay/idempotency и отсутствие terminal false failure.

Проверки:

- rollout status deployments;
- model status after recovery;
- restartCount and events;
- final worker logs.

Артефакт:

- HA/recovery evidence.

### Slice 5. Delete and GC

Цель:

- удалить test models;
- проверить cleanup request lifecycle;
- дождаться GC/done или зафиксировать deliberate delay;
- убедиться, что finalizers/runtime objects ушли.

Проверки:

- `kubectl get secret,job,pod,lease -n d8-ai-models`
- `kubectl get events -n d8-ai-models`
- DMCR GC logs.

Артефакт:

- Delete/GC evidence.

### Slice 6. Fix defects if found

Цель:

- локализовать только воспроизводимые defects;
- внести минимальные патчи;
- прогнать targeted checks.

Файлы:

- будут определены по фактическому дефекту.

Проверки:

- targeted `go test`;
- `make helm-template` / `make verify` если затронуты templates/API.

Артефакт:

- исправление без расползания scope.

### Slice 7. Post-fix HA replay

Цель:

- после выката сборки с upload handoff fix повторить Upload publication;
- во время active upload/publish удалить один controller pod, один DMCR pod и
  active source-worker pod по отдельности;
- доказать replay/idempotency по state secrets, worker logs, DMCR direct-upload
  state и финальному `Ready`;
- повторить delete/GC и проверить structured `deletedRegistryBlobCount`.

Проверки:

- `kubectl get model -n ai-models-e2e -o yaml`
- source-worker/direct-upload/upload-session state secret continuity;
- controller/upload-gateway/DMCR logs;
- `kubectl auth can-i`/static RBAC deny-path smoke для user-facing roles;
- final `d8-ai-models` restartCount and module readiness.

Артефакт:

- post-fix HA/recovery evidence в `NOTES.ru.md`; после этого bundle можно
  архивировать.

### Slice 8. ClusterModel Gemma 4 HA replay

Цель:

- создать cluster-scoped `ClusterModel` для самой маленькой публично доступной
  Gemma 4 модели `google/gemma-4-E2B-it`;
- проверить publication без `authSecretRef` и namespace-local shortcuts;
- во время active publication по одному прервать controller, DMCR и
  source-worker, если соответствующая стадия достаточно долгая;
- раскатить workload с `ai.deckhouse.io/clustermodel` и проверить delivery;
- удалить `ClusterModel` и проверить cleanup/GC evidence.

Проверки:

- `kubectl get clustermodel gemma-4-e2b-it-e2e -o yaml`
- source-worker/direct-upload state secret continuity;
- controller/worker/DMCR logs with enough fields for replay diagnosis;
- controller and DMCR metrics before/after;
- final restartCount, events and absence of leaked test resources.

Артефакт:

- ClusterModel/Gemma 4 HA evidence в `NOTES.ru.md` и список найденных
  архитектурных/эксплуатационных расхождений.

## 5. Rollback point

Все live e2e resources должны иметь label
`ai.deckhouse.io/live-e2e=ha-validation`. Rollback: удалить namespace
`ai-models-e2e`, удалить оставшиеся test objects по label и не трогать
production resources вне `d8-ai-models`.

## 6. Final validation

- `d8-ai-models` deployments ready, restartCount не растёт.
- Нет test `Model`/`ClusterModel`/workload resources.
- Нет stale DMCR maintenance ack leases от удалённых pod identities.
- Если менялся код: targeted tests и `git diff --check`.
