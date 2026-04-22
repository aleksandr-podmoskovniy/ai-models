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
2. Убрать второй полный read из controller-owned publication sealing path:
   - `DMCR direct-upload` должен перестать повторно читать heavy object после
     multipart completion;
   - direct-upload contract должен явно фиксировать, что этот fast path
     относится к trusted controller-owned publisher boundary, а не к generic
     untrusted external ingest.

## 4. Scope

- оформить public GC contract в `config-values.yaml` и module docs;
- расширить `dmcr-cleaner` CLI до operator-facing `gc check` /
  `gc auto-cleanup`;
- добавить stale published-artifact sweep по ownership model
  `Model` / `ClusterModel` against registry/raw storage prefixes;
- встроить scheduled auto-cleanup в `dmcr` Pod wiring;
- переделать direct-upload sealing contract так, чтобы trusted
  controller-owned publisher больше не вызывал full-object read inside `DMCR`;
- синхронизировать docs и bundle notes с новым fast-seal / GC contract.

## 5. Non-goals

- не проектировать generic public cleanup API на уровне `Model.spec`;
- не перепроектировать phase-2 `DMZ` / node-local cache / distribution
  topology;
- не превращать `dmcr-cleaner` в общий storage-admin swiss-army knife вне
  bounded stale sweep для `ai-models` ownership model;
- не обещать generic untrusted direct-upload fast path, если в текущем phase-1
  boundary direct-upload остаётся internal controller-owned interface;
- не делать live cluster rollout или cluster validation в этом bundle.

## 6. Затрагиваемые области

- `plans/active/phase1-gc-sweep-and-fast-seal/*`
- `images/dmcr/internal/directupload/*`
- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/cmd/dmcr-cleaner/*`
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
  - затем запускать registry `garbage-collect`.
- Sidecar wiring умеет запускать scheduled sweep без controller-triggered
  delete event.
- `DMCR` fast sealing path больше не делает full-object read внутри
  `sealUploadedObject()` для controller-owned direct-upload flow.
- В docs и code явно зафиксирована trust boundary:
  - direct-upload fast seal относится к trusted internal publisher path;
  - phase-1 не заявляет generic untrusted fast-seal guarantee.
- Тесты покрывают:
  - stale sweep detection and cleanup decisions;
  - CLI check/auto-cleanup behavior;
  - schedule wiring;
  - direct-upload complete path without backend reread.
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
