# Phase 2: coalesced DMCR GC without per-delete rollout

## Контекст

Текущий phase-2 delete path для `Model` / `ClusterModel` после успешного
`artifact-cleanup` сразу создаёт `dmcr` garbage-collection request со
`switch`-аннотацией. Hook `images/hooks/pkg/hooks/dmcr_garbage_collection`
видит такую request Secret, переводит `dmcr` в maintenance/read-only mode через
internal values, а Deployment получает новый `checksum/config` и пересоздаётся.

В результате удаление каждой опубликованной модели почти напрямую приводит к
rollout `dmcr`, хотя физический blob GC не должен быть жёстко привязан к одному
delete-событию.

Этот bundle является компактным continuation workstream поверх
`plans/active/phase2-runtime-followups/`, но не дублирует его как source of
truth для остального publication/runtime path.

## Постановка задачи

Убрать per-delete coupling между удалением модели и переключением `dmcr` в
maintenance mode. Controller должен завершать delete path после enqueue
internal GC request, а сам physical DMCR GC должен запускаться отложенно и
коалесцировать несколько pending requests в один maintenance cycle.

## Scope

- изменить internal GC request lifecycle в controller cleanup flow;
- перевести `dmcr-cleaner` на always-on loop вместо `pause`;
- оставить hook ответственным только за перевод `dmcr` в maintenance/read-only
  mode, но не за немедленный запуск GC после каждого delete;
- обновить focused tests и runtime docs по новому GC lifecycle.

## Non-goals

- не перепроектировать `DMCR` backend или его storage engine;
- не вводить новый public/API contract или values knob для GC scheduling;
- не менять publication path, OCI auth contract или artifact-cleanup job shape;
- не делать governance/workflow changes.

## Затрагиваемые области

- `images/controller/internal/application/deletion/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/cmd/dmcr-cleaner/*`
- `images/hooks/pkg/hooks/dmcr_garbage_collection/*`
- `templates/dmcr/deployment.yaml`
- `images/dmcr/README.md`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`

## Критерии приёмки

- удаление published `Model` / `ClusterModel` больше не ждёт завершения
  physical DMCR GC и не держит finalizer до конца maintenance cycle;
- controller создаёт/обновляет internal GC request queue entry после
  completed cleanup job и снимает finalizer в том же delete flow;
- `dmcr-cleaner` работает как always-on loop и сам переводит pending requests в
  active GC cycle только после internal debounce/coalescing window;
- hook включает `garbageCollectionModeEnabled` только по active switched
  requests, а не по любому новому delete request;
- focused tests покрывают:
  - delete decision без ожидания completed GC;
  - request secret enqueue semantics;
  - hook activation только по switched request;
  - `dmcr-cleaner` arming/cleanup path для queued requests;
- docs отражают, что physical DMCR GC теперь deferred/coalesced, а не
  synchronous per-model delete step.

## Риски

- можно сломать idempotency delete flow и заново создавать лишние GC requests;
- можно получить stuck queued request, если `dmcr-cleaner` не переведёт его в
  active cycle;
- можно случайно сохранить Pod-spec drift, если sidecar command останется
  mode-dependent и продолжит провоцировать rollout без нужды.
