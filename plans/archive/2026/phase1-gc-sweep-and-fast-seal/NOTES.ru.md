## 1. Итог текущего continuation slice

В этом bundle закрыты оба заявленных phase-1 долга:

1. Productized stale sweep для `DMCR`:
   - появился public module contract `dmcr.gc.schedule`;
   - текущий default schedule после follow-up correction:
     `*/20 * * * *`;
   - `dmcr-cleaner` теперь умеет `gc check` и `gc auto-cleanup`;
   - stale discovery сравнивает live `Model` / `ClusterModel` cleanup handles с
     фактическими repository/source-mirror prefix в storage;
   - maintenance `gc run` теперь перед `registry garbage-collect` делает тот
     же stale sweep, а schedule enqueue встроен в существующий secret-driven
     maintenance choreography без нового параллельного GC path;
   - для stale discovery и operator-facing commands добавлен cluster-scope RBAC
     на list/get `models` и `clustermodels`.
2. No-copy sealing для controller-owned publication path:
   - `DMCR direct-upload` больше не делает второй полный storage-side copy
     после `CompleteMultipartUpload(...)`;
   - continuation `plans/active/dmcr-zero-trust-ingest` заменил доверие к
     controller-provided digest на `DMCR`-owned verification read;
   - актуальная целевая картина: нет второй полной записи объекта, но есть
     один проверочный проход чтения по physical upload object.

## 2. Основные затронутые области

- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/cmd/dmcr-cleaner/cmd/gc.go`
- `images/dmcr/internal/directupload/service.go`
- `templates/dmcr/deployment.yaml`
- `templates/dmcr/rbac.yaml`
- `templates/_helpers.tpl`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `images/dmcr/README.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

## 3. Проверки

Узкие проверки:

- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/... ./internal/directupload/...`

Repo-level:

- `make verify`

Обе проверки прошли успешно локально.

## 4. Residual risk

- Закрыто continuation slice 2026-04-23: `dmcr-cleaner gc run` получил
  внутренний lease-based executor ownership на
  `coordination.k8s.io/Lease/dmcr-gc-executor`.
- Lease holder выполняет scheduled enqueue, arm/delete request secrets и
  active `auto-cleanup`; non-holder replica остаётся standby и не мутирует GC
  state.
- Lease tuning остаётся internal runtime detail и не выводится в public
  module settings.

## 5. Continuation checks 2026-04-23

Узкая проверка:

- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`

Repo-level:

- `make fmt`
- `make verify`

Результат:

- прошла успешно локально.
- `make verify` прошёл успешно локально; cluster rollout/validation не
  выполнялись.

## 6. Continuation slice 2026-04-23: orphan direct-upload GC

Read-only review inputs, зафиксированные до реализации:

- `backend_integrator`:
  - orphan direct-upload cleanup должен жить только внутри `dmcr-cleaner` как
    третья internal GC категория;
  - published physical blobs остаются под authority registry GC через
    `sealeds3.Delete()`;
  - authoritative live signal для physical object — только `.dmcr-sealed`
    metadata, не controller state.
- `integration_architect`:
  - storage inventory boundary нужно расширить до small-metadata reads и
    object timestamps, иначе безопасный orphan sweep невозможен;
  - public/operator semantics остаются на `gc check` / `gc auto-cleanup`
    без нового public knob;
  - orphan sweep должен быть fail-closed и не превращаться в generic bucket
    janitor.

- Productized stale sweep расширен на третью internal DMCR-owned категорию:
  orphan direct-upload prefixes под
  `_ai_models/direct-upload/objects/<session-id>`.
- `dmcr-cleaner gc check` и `gc auto-cleanup` теперь:
  - отдельно считают stored/referenced direct-upload prefixes;
  - находят orphan только если prefix не защищён ни одной валидной
    `.dmcr-sealed` metadata reference;
  - удаляют только такие orphan prefixes, если они старше bounded internal
    stale-age window `DMCR_DIRECT_UPLOAD_SESSION_TTL + activation delay`;
  - не трогают canonical blob paths и repository links.
- Published heavy blobs по-прежнему удаляются только через registry
  `garbage-collect` и `sealeds3.Delete()`, а не новым prefix sweeper.
