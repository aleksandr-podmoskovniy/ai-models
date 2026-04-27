# План

## Orchestration

Изначальный planning slice был `solo`, потому что фиксировал chaos matrix без
изменения runtime code.

Execution continuation 2026-04-26 перешёл в live-evidence-driven runtime/HA
fixes после подтверждённой гонки `DMCR` GC против active direct-upload. Для
таких изменений целевой режим — минимум `light`, а при широкой смене
controller/DMCR/runtime поведения — `full` с `integration_architect` и финальным
`reviewer`.

Фактическая проверка continuation:

- `integration_architect` read-only review выполнен после реализации.
- финальный `reviewer` read-only review выполнен после реализации.
- оба review указали на один и тот же дефект: malformed/rotated
  direct-upload token не должен валить весь `DMCR` GC cycle.
- `reviewer` дополнительно указал workflow gap: выводы subagents должны быть
  записаны в bundle.

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

## Execution continuation 2026-04-26

### Scope

- Дождаться rollout новой сборки в `k8s.apiac.ru`.
- Проверить module/controller/DMCR/node-cache runtime health перед тестом.
- Прогнать controlled e2e/chaos subset из matrix, начиная с безопасных
  pod-level сценариев.
- Фиксировать evidence по status, events, logs, state Secrets, GC requests и
  cleanup.
- Исправлять только подтверждённые code defects; live/transient cluster issues
  фиксировать отдельно как operations findings.

### Rollout wait policy

- Подождать 5 минут после пользовательского push.
- Проверить rollout новой сборки.
- Если новая сборка ещё не выкатилась, подождать ещё 5 минут.
- После этого проверять readiness каждую минуту до явного успеха или
  диагностируемого blocker.

### Execution guardrails

- Использовать отдельный timestamped namespace.
- Не ребутать ноды без отдельного подтверждения.
- Начать с C0/C1/C3/C5/C8/C10/C11; расширять до C13/C14 только если
  `nodeCache.enabled=true` реально включён в live module config.
- Останавливать тест при росте ошибок RGW/Ceph/DMCR вне тестового namespace.

### Execution findings 2026-04-26

- Rollout check: `ModulePullOverride/ai-models` already pointed to
  `main@sha256:e68a3d0be875e91c6a5b4d12b5980374f04215cb4a09ffabef24edda5c4f6df9`;
  repeated checks did not change controller/DMCR pod templates.
- Registry/CI check: GitHub build for `5b10545` completed successfully, but
  `crane copy` reused the existing bundle manifest because the commit only
  changed governance/plans and did not change module payload.
- Live `ModuleConfig` has no `nodeCache.enabled=true`, so C13/C14 are blocked
  until that setting is explicitly enabled.
- C0 failed before chaos injection: scheduled DMCR GC ran during active
  HuggingFace publication and removed fresh direct-upload multipart state.
- Evidence: Model became terminal `Failed` with `NoSuchUpload` / `NoSuchKey`
  while `dmcr-garbage-collection` logged a scheduled cycle with
  `open_direct_upload_multipart_upload_count=1` and
  `stale_direct_upload_multipart_upload_count=1` at the same time.
- Root cause: GC treated fresh direct-upload prefixes/uploads as immediately
  stale whenever no cleanup-state owners existed. Active publication does not
  create cleanup-state owner objects, so scheduled GC could race normal upload.
- Fix: non-targeted direct-upload prefixes and multipart uploads are now always
  age-bounded. Explicit delete/cleanup requests still target exact
  direct-upload prefixes/uploads derived from cleanup state token.
- Follow-up hardening: controller no longer writes direct-upload immediate mode
  on GC requests without a session token; DMCR ignores old no-token immediate
  requests as plain GC requests; target direct-upload prefixes are normalized
  before matching.
- Legacy cleanup: removed the dead `ignoreDeletingOwners` GC policy flag. Live
  inventory is driven by module-private cleanup-state Secrets, so the flag no
  longer had any runtime semantics.
