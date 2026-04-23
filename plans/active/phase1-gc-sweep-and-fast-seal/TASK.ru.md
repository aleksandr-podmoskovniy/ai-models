## 1. Заголовок

Phase-1 closure: productized stale GC и fast sealing для controller-owned publication

## 2. Контекст

Текущий phase-1 baseline уже даёт:

- controller-owned publication в OCI `ModelPack` artifacts;
- внутренний `DMCR` с direct-upload ingest и `sealeds3`-based physical/logical
  blob split;
- deferred/coalesced internal GC request lifecycle вместо жёсткого
  per-delete `dmcr` rollout.

Но по итогам live triage и локального review остаются два не закрытых phase-1
долга:

1. `DMCR` cleanup остаётся только controller-driven:
   - нет public `gc.schedule`;
   - нет operator-facing `gc check` surface;
   - нет productized stale-object sweep по паттерну `virtualization`.
2. `PublicationSealing` для больших моделей остаётся тяжёлой latency точкой:
   - `CompleteMultipartUpload(...)` уже не делает full backend copy;
   - но `DMCR` всё ещё перечитывает весь загруженный object в
     `sealUploadedObject()` только ради digest/size verification.

Continuation на 2026-04-22 после закрытия этих двух долгов:

3. `dmcr-cleaner gc run` всё ещё не гарантирует single executor ownership при
   `DMCR` HA > 1:
   - sidecar запущен в каждой replica `DMCR`;
   - schedule enqueue, arm/delete request secrets и `auto-cleanup` исполняются
     без lease-based coordination;
   - при takeover/failover возможен duplicate maintenance cycle или гонка за
     cleanup ownership.

Continuation на 2026-04-23 после live inspection bucket layout:

4. Productized stale sweep всё ещё не закрывает orphan `direct-upload`
   physical objects:
   - опубликованные heavy blobs живут под
     `_ai_models/direct-upload/objects/<session-id>/data` и защищены
     `.dmcr-sealed` metadata;
   - текущий stale sweep сравнивает только repository/source-mirror prefixes и
     не инвентаризирует orphan completed direct-upload objects без sealed
     reference;
   - из-за этого незавершённые/брошенные direct-upload uploads могут оставлять
     physical residue в bucket даже при рабочем `gc check` / `auto-cleanup`.

Предшественники:

- `plans/active/live-cluster-error-triage/*`
- `plans/active/dmcr-zero-trust-ingest/*`
- `plans/active/phase2-dmcr-gc-coalescing/*`

Этот bundle не переписывает их историю, а закрывает оставшийся phase-1 gap
компактным continuation workstream.

## 3. Постановка задачи

Нужно закрыть оба долга production-grade способом без phase drift:

1. Дать `ai-models` productized stale cleanup surface по границе phase-1:
   - public module setting наподобие `virtualization.dvcr.gc.schedule`;
   - operator-facing `dmcr-cleaner gc check`;
   - scheduled `dmcr-cleaner gc auto-cleanup`, который ищет stale published
     model repositories и связанные raw/source-mirror prefixes без привязки к
     delete-событию конкретного `Model`.
2. Убрать второй полный copy из controller-owned publication sealing path:
   - `DMCR direct-upload` не должен переписывать heavy object после multipart
     completion;
   - дальнейшая проверка digest вынесена в continuation
     `plans/active/dmcr-zero-trust-ingest`.
3. Дожать `dmcr-cleaner` до HA-safe runtime ownership:
   - scheduled sweep и active GC request cycle должны исполняться только одним
     live executor;
   - ownership должен оставаться bounded internal runtime concern, а не новым
     public module contract.
4. Добить stale sweep до orphan direct-upload residue:
   - `dmcr-cleaner gc check` должен видеть orphan
     `_ai_models/direct-upload/objects/<session-id>` prefixes, которые не
     защищены `.dmcr-sealed` metadata и пережили bounded stale-age threshold;
   - `dmcr-cleaner gc auto-cleanup` должен удалять только такие orphan
     prefixes, не конкурируя с registry `garbage-collect` за опубликованные
     physical blobs.

## 4. Scope

- оформить public GC contract в `config-values.yaml` и module docs;
- расширить `dmcr-cleaner` CLI до operator-facing `gc check` /
  `gc auto-cleanup`;
- добавить stale published-artifact sweep по ownership model
  `Model` / `ClusterModel` against registry/raw storage prefixes;
