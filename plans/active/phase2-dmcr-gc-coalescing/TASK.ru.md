# Phase 2: zero-rollout coalesced DMCR GC

## Контекст

Изначальный phase-2 delete path для `Model` / `ClusterModel` после успешного
`artifact-cleanup` сразу создавал `dmcr` garbage-collection request со
`switch`-аннотацией. Hook `images/hooks/pkg/hooks/dmcr_garbage_collection`
видел такую request Secret, переводил `dmcr` в maintenance/read-only mode через
internal values, а Deployment получал новый `checksum/config` и пересоздавался.

В результате удаление каждой опубликованной модели почти напрямую приводит к
rollout `dmcr`, хотя физический blob GC не должен быть жёстко привязан к одному
delete-событию.

Первый corrective slice уже перевёл controller delete path на queued request и
убрал нестабильный `checksum/secret`, но архитектурный хвост остался:
switched GC request всё ещё включает `garbageCollectionModeEnabled` через Helm
render, меняет `dmcr` ConfigMap и запускает controlled rollout. Нужно закрыть
именно этот runtime tail.

Этот bundle является компактным continuation workstream поверх archived
`plans/archive/2026/phase2-runtime-followups/`, но не дублирует его как
source of truth для остального publication/runtime path.

## Постановка задачи

Убрать coupling между physical DMCR GC и PodTemplate/ConfigMap rollout.
Controller должен завершать delete path после enqueue internal GC request,
`dmcr-cleaner` должен коалесцировать requests и запускать GC через runtime
write gate, а не через Helm-driven readonly ConfigMap.

## Scope

- изменить internal GC request lifecycle в controller cleanup flow;
- перевести `dmcr-cleaner` на always-on loop вместо `pause`;
- заменить Helm-driven maintenance/read-only mode на internal runtime write
  gate, который блокирует DMCR/direct-upload writes на время active GC cycle
  без изменения PodTemplate;
- усилить HA semantics runtime gate: active GC должен ждать pod-local ack quorum
  от runtime containers через cleaner-owned Lease mirror, без Kubernetes token в
  `dmcr` и `dmcr-direct-upload`;
- убрать hook-driven `aiModels.internal.dmcr.garbageCollectionModeEnabled`
  из live GC path;
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
- `images/dmcr/internal/maintenance/*` или соседняя bounded runtime gate
  boundary, если отдельный package оправдан ownership/runtime contract;
- `images/dmcr/cmd/dmcr/*`
- `images/dmcr/internal/directupload/*`
- `images/dmcr/cmd/dmcr-cleaner/*`
- `images/hooks/pkg/hooks/dmcr_garbage_collection/*`
- `templates/dmcr/configmap.yaml`
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
- active GC cycle не меняет `Deployment/dmcr` PodTemplate, `checksum/config`
  или `checksum/secret`;
- во время active GC cycle DMCR registry write methods and direct-upload write
  endpoints получают bounded maintenance response, а read paths остаются
  доступными;
- в HA режиме leader cleaner не запускает destructive GC до quorum ack от
  runtime containers, observed через namespace-local Lease objects;
- full destructive cleanup window bounded одним timeout, а maintenance gate
  duration не может быть короче этого window plus safety margin;
- hook больше не является GC maintenance switch и не пишет
  `garbageCollectionModeEnabled` для cleanup lifecycle;
- `dmcr` ConfigMap не содержит per-cycle readonly toggle;
- focused tests покрывают:
  - delete decision без ожидания completed GC;
  - request secret enqueue semantics;
  - runtime write gate blocks writes while active and allows reads;
  - direct-upload write gate blocks mutation endpoints while active;
  - `dmcr-cleaner` arming/cleanup path для queued requests;
- docs отражают, что physical DMCR GC теперь deferred/coalesced, а не
  synchronous per-model delete step and not a rollout trigger.

## Риски

- можно сломать idempotency delete flow и заново создавать лишние GC requests;
- можно получить stuck queued request, если `dmcr-cleaner` не переведёт его в
  active cycle;
- можно сделать небезопасный GC, если убрать readonly ConfigMap без
  эквивалентного runtime write gate;
- можно оставить advisory-only gate в HA, если leader cleaner начнёт GC до того,
  как standby pod mirrored state and runtime containers acknowledged it;
- можно случайно сохранить Pod-spec drift, если hook/templates всё ещё меняют
  config checksum на каждый active request.