- Direct-upload API retry hardening: client retries transient transport errors
  and backend `500/502/503/504` responses, while keeping `4xx` validation/auth
  failures terminal. Retry/error classification was split out of
  `direct_upload_client.go` to keep the file under the controller LOC budget.
- Direct-upload object-store backoff: part upload recovery now waits with
  bounded exponential backoff when `listParts` shows no progress after a
  failed PUT. If the part was actually committed, publishing continues without
  delay. This avoids tight retry loops against RGW/S3 during transient storage
  failures.
- DMCR GC runner structure was split without behavior changes:
  maintenance-gate sync/ack/release moved to `maintenance_gate.go`, registry
  shell execution moved to `cleanup_exec.go`, and matching tests were split by
  decision surface. This removes the remaining >350 LOC runner/test hotspots in
  the touched GC boundary.
- Review fixes: malformed, signature-mismatched, missing-secret or invalid-path
  direct-upload targeted tokens now degrade to plain age-bounded GC instead of
  failing the whole active GC cycle. Valid tokens in the same batch are still
  honored.
- Review fixes: direct-upload API transport retry is now narrowed. Transient
  connection reset/refused/timeout/EPIPE/temporary DNS errors retry, while TLS
  certificate errors and DNS not-found fail fast as terminal configuration
  failures.
- DMCR GC result persistence: successful active request Secrets now move to
  `phase=done`, drop the direct-upload session token, store bounded
  `result.json` with counts/registry output and keep `completed-at` for
  operator inspection. Completed request Secrets are ignored by active/queued
  selection and pruned after `completed-request-ttl`.
- Controller delete observation now treats `phase=done` as internal
  `GarbageCollectionStateComplete`, so a persisted private GC result does not
  look like a stuck queued request.
- Runtime observability alignment: `runtimehealth` is now registered
  independently of `nodeCache.enabled`, so workload-delivery metrics are
  present for the current MaterializeBridge baseline. The same collector also
  exports private `DMCR` GC request counts by phase and per-request age, making
  queued/armed/done lifecycle machine-visible without turning GC request
  Secrets into public API.
- LOC cleanup: `cmd/ai-models-controller/config.go` was split so config
  parsing/validation and bootstrap wiring are separate files below the
  controller LOC budget.
- RBAC/legacy cleanup: `DMCR` service account no longer gets cluster-wide
  `Model` / `ClusterModel` reads. GC live inventory is explicitly based on
  module-private cleanup-state Secrets, and stale test scaffolding that
  pretended model annotations were still a live source was removed.
- Follow-up RBAC/legacy cleanup: `dmcr-cleaner gc check` help text was aligned
  with the private cleanup-state ownership model, so operator-facing CLI no
  longer advertises live `Model` / `ClusterModel` ownership as the source of
  truth.
- Follow-up virtualization-style GC hardening: direct operator-facing
  `dmcr-cleaner gc auto-cleanup` was removed. Destructive cleanup now stays
  behind `dmcr-cleaner gc run`, so every physical registry/source/direct-upload
  cleanup goes through request coalescing, the zero-rollout maintenance gate,
  pod-local ack quorum and persisted result state. The remaining internal
  `AutoCleanup*` naming was also removed so code vocabulary matches the gated
  lifecycle.

### Validation 2026-04-26

- `cd images/dmcr && go test ./internal/garbagecollection` passed.
- `cd images/dmcr && go test ./...` passed.
- `make verify` passed.
- After follow-up hardening, `cd images/controller && go test
  ./internal/controllers/catalogcleanup`, `cd images/dmcr && go test
  ./internal/garbagecollection`, and `make verify` passed again.
- After direct-upload retry hardening, `cd images/controller && go test
  ./internal/adapters/modelpack/oci ./internal/controllers/catalogcleanup`,
  `cd images/dmcr && go test ./internal/garbagecollection`, and `make verify`
  passed.
