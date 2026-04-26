# План

## Orchestration

Режим этого slice: `solo`.

Причина: текущая задача проектирует тестовый план и не меняет runtime code.
При переходе к реализации/исполнению destructive chaos нужен отдельный
execution slice. Для него режим должен быть минимум `light`, а при изменении
controller/DMCR/runtime поведения — `full` с `integration_architect` и финальным
`reviewer`.

## Read-only выводы по соседним паттернам

### Что делает `virtualization` / `DVCR`

- Runtime pods создаются детерминированно через controller helpers, имеют
  `RestartPolicyOnFailure`, owner references, finalizers, restricted security
  context и повторно используются после transient падений.
- Import/data path использует retry/backoff с классификацией transient ошибок,
  а не превращает каждую сетевую/registry ошибку в terminal failure.
- Для data transfer есть progress reader и Prometheus metrics: видны bytes,
  speed, phase и длительность.
- DVCR GC — это явная lifecycle state machine поверх одного Secret и
  Deployment condition/result:
  `waiting provisioning -> enable GC mode/read-only -> wait done/timeout ->
  persist result -> cleanup marker`.
- DVCR GC не удаляет marker до того, как результат сохранён в наблюдаемое
  место. Это важный UX/HA паттерн: после сбоя controller не теряет сведения,
  чем закончился GC.
- Generic GC controllers в virtualization используют cron source, cache sync
  timeout, panic recovery и idempotent delete candidates.

### Что уже есть в `ai-models`

- Publish worker pod имеет детерминированное имя и state Secret.
- При failed worker pod и живой direct-upload state сервис умеет пересоздать
  worker.
- Direct-upload publisher хранит committed-layer journal и умеет продолжать
  upload после уже завершённых частей.
- Direct-upload API retry уже покрывает `503` с bounded exponential backoff.
- `dmcr` GC имеет executor Lease, maintenance gate, ACK quorum и read-only
  окно для `dmcr` / direct-upload sidecar.
- GC request Secrets поддерживают queued/armed lifecycle и targeted cleanup для
  direct-upload prefixes/multipart uploads.

### Что нужно доказать, а не предполагать

- Restart worker во время большого layer upload не должен начинать всё с нуля,
  терять committed journal или оставлять orphan multipart upload.
- Restart controller во время publish/delete не должен ломать state handoff.
- Restart `dmcr` во время upload/complete/GC не должен переводить publication в
  terminal `Failed`, если ошибка transient.
- Read-only GC window не должно превращать публикацию в terminal failure.
- Workload delivery должен сходиться после потери pod-local materialization.
- Delete во время partial publish должен чистить partial registry/direct-upload
  state.
- GC result должен быть наблюдаемым, а не только в короткоживущих логах sidecar.

## Общие guardrails перед запуском

1. Использовать отдельный namespace и timestamped имена:
   `ai-models-chaos-<YYYYMMDDHHMMSS>`.
2. Использовать маленькую, но не микроскопическую модель:
   `google/gemma-4-E2B-it` достаточно велика, чтобы поймать upload/materialize
   recovery, но не создаёт ненужный риск как крупные 30B/70B модели.
3. Зафиксировать baseline:
   `kubectl get pods -n d8-ai-models`, restart counters, module status, events,
   `dmcr` pods, controller pods, GC sidecar logs, active `dmcr-gc-*`.
4. Вести watch evidence в файл:
   `Model`, workload Pod, publish worker Pod, controller logs, `dmcr` logs,
   GC logs, Events, state Secret names, registry cleanup report.
5. Не удалять Secrets с auth/token data вручную, кроме тестовых state Secrets в
   специально отмеченном negative scenario.
6. Не ребутать реальные worker nodes до прохождения pod-delete и drain сценариев.
7. Остановить тест при росте ошибок RGW/Ceph/DMCR вне тестового namespace.

## Инварианты успешного восстановления

- `Model` после transient отказов приходит в `Ready=True` с тем же digest.
- Для transient `503` / read-only / pod restart нет terminal `Failed`, если
  retry budget не исчерпан.
- Progress/state монотонен: completed layer не грузится заново как новый layer,
  committed journal сохраняется.
- Workload после convergence имеет правильные
  `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST`,
  `AI_MODELS_MODEL_FAMILY` и реально видит модель.
- Нет бесконечных stale Pods, stale finalizers, stale projected Secrets,
  dangling `dmcr-gc-*` и orphan publish/cleanup state Secrets для тестового UID.