- Metadata inventory сделан fail-closed:
  - если `.dmcr-sealed` metadata не читается или не парсится, весь orphan
    direct-upload report/cleanup slice падает ошибкой и не пытается делать
    age-only deletion.

Основные затронутые области:

- `images/dmcr/internal/garbagecollection/storage_s3.go`
- `images/dmcr/internal/garbagecollection/directupload_inventory.go`
- `images/dmcr/internal/garbagecollection/directupload_inventory_test.go`
- `images/dmcr/internal/garbagecollection/cleanup.go`
- `images/dmcr/internal/garbagecollection/report.go`
- `images/dmcr/internal/garbagecollection/runner.go`
- `images/dmcr/README.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `openapi/config-values.yaml`

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- `git diff --check`
- `make verify`

Результат:

- все проверки прошли локально;
- cluster rollout/validation не выполнялись.

## 7. Continuation slice 2026-04-23: fast-seal diagnostics and checksum path

Read-only review inputs, зафиксированные до реализации:

- `backend_integrator`:
  - текущий fast-path принимает только trusted `HeadObject.ChecksumSHA256`
    вместе с `ChecksumType=FULL_OBJECT`;
  - для large multipart uploads на generic AWS/S3 path это не portable fast
    path для `sha256`, поэтому reread остаётся safe baseline;
  - minimal safe patch сейчас не должен притворяться, что проблема решается
    одной checksum-настройкой, а обязан сделать checksum-path explainable:
    method, fallback reason, checksum type/presence.
- `integration_architect`:
  - checksum support для S3-compatible backend должен оставаться
    best-effort, не mandatory;
  - отсутствие/unsupported/composite checksum metadata не должно ломать
    upload, только переключать path на verification reread;
  - operator-facing improvement текущего slice — bounded observability:
    checksum path, fallback reason, progress/throughput reread без утечки
    presigned URLs, session tokens и credential-bearing data.

Implementation boundary этого continuation slice:

- не обещать generic portable multipart `full-object SHA256` на S3 backend;
- не менять current safe fallback policy:
  - trusted `full-object sha256` используется только если backend реально его
    дал;
  - иначе выполняется streaming reread physical object;
- добить observability так, чтобы live cluster triage сразу показывал:
  - trusted backend checksum vs reread;
  - причину reread;
  - backend checksum shape;
  - bounded reread progress/throughput для long `PublicationSealing`.

Фактически реализовано в коде:

- `images/dmcr/internal/directupload` теперь:
  - различает `trusted-backend-sha256` и `verification-read`;
  - логирует `fallbackReason`, `backendChecksumType`,
    `backendSHA256Present`, `availableChecksums`;
  - во время reread large object даёт bounded progress/throughput logs по 1 GiB
    шагам вместо длинной немой паузы между `verification started` и
    `verification completed`.
- `images/controller/internal/adapters/k8s/sourceworker/progress.go` теперь
  говорит `verifying and sealing`, чтобы `99%` читался как post-upload verify.
- `images/controller/cmd/ai-models-artifact-runtime/publish_worker.go` больше
  не persist-ит terminal failed state на `context canceled` /
  `deadline exceeded`; interrupted worker остаётся resumable для следующего
  reconcile.

## 8. Continuation slice 2026-04-23: delete-triggered GC fast arm

Наблюдение из live bucket triage:

- периодический sweep уже productized и wired по `dmcr.gc.schedule`, но
  delete-triggered backend cleanup всё ещё оставлял bucket грязным на время
  общего `ActivationDelay`;
- причина была не в отсутствии schedule, а в том, что controller создавал
  только queued GC request, а maintenance/read-only mode включался позже уже
  внутри `dmcr-cleaner`.

Минимальная безопасная правка:

- controller-owned delete flow теперь создаёт сразу armed GC request:
  secret получает и `ai.deckhouse.io/dmcr-gc-requested-at`, и
  `ai.deckhouse.io/dmcr-gc-switch` с одним и тем же timestamp;
- scheduled/internal requests из `dmcr-cleaner` остаются queued и продолжают
  использовать debounce/coalescing path;
- hook choreography не меняется: наличие `dmcr-gc-switch` на request secret по-
  прежнему единственный trigger для maintenance/read-only mode.

Ожидаемый operational эффект:

- после удаления `Model`/`ClusterModel` physical GC стартует без лишнего
  ожидания общего debounce окна;
- schedule path остаётся мягким и не превращает periodic sweep в постоянный
  immediate maintenance churn.

## 9. Continuation slice 2026-04-23: immediate orphan direct-upload reclaim

Наблюдение из live cluster + bucket inspection:

- armed delete-triggered GC request уже стартует maintenance cycle сразу, но
  orphan direct-upload cleanup всё ещё использует bounded stale-age
  `24h session TTL + 10m activation delay`;
- из-за этого после удаления модели physical objects под
  `_ai_models/direct-upload/objects/<session-id>/data` могут оставаться в
  bucket до суток;
- прямой global age-bypass unsafe: он может снести fresh resumable session
  другой ещё живой модели, потому что direct-upload prefix сам по себе не
  несёт owner identity.

Выбранная continuation boundary:

- delete-triggered path получает отдельный internal cleanup intent через GC
  request annotation плюс per-owner snapshot текущей unfinished
  direct-upload session token в Secret `data`;
- `dmcr-cleaner` не делает global live dependency на controller state secrets и
  не ослабляет общий stale-age policy для всех orphan prefixes;
- delete-triggered active GC cycle декодирует только exact snapshot token,
  нормализует один session prefix и удаляет его immediately, если на него нет
  sealed reference;
- manual `gc check` и scheduled sweep остаются conservative и не получают
  global age bypass.

## 10. Continuation slice 2026-04-23: relaxed direct-upload verification policy

Read-only sanity check (`backend_integrator`), зафиксированный до реализации:

- policy и фактический verification source нужно разводить:
  - policy `trusted-backend-or-client-asserted` для нового default;
  - policy `trusted-backend-or-reread` для будущего stricter path;
  - outcome/source логируется отдельно как `trusted-backend-sha256`,
    `client-asserted` или `object-reread`.
- wiring должен оставаться полностью внутри `dmcr-direct-upload`:
  - один internal env;
  - parse в `cmd/dmcr-direct-upload`;
  - policy живёт в `internal/directupload`;
  - controller contract `complete(session,digest,size,parts)` не меняется.
- operator-facing logs/docs обязаны перестать говорить `verified`, если digest
  был просто принят от controller без reread:
  - нужны `verificationPolicy`, `verificationSource`, `fallbackReason`,
    backend checksum shape и declared/final digest source.

Continuation boundary этого slice:

- phase-1 default больше не обещает mandatory reread для large direct-upload
  blobs;
- trusted backend `full-object sha256` по-прежнему принимается как strongest
  fast path;
- если trusted backend checksum нет, phase-1 default принимает
  client-declared digest/size без reread, кроме случая отсутствующего
  client digest, когда reread остаётся вынужденным;
- более жёсткий zero-trust reread остаётся отдельной internal policy, а не
  implicit default.

Фактически реализовано в коде:

- `images/dmcr/internal/directupload` теперь различает:
  - verification policy:
    - `trusted-backend-or-client-asserted` default;
    - `trusted-backend-or-reread` strict internal alternative;
  - verification source:
    - `trusted-backend-sha256`;
    - `client-asserted`;
    - `object-reread`.
- `dmcr-direct-upload` принимает один internal env
  `DMCR_DIRECT_UPLOAD_VERIFICATION_POLICY`, который parse'ится в
  `cmd/dmcr-direct-upload` и не выводится в public module values.
- При отсутствии trusted backend checksum helper по default:
  - принимает controller-declared digest;
  - использует backend size, если storage успел отдать её через attributes;
  - не reread'ит объект только ради digest mismatch detection.
- Если declared digest отсутствует, helper всё ещё вынужден reread'ить объект,
  чтобы получить canonical digest-addressed blob key.
- Service logs теперь различают `verificationPolicy` и
  `verificationSource`; wording `verification source selected` не
  притворяется strict verification там, где сработал `client-asserted`.

Проверки текущего continuation slice:

- `cd images/dmcr && go test ./internal/directupload/...`
- `cd images/dmcr && go test ./cmd/dmcr-direct-upload`

Результат:

- обе проверки прошли локально;
- `make verify` остаётся финальной repo-level проверкой после sync docs/bundle.

## 11. Continuation slice 2026-04-23: direct-upload multipart residue cleanup

Live evidence, зафиксированное до реализации:

- в кластере `k8s.apiac.ru` на момент проверки живых `Model` и
  `ClusterModel` уже не было;
- visible ordinary objects под `dmcr/` оставались только `3`, суммарно
  `16024796230` байт;
- при этом через S3 API обнаружились `3` open multipart uploads под
  `dmcr/_ai_models/direct-upload/objects/.../data`:
  - `2d16b8822c7876d8d982e48fdac1889d` — `845` parts;
  - `7ea4a3e145d3694d001aaf985cfb5156` — `1907` parts;
  - `9b09d2b873555fefa2b5230e0c73f175` — `153` parts;
- суммарно это дало `2905` outstanding parts и объяснило bucket stats вида
  `2.9k objects`, которые не совпадали с visible object tree в bucket browser.

Read-only review input (`integration_architect`):

- multipart residue должен чиститься внутри `dmcr-cleaner`, а не controller и
  не helper HTTP API;
- delete-triggered path должен использовать exact `{objectKey, uploadID}` из
  уже snapshot'нутого session token, а не replay helper `/abort` endpoint;
- operator-facing report обязан явно разводить visible direct-upload objects и
  open multipart uploads/parts.

Фактически реализовано:

- `prefixStore` расширен на bounded multipart boundary:
  - inventory open multipart uploads;
  - part counting;
  - idempotent `AbortMultipartUpload`.
- `dmcr-cleaner` теперь:
  - считает open direct-upload multipart uploads и aggregate part count;
  - отдельно репортит stale orphan multipart uploads;
  - abort'ит их в active cleanup cycle вместе с existing stale prefix delete.
- delete-triggered cleanup policy теперь декодирует из session token не только
  `objectKey`, но и `uploadID`, поэтому immediate cleanup умеет abort'ить
  exact upload session, а не только age-bypass'ить visible object prefix.
- `gc check` / `auto-cleanup` CLI wording и formatted report теперь явно
  различают:
  - `Stored direct-upload object prefixes`;
  - `Open direct-upload multipart uploads`;
  - `Open direct-upload multipart parts`;
  - `Stale orphan direct-upload multipart uploads`.
- follow-up correction после live cluster triage:
  - если в кластере вообще не осталось live `Model` / `ClusterModel`, GC
    больше не держит orphan direct-upload residue за generic `24h` age gate;
  - после `registry garbage-collect` выполняется второй targeted
    direct-upload sweep по prefixes, которые были protected до GC и могли
    стать orphan только после удаления canonical blob metadata.

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/dmcr && go test ./cmd/dmcr-cleaner/...`