- After part-upload recovery backoff, the same targeted tests and `make verify`
  passed.
- After DMCR GC runner split, `cd images/dmcr && go test
  ./internal/garbagecollection`, `git diff --check`, and `make verify` passed.
- After read-only review fixes, `cd images/dmcr && go test
  ./internal/garbagecollection`, `cd images/controller && go test
  ./internal/adapters/modelpack/oci ./internal/controllers/catalogcleanup`,
  `git diff --check`, and `make verify` passed.
- After DMCR GC result persistence, `cd images/dmcr && go test
  ./internal/garbagecollection ./cmd/dmcr-cleaner/cmd`, `cd
  images/controller && go test ./internal/application/deletion
  ./internal/controllers/catalogcleanup ./internal/adapters/modelpack/oci`, and
  `git diff --check` passed.
- After runtime observability alignment, `cd images/controller && go test
  ./internal/monitoring/runtimehealth ./internal/bootstrap
  ./cmd/ai-models-controller`, `cd images/dmcr && go test
  ./internal/garbagecollection ./cmd/dmcr-cleaner/cmd`, `git diff --check`,
  and `make verify` passed.
- After RBAC/legacy cleanup, `cd images/dmcr && go test
  ./internal/garbagecollection ./cmd/dmcr-cleaner/cmd`, `cd images/controller
  && go test ./internal/monitoring/runtimehealth ./internal/bootstrap
  ./cmd/ai-models-controller ./internal/controllers/catalogcleanup
  ./internal/application/deletion ./internal/adapters/modelpack/oci`,
  `git diff --check`, and `make verify` passed.
- Render evidence: `templates/dmcr/rbac.yaml` renders only a namespaced
  `Role`/`RoleBinding` for module-private `Secret`/`Lease` state; no
  `d8:ai-models:dmcr` `ClusterRole`/`ClusterRoleBinding` remains.
- After removing direct `gc auto-cleanup`, `cd images/dmcr && go test
  ./cmd/dmcr-cleaner/cmd ./internal/garbagecollection` passed.
- Final post-cleanup validation: no live code/docs/templates/render output
  contain `auto-cleanup`, `AutoCleanup` or `autoCleanup`; `git diff --check`
  and `make verify` passed.
- Live hard e2e must resume only after the DMCR GC fix is built and deployed;
  the current live bundle still contains the race.

### Live rollout check 2026-04-26 23:04 MSK

- First kubeconfig current-context check showed `k8s-dvp.apiac.ru`, where
  `ai-models` is disabled and `d8-ai-models` has no pods. All meaningful
  rollout checks were then run explicitly with `--context k8s.apiac.ru`.
- `k8s.apiac.ru` module state was `Ready`, source
  `aleksandr-podmoskovniy-ghcr`, version `main`, deployed release
  `ai-models-v0.0.7`.
- Initial live pods were old: controller image
  `sha256:b688ae187ed87cb327b1b3de3b7f09cfc148794803be77cf6d69aab51b3be602`,
  DMCR image
  `sha256:a3e769930047b1ebc2c96f0750dfb377028fefd0abf7306f72c29da4ee531233`.
- During polling the cluster rolled to controller image
  `sha256:586e460c4d7fd127d727f81a2658e432836f5176a098317049c4f38f8d7cb528`
  and DMCR image
  `sha256:107c052898643b3c1a915fd037549b0c73f337d4efbedb7bcae796f03dfa552c`,
  both deployments reached `2/2` ready.
- Contract check inside the new DMCR pod still showed
  `dmcr-cleaner gc auto-cleanup` and old `gc check` help text referencing live
  `Model` / `ClusterModel` ownership. This proves the live image does not
  include the latest local cleanup-bypass removal.
- Hard e2e/chaos was intentionally not started because it would validate the
  wrong runtime contract. Resume only after the current local diff is committed,
  pushed, built and the in-pod `dmcr-cleaner gc --help` no longer lists
  `auto-cleanup`.