- `dmcr` GC после delete освобождает registry/object-storage state для
  тестового artifact или явно сообщает, почему нечего удалять.
- Injected restarts видны и объяснимы; дополнительных restart loops нет.
- Логи содержат operation UID, model namespace/name, stage, digest/layer,
  attempt, retry wait, duration и классификацию ошибки.

## Failure matrix

| ID | Сценарий | Инъекция | Ожидаемое поведение | Evidence |
| --- | --- | --- | --- | --- |
| C0 | Baseline control | Publish + workload + delete без отказов | Повторить happy path и зафиксировать baseline timings | Model status, worker logs, workload logs, GC logs |
| C1 | Publish worker restart during upload | Удалить publish worker Pod после начала большого layer upload | Controller пересоздаёт worker, direct-upload state resume, Model Ready | Новый worker UID, state Secret, committed layers, no orphan multipart |
| C2 | Publish worker node disruption | Сначала delete Pod, затем controlled drain выбранной node | Worker появляется на допустимой node и продолжает upload | Events scheduling, worker logs, no terminal Failed |
| C3 | Controller restart during publish | Удалить leader/controller Pod во время upload | Новый leader продолжает reconcile, worker/state не теряются | Controller logs before/after, Model conditions stable |
| C4 | Controller restart during delete | Начать delete Model и удалить controller Pod до finalizer release | Finalizer/GC state продолжаются после рестарта | Finalizers, cleanup state, GC request lifecycle |
| C5 | DMCR pod restart during direct-upload API | Удалить один `dmcr` Pod во время init/complete/list parts | Worker retries transient API errors and succeeds | Worker retry logs, DMCR pod UID change, Model Ready |
| C6 | DMCR restart during part upload | Удалить `dmcr` Pod во время active upload | Upload не становится terminal Failed; resume/retry path срабатывает | Direct-upload logs, worker exit code, state Secret |
| C7 | GC read-only during publish | Запустить GC marker во время publication | `503 UNAVAILABLE` классифицируется как transient/requeue, не terminal Failed | Worker/controller logs, Model condition, GC gate logs |
| C8 | Workload materializer loss | Удалить workload Pod во время materialize | ReplicaSet создаёт новый Pod, materialization повторяется и workload Running | Init/container logs, Events, final model files |
| C9 | Controller restart during workload patch | Удалить controller Pod во время delivery mutation | Workload сходится к одному корректному template, без oscillation | Deployment generations, Pod template diff, Events |
| C10 | Delete during partial publish | Удалить Model во время active upload | Worker/cleanup path завершается безопасно, GC чистит partial state | Finalizer release, direct-upload target cleanup, no orphan Secrets |
| C11 | DMCR GC pod restart / lease transfer | Удалить текущий GC executor Pod во время GC window | Lease переходит, gate освобождается, request не теряется | Lease holder, gate file logs, GC request completion |
| C12 | DMCR ACK quorum timeout | Вызвать GC при недоступном одном replica/sidecar | GC не ломает registry, остаётся queued/retry, явно логирует quorum miss | GC logs, no stuck read-only gate |
| C13 | Node-cache runtime restart | Только если `nodeCache.enabled=true`: удалить node-cache runtime Pod | CSI mount ждёт ready digest, workload eventually Running | Node label, CSI logs, workload Events |
| C14 | Node-cache unsuitable node | Только если `nodeCache.enabled=true`: workload на node без ready cache/local storage | Scheduler/guardrail не допускает silent hang на неподходящей node | Node labels, scheduling Events, CSI errors |
| C15 | Corrupted test state Secret | Намеренно испортить только тестовый direct-upload state Secret | Ошибка explicit и безопасная, без panic/restart loop | Controller/worker logs, terminal condition reason |

## Slice execution order

### Slice 1. Harness and passive observation

- Написать shell harness, который создаёт test namespace, test Model,
  test workload и параллельно собирает events/logs/status.
- Команды должны быть idempotent и иметь `cleanup` mode.
- Проверка: C0 без отказов.

### Slice 2. Worker resume

- Прогнать C1 и C2.
- Основной вопрос: direct-upload resume реально работает при pod death, а не
  только при unit-test replay.
- Stop condition: Model ушёл в terminal `Failed` от transient pod/node loss.

### Slice 3. Controller HA recovery

- Прогнать C3, C4 и C9.
- Проверить leader switch, повторный reconcile, отсутствие double-create и
  template oscillation.