- встроить scheduled auto-cleanup в `dmcr` Pod wiring;
- добавить bounded lease-based executor ownership для `dmcr-cleaner gc run`;
- добавить bounded orphan direct-upload discovery/cleanup, который не удаляет
  published physical blobs, пока на них есть sealed reference;
- переделать direct-upload sealing contract так, чтобы controller-owned
  publisher больше не вызывал full-object copy inside `DMCR`;
- синхронизировать docs и bundle notes с новым fast-seal / GC contract.

## 5. Non-goals

- не проектировать generic public cleanup API на уровне `Model.spec`;
- не перепроектировать phase-2 `DMZ` / node-local cache / distribution
  topology;
- не превращать `dmcr-cleaner` в общий storage-admin swiss-army knife вне
  bounded stale sweep для `ai-models` ownership model;
- не обещать generic untrusted direct-upload fast path, если в текущем phase-1
  boundary direct-upload остаётся internal controller-owned interface;
- не выводить lease/election tuning в public module settings без отдельной
  необходимости;
- не делать live cluster rollout или cluster validation в этом bundle.

## 6. Затрагиваемые области

- `plans/active/phase1-gc-sweep-and-fast-seal/*`
- `images/dmcr/internal/directupload/*`
- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/cmd/dmcr-cleaner/*`
- при необходимости `images/dmcr/internal/sealedblob/*`
- `templates/dmcr/*`
- `templates/dmcr/rbac.yaml`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `images/hooks/pkg/hooks/dmcr_garbage_collection/*`
- `images/dmcr/README.md`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`

## 7. Критерии приёмки

- В module public contract появился bounded GC setting:
  - `dmcr.gc.schedule` или эквивалентный stable user-facing slice;
  - docs объясняют schedule semantics и не выдают внутренние request secrets за
    user-facing contract.
- `dmcr-cleaner gc check` локально умеет:
  - перечислять stale published repository/raw prefixes, ownership которых не
    подтверждается живыми `Model` / `ClusterModel`;
  - не удалять данные;
  - выдавать объяснимый report, пригодный для operator use.
- `dmcr-cleaner gc auto-cleanup` локально умеет:
  - использовать тот же stale discovery surface;
  - удалять stale published metadata/raw prefixes;
  - удалять orphan direct-upload prefixes только если они не защищены sealed
    reference, older than bounded stale-age threshold и удаляются строго по
    одному session prefix без prefix-collision с соседними session IDs;
  - затем запускать registry `garbage-collect`.
- Sidecar wiring умеет запускать scheduled sweep без controller-triggered
  delete event.
- При `DMCR` HA runtime cleanup ownership bounded:
  - только lease holder имеет право enqueue scheduled request, arm/delete
    request secrets и запускать `auto-cleanup` из `gc run`;
  - non-holder replica остаётся standby и не мутирует GC state;
  - ownership реализован внутренним lease в namespace `DMCR`, без нового
    public values contract.
- `DMCR` direct-upload sealing path больше не делает full-object copy для
  controller-owned direct-upload flow.
- В docs и code явно зафиксирована continuation boundary:
  - этот bundle закрывает no-copy storage layout;
  - `dmcr-zero-trust-ingest` закрывает `DMCR`-owned digest verification.
- Тесты покрывают:
  - stale sweep detection and cleanup decisions;
  - orphan direct-upload detection against sealed references;
  - bounded age gate for orphan direct-upload cleanup;
  - malformed `.dmcr-sealed` metadata fail-closed behavior;
  - exact direct-upload delete boundary без `session-1` / `session-10`
    over-delete;
  - CLI check/auto-cleanup behavior;
  - schedule wiring;
  - lease acquisition/renew/takeover и non-holder standby behavior;
  - direct-upload complete path without full backend copy.
- Перед завершением проходит:
  - `make fmt`
  - `make test`
  - `make verify`

## 8. Риски

- можно размыть GC surface и случайно тащить phase-2/phase-3 storage-admin
  задачи в phase-1;
- можно сделать stale sweep неидемпотентным и получить destructive false
  positives;
- можно убрать `sealUploadedObject()` без явной фиксации trusted boundary и
  тем самым незаметно ослабить security narrative;
- можно смешать productized stale sweep с legacy internal queued-request GC и
  оставить две конкурирующие cleanup истории вместо одной explainable модели.
- можно сделать lease ownership слишком хрупким и получить stuck cleanup или,
  наоборот, duplicate executor при failover.