Результат:

- обе проверки прошли локально;
- `make verify` остаётся финальной repo-level проверкой после sync docs/bundle.

## 12. Follow-up correction 2026-04-23: delete-triggered GC against deleting owners

Live cluster evidence после rollout предыдущего slice:

- `dmcr-cleaner gc check` уже видел `6` stale orphan direct-upload object
  prefixes и `3` stale orphan multipart uploads, то есть stale discovery path
  работал;
- при этом bytes в bucket не исчезали после delete-triggered cleanup, а в
  `d8-ai-models` оставались `ai-model-publish-state-*` secrets уже удалённых
  моделей;
- значит production gap оставался не в stale inventory, а в delete-time
  choreography между controller finalizer и active GC cycle.

Уточнённый корень дефекта:

- live inventory для `dmcr-cleaner` считал объекты с `deletionTimestamp` как
  live owners;
- из-за этого delete-triggered active GC, запущенный ещё до фактического
  исчезновения `Model`, видел repository prefix удаляемой модели как live,
  не prun'ил repository metadata и не освобождал blob references;
- request после успешного active cycle удалялся, а когда объект окончательно
  исчезал, второго active cycle уже не происходило;
- параллельно controller release finalizer не удалял publication runtime
  residue (`ai-model-publish-state-*` и related runtime secrets), если delete
  шёл по cleanup-handle path.