### Slice 4. DMCR transient and read-only recovery

- Прогнать C5, C6, C7.
- Основной ожидаемый дефект: не все DMCR `503` / connection reset / read-only
  ошибки сейчас могут классифицироваться как retryable на всех этапах upload.
- Если publication становится terminal `Failed`, фиксировать exact call path и
  переводить в implementation slice: retry/requeue transient DMCR errors.

### Slice 5. Workload runtime recovery

- Прогнать C8.
- Если `nodeCache.enabled=true`, добавить C13/C14.
- Для текущего `MaterializeBridge` отдельно проверить, что потеря `emptyDir`
  не оставляет workload в вечном init loop без понятной причины.

### Slice 6. Delete and GC resilience

- Прогнать C10, C11, C12.
- Проверить, что GC request имеет понятный lifecycle, а результат не теряется
  вместе с короткоживущими логами.
- Если result остаётся только в logs, это architectural gap: нужен DVCR-style
  persisted GC result / condition.

### Slice 7. Negative state corruption

- Прогнать C15 только на отдельном тестовом объекте.
- Цель не recovery любой ценой, а безопасный explicit fail без panic loop и без
  удаления чужих данных.

## Команды evidence

Базовый набор:

```bash
kubectl get modules ai-models -o yaml
kubectl -n d8-ai-models get pods -o wide
kubectl -n d8-ai-models get deploy ai-models-controller ai-models-dmcr -o yaml
kubectl -n d8-ai-models logs deploy/ai-models-controller --since=30m --all-containers
kubectl -n d8-ai-models logs deploy/ai-models-dmcr -c dmcr --since=30m
kubectl -n d8-ai-models logs deploy/ai-models-dmcr -c dmcr-direct-upload --since=30m
kubectl -n d8-ai-models logs deploy/ai-models-dmcr -c dmcr-garbage-collection --since=30m
kubectl get events -A --sort-by=.lastTimestamp
kubectl get models.ai.deckhouse.io -A -o yaml
kubectl -n d8-ai-models get secret -l ai.deckhouse.io/model-uid=<uid> -o name
kubectl -n d8-ai-models get secret -l ai.deckhouse.io/dmcr-gc-request=true -o yaml
```

Для workload:

```bash
kubectl -n <test-ns> get deploy,pod -o wide
kubectl -n <test-ns> describe pod <pod>
kubectl -n <test-ns> logs <pod> --all-containers
kubectl -n <test-ns> exec <pod> -- env | grep '^AI_MODELS_'
kubectl -n <test-ns> exec <pod> -- sh -c 'find "$AI_MODELS_MODEL_PATH" -maxdepth 2 -type f | head'
```

Для injected restarts:

```bash
kubectl -n d8-ai-models get pod -w
kubectl -n d8-ai-models delete pod <controller-pod>
kubectl -n d8-ai-models delete pod <dmcr-pod>
kubectl -n d8-ai-models delete pod <publish-worker-pod>
```

Node drain/reboot команды не включаются в автоматический harness до отдельного
подтверждения.

## Ожидаемые архитектурные доработки после теста

- Publish worker/controller должны различать retryable DMCR read-only/503/
  connection reset и terminal validation/auth errors.
- Public status должен показывать `Retrying/Recovering` с bounded retry budget,
  а не сразу `Failed`.
- GC UX должен стать ближе к DVCR: visible queued/armed/running/done/timeout,
  persisted result и понятная причина, почему marker ещё жив.
- Runtime logs должны быть operation-oriented: one operation UID, phase,
  digest/layer, attempt, duration, retry delay, error class.
- State Secrets должны иметь owner/link labels, позволяющие безопасно найти и
  удалить только тестовые/orphan state.
- Для node-cache/CSI нужен отдельный chaos slice, потому что live baseline был
  проверен с `nodeCache.enabled=false`.

## Definition of done для execution continuation

- Все сценарии C0-C12 либо прошли, либо дали точный кодовый дефект с evidence.
- Для `nodeCache.enabled=true` отдельно пройдены C13-C14.
- После cleanup нет тестовых `Model`, workload, publish worker Pods, projected
  Secrets, `dmcr-gc-*`, cleanup state или publish state Secrets.
- `dmcr` и controller не имеют restart loops после инъекций.
- Все найденные gaps либо исправлены, либо вынесены в следующий task bundle с
  конкретным implementation slice.