### Live rollout recheck 2026-04-26 23:15 MSK

- Waited another 5 minutes before checking the cluster again.
- Polling attempts at `23:15` through `23:20` MSK still showed the same live
  controller image
  `sha256:586e460c4d7fd127d727f81a2658e432836f5176a098317049c4f38f8d7cb528`
  and DMCR image
  `sha256:107c052898643b3c1a915fd037549b0c73f337d4efbedb7bcae796f03dfa552c`.
- In-pod contract check still reported `dmcr-cleaner gc auto-cleanup`, so the
  cleanup-bypass removal was not live.
- Local git check showed `HEAD` equals `origin/main`, while the working tree
  still contains the uncommitted/unpushed diff removing `auto-cleanup` and
  renaming internal `AutoCleanup*` vocabulary. Polling was stopped because the
  cluster cannot roll out a diff that has not reached the remote build source.

### Live chaos retry 2026-04-26 23:51 MSK

- Fresh rollout check on `k8s.apiac.ru` showed module `ai-models` Ready and
  newly rolled controller/DMCR pods. In-pod `dmcr-cleaner gc --help` no longer
  exposed the legacy `auto-cleanup` command; the live contract has only
  `check` and `run`.
- The cluster still has both legacy `ai-models.deckhouse.io` and current
  `ai.deckhouse.io` CRDs installed. Short `kubectl get model` resolves to the
  legacy group on this cluster, so chaos harnesses must use fully-qualified
  `models.ai.deckhouse.io` and `clustermodels.ai.deckhouse.io` resource names.
- Cleanup attempt for empty legacy CRDs
  `models.ai-models.deckhouse.io` / `clustermodels.ai-models.deckhouse.io` was
  blocked by cluster `ValidatingAdmissionPolicy` `label-objects.deckhouse.io`
  because those CRDs carry protected `heritage: deckhouse` labels. This is a
  live-ops cleanup blocker outside module code; harnesses must stay explicit
  until platform-owned CRD migration cleanup is performed.
- C1 found a real runtime HA defect before direct-upload started: deleting the
  source worker during HuggingFace metadata fetch made the current
  `ai.deckhouse.io/v1alpha1` `Model` terminal `Failed` with
  `Get "https://huggingface.co/api/models/Qwen/Qwen2.5-0.5B-Instruct": context canceled`.
- Evidence namespace:
  `/tmp/ai-models-chaos-20260426235148`; test Model
  `ai-models-chaos-20260426235148/qwen-chaos-20260426235148`; failed condition
  reason `PublicationFailed`.
- Root cause: `sourceworker` only recreated failed pods when direct-upload
  state had already entered `Running`. A worker interrupted before
  direct-upload state existed was returned as a failed runtime handle and then
  projected into a business publication failure.
- Fix: `internal/adapters/k8s/sourceworker` now treats failed publish pods with
  runtime-loss termination messages such as `context canceled` or
  `signal: terminated` as retryable pod loss and recreates them before
  returning a failed handle to catalog status.
- Test evidence: `cd images/controller && go test
  ./internal/adapters/k8s/sourceworker ./internal/application/publishobserve
  ./internal/controllers/catalogstatus` passed.
- Repo evidence after the fix: `git diff --check` and `make verify` passed.
- Repeat live chaos is blocked until this source-worker recovery fix is built
  and rolled out to the cluster.

### Review residual risks 2026-04-26

- GC result persistence is now implemented as module-private Secret state, not
  public `Model`/`ClusterModel` status. A later UX slice can project a compact
  user-facing summary if product UX needs it.
- If controller cannot snapshot a direct-upload session token, delete-time
  cleanup safely falls back to age-bounded cleanup. This can leave partial
  direct-upload objects until stale-age expiry, but avoids unsafe broad
  deletion.
- Live hard e2e remains blocked until this code is built and deployed.