Фактически реализовано:

- delete-triggered active GC cycle теперь отдельно игнорирует owners с
  `deletionTimestamp`, но обычные `gc check` и scheduled/manual
  `gc auto-cleanup` сохраняют прежнюю conservative semantics и продолжают
  считать deleting owners live до их фактического исчезновения;
- `catalogcleanup` перед снятием finalizer всегда добивает
  `deletePublicationRuntimeResources`, поэтому после completed cleanup path не
  остаются publication state secrets и другие source-worker/upload-session
  остатки в `d8-ai-models`.

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/controller && go test ./internal/controllers/catalogcleanup`
- `make verify`

Результат:

- все три проверки прошли локально.

## 13. Follow-up correction 2026-04-23: startup backfill for scheduled GC

Live cluster evidence:

- fresh publish/delete/GC smoke for
  `ai-models-smoke/tiny-random-phi-gc-20260423-1` worked end-to-end in the
  current build:
  - cleanup job completed;
  - delete-triggered `dmcr-gc-*` request was created and armed;
  - `dmcr` entered and left maintenance mode;
  - post-cycle `dmcr-cleaner gc check` was clean.
- historical S3 residue existed only because no active delete-triggered or
  scheduled request was present after the latest rollout.
- `dmcr-cleaner gc run --schedule=0 2 * * *` computed the next future cron
  tick and did not backfill the missed daily tick after the `18:25 UTC`
  rollout.

Уточнённый корень дефекта:

- fresh delete path is healthy;
- stale discovery path is healthy;
- missing part is trigger/backfill behavior for already-existing stale residue
  after `dmcr` restarts after the configured schedule tick.

Фактически реализовано:

- `schedulePlanner` tracks startup backfill state per runner start.
- `gc run` now performs the startup backfill only when schedule is configured
  and the current replica is the lease holder.
- the startup path first checks existing GC request secrets:
  - if another queued/active request exists, it does nothing;
  - if no request exists, it runs report-only stale discovery through the same
    `Check` path used by `dmcr-cleaner gc check`.
- transient `Check` failures do not mark startup backfill completed; the helper
  logs the failure and retries after a bounded delay instead of hammering S3 on
  every rescan tick.
- if the report contains stale repositories, raw prefixes, orphan
  direct-upload prefixes, or stale direct-upload multipart uploads, the helper
  creates the normal `dmcr-gc-scheduled` request.
- the startup path does not run destructive cleanup directly and does not
  bypass activation-delay or maintenance-mode choreography.
- empty buckets no longer produce a maintenance cycle on every restart.
- `images/dmcr/README.md` and `docs/CONFIGURATION*.md` now document the
  startup backfill behavior as part of the existing scheduled stale-sweep
  contract.

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/dmcr && go test ./cmd/dmcr-cleaner/...`
- `make verify`

Результат:

- все три проверки прошли локально.

## 14. Follow-up correction 2026-04-23: default GC cadence 20 minutes

Причина:

- live эксплуатация показала, что daily default слишком медленный для
  обнаруженного orphan S3 residue;
- startup backfill закрывает rollout-after-tick gap, но обычная periodic
  эксплуатация всё равно должна иметь bounded cadence без ручного patch.

Фактически реализовано:

- default `dmcr.gc.schedule` сменён с `0 2 * * *` на `*/20 * * * *`;
- runtime fallback в `templates/_helpers.tpl` использует тот же default;
- `openapi/config-values.yaml` и generated/internal defaults в
  `openapi/values.yaml` синхронизированы;
- `docs/CONFIGURATION*.md` больше не описывают daily `02:00 UTC`.

Ограничение:

- schedule по-прежнему только создаёт queued scheduled request;
- cleanup не запускается напрямую и проходит через existing
  activation-delay / maintenance-mode choreography.

Проверки:

- `make helm-template`
- `make verify`

Результат:

- обе проверки прошли локально;
- rendered manifests содержат `--schedule=*/20 * * * *`.
