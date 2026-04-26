## 1. Current phase

Этап 2. `Model` / `ClusterModel` уже landed и current workstream не меняет
public source intent, но расширяет publication/distribution/runtime delivery
architecture для production-grade deployment topologies.

## 2. Orchestration

`full`

Причина:

- задача меняет больше одной области репозитория;
- это не mechanical cleanup, а новая architecture surface;
- она одновременно затрагивает storage, registry/distribution, runtime
  delivery, module config boundaries и status/observability expectations.

Read-only reviews, обязательные до implementation slices:

- `repo_architect`
  - проверить, не превращаем ли мы `images/controller` в patchwork из ещё
    одного giant runtime framework;
- `integration_architect`
  - проверить `DMZ`, registry, auth/trust, node-cache storage and HA
    boundaries;
- `api_designer`
  - проверить, что новый runtime/distribution design не тащит лишние knobs в
    public `Model.spec` и правильно держит spec/status split.

## 3. Slices

### Slice 1. Зафиксировать target architecture и seam split

Цель:

- формально отделить:
  - publication
  - distribution
  - runtime delivery
  - node-local cache management
- не допустить смешения source ingest, registry replication и workload mount
  wiring в одном contract.

Файлы/каталоги:

- `plans/active/phase2-model-distribution-architecture/*`
- при необходимости `images/controller/STRUCTURE.ru.md`
- при необходимости `images/controller/README.md`

Проверки:

- `rg -n "materialize-artifact|DMCR|modeldelivery|source mirror|cache|modelpack/oci" images/controller`

Артефакт результата:

- bundle с defendable architecture vocabulary и explicit boundary map.

### Slice 2. Спроектировать `DMZ` proxy/mirror registry scenario

Цель:

- определить production-ready scenario, где canonical OCI artifact живёт в
  internal `DMCR`, а `DMZ` registry выступает как bounded distribution tier.

Нужно зафиксировать:

- кто canonical source of truth;
- mirror/proxy vs promotion/replication semantics;
- trust/auth path для:
  - publication
  - mirror sync
  - runtime pull;
- что происходит при lag/outage `DMZ` registry;
- как это отражается в status, audit и docs.

Файлы/каталоги:

- `plans/active/phase2-model-distribution-architecture/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- возможно `images/controller/README.md`

Проверки:

- doc/plan consistency review

Артефакт результата:

- explicit `DMZ` distribution model, не ломающая current canonical publication
  contract.

### Slice 3. Зафиксировать `DMCR direct-upload v2` как единственный publication target

Цель:

- зафиксировать новый publication protocol, где `HF`/`Upload` читаются один
  раз на уровне publication unit, а published artifact собирается как manifest
  над набором sealed blob-ов вместо одного монолитного heavy layer.

Нужно зафиксировать:

- current constraints в текущем digest-first helper:
  - session start depends on final digest;
  - publisher по умолчанию мыслит heavy publication как upload whole layer;
  - full-model repack в giant blob плохо масштабируется по resume/dedupe/cache;
- почему монолитный late-digest blob тоже не идеален:
  - превращает `DMCR` в тяжёлый data-plane bottleneck, если сервер сам считает
    digest на полном потоке;
  - оставляет слишком крупную единицу возобновления и дедупликации;
  - плохо совпадает с node-cache reuse, когда модель уже состоит из отдельных
    больших файлов;
- какие части published contract обязаны оставаться стабильными, даже если
  внутри publisher/materializer меняются:
  - число слоёв;
  - порядок слоёв;
  - разбиение `weight` / `weight.config` / companion layers;
  - runtime/distribution topology поверх того же artifact by digest;
- minimum contract для нового `DMCR direct-upload v2`:
  - publication session содержит набор blob-сессий и завершается published
    manifest;
  - large source file -> dedicated raw blob session;
  - small files -> bounded bundle session where justified;
  - `complete(blobSession, digest, size, parts)` запечатывает reusable blob;
  - `complete(publicationSession)` публикует только config/manifest поверх уже
    sealed blob-set;
  - `abort` и TTL cleanup работают как на уровне blob session, так и на уровне
    publication session;
- minimum storage/index contract для этого протокола:
  - sealed blob registry хранит digest, size, mediaType, physical key, state;
  - publication session хранит file-to-blob mapping до публикации manifest;
  - duplicate digest detection и reuse происходят на уровне отдельного blob-а,
    а не только всего итогового артефакта;
- minimum contract для stream-capable publisher поверх этого протокола:
  - trusted internal publisher stream'ит large file directly to storage session;
  - digest считается на стороне trusted publisher для internal sources;
  - manifest/config assembly after all required blob units are sealed;
  - no false "streaming" via hidden temp dirs;
- preferred production shape:
  - `DMCR` остаётся control-plane coordinator, а не обязательным heavy
    data-plane relay для каждого blob;
  - trusted internal ingest может идти direct-to-storage under signed session;
  - untrusted external upload идёт в temporary staged object с
    последующим seal-in-place и одним дополнительным read без heavy rewrite;
- relation к source mirror:
  - stream from HF/upload source directly;
  - or stream from durable mirror objects;
  - or reject both as non-goal for now.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase2-model-distribution-architecture/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/... ./internal/dataplane/publishworker/...`

Артефакт результата:

- written technical verdict:
  - `DMCR direct-upload v2` is the sole target publication protocol;
  - final published artifact is manifest-over-blob-set, not monolithic blob;
  - implementation slices for publisher and backend protocol are scoped.

Текущий landed step в репозитории:

- native `modelpack/oci` уже умеет смешанную streamable object-source
  публикацию: bounded bundle для мелких companion-файлов и raw-слои для
  крупных model payload;
- live object-source planning теперь режет multi-file source не в один
  монолитный tar по умолчанию, а в bounded companion bundle плюс отдельные
  raw-слои для тяжёлых model files, сохраняя тот же published contract:
  стабильный materialized root для multi-file модели и single-file entrypoint
  для single-file publish;
- live `native-oci` publish path теперь валидирует layer plan отдельно от
  фактического upload, а one-pass direct upload применяется к raw-слоям
  отдельно, без возврата к full-layer prehash для range-capable object/local
  sources;
- live code уже переведён на единый `DMCR direct-upload v2`:
  - session start больше не требует итогового digest;
  - range-capable raw-слои публикуются controller-side за один проход с
    вычислением digest во время upload;
  - helper получает `digest/size` только на `complete(...)`;
  - старый digest-first helper удалён из live controller/dmcr кода;
- `DMCR direct-upload v2` дополнительно ужесточён на helper-стороне:
  - signed upload-session теперь имеет bounded TTL;
  - при неудаче записи repository link после sealing временный session object
    удаляется, а не остаётся лежать в backing storage;
- текущий backend seal внутри `DMCR` пока всё ещё двухшаговый на стороне
  storage backend:
  - multipart upload сначала собирается во временный объект;
  - затем helper фиксирует его под каноническим digest-addressed blob key;
  - это убирает повторное чтение исходного remote source на publish-worker, но
    ещё не убирает внутреннюю storage-side copy;
- `publishworker` уже переведён на mixed bundle/raw publish для `HuggingFace`
  `Direct/Mirror`, при этом staged single-file upload и local direct
  single-file input остаются raw;
- из live `publishworker` дополнительно убраны уже мёртвые seams старого
  publication flow:
  - helper `resolveAndPublish(...)`, который больше не вызывался после перехода
    на layer-aware publish, удалён;
  - развилка `huggingFaceSupportsStreamingPublish(...)` схлопнута до
    безусловного `SkipLocalMaterialization=true`, потому что live
    object-source path больше не оставляет отдельного non-streaming branch;
- archive input остаётся на отдельном archive-source path;
- consumer-facing materialized contract сохранён: multi-file модель по-прежнему
  собирается под стабильным корнем `model/`, а single-file publish сохраняет
  single-file entrypoint.

### Slice 4. Спроектировать node-local cache mount service

Цель:

- определить единственную целевую runtime/distribution topology, где node
  daemon может:
  - держать digest-addressed local cache;
  - монтировать model path в workload;
  - re-use one cached model across many pods on the same node;
  - clean up cold models;
  - refresh/fetch artifacts from registry per node.

Нужно зафиксировать:

- daemon vs CSI node plugin vs hybrid shape;
- workload-facing contract:
  - what gets mounted;
  - read-only vs writable;
  - mount lifecycle;
- как выглядит cutover, после которого current init-container materialization
  перестаёт быть частью target architecture;
- node cache layout and coordination;
- multi-pod same-node sharing and lock model;
- eviction/TTL/space-pressure behavior;
- behavior for distributed inference across many nodes.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `plans/active/phase2-model-distribution-architecture/*`
- external reference:
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/sds-node-configurator/docs/*`

Проверки:

- architecture consistency review

Артефакт результата:

- target design for node-local cache service as the only final runtime delivery
  topology without leaking storage-specific details into `Model.spec`.

### Slice 5. Зафиксировать boundary с `sds-node-configurator`

Цель:

- не спутать storage substrate и delivery plane.

Нужно зафиксировать:

- что именно может давать `sds-node-configurator`:
  - local storage classes
  - node-bound volumes
  - disk/VG provisioning substrate;
- что он не даёт сам по себе:
  - digest-aware model cache
  - registry pull manager
  - mount fan-out into many workloads
  - cache eviction semantics;
- какие knobs должны жить в ai-models module config:
  - cache size budget;
  - class/name of storage substrate;
  - cache policy defaults.

Файлы/каталоги:

- `plans/active/phase2-model-distribution-architecture/*`
- `docs/CONFIGURATION*.md`
- external reference:
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/sds-node-configurator/docs/LAYOUTS.ru.md`

Проверки:

- doc/plan consistency review

Артефакт результата:

- explicit integration boundary that keeps ai-models storage-agnostic above the
  cache-service layer.

### Slice 6. Нарезать bounded implementation follow-ups

Цель:

- перевести architecture bundle в sequence of executable implementation tasks.

Candidate follow-up bundles:

1. `dmz-registry-distribution`
   - registry mirror/proxy topology
   - auth/trust/status/docs
2. `dmcr-direct-upload-v2`
   - backend protocol
   - temp-session lifecycle
   - cleanup/TTL
3. `stream-capable-modelpack-publisher`
   - one-pass HF/upload -> OCI publication over `DMCR direct-upload v2`
4. `node-cache-runtime-delivery`
   - daemon/CSI-like mount service
   - cutover to single runtime topology
5. `runtime-distribution-observability`
   - metrics/events/status for distribution lag and node cache health

Текущий execution follow-up inside this slice:

1. `runtime-delivery-bridge-stabilization`
   - workload-delivery runtime `imagePullSecret` contract for materializer
   - conflict-safe `catalogstatus` status persist path
   - bounded operator-facing delivery progress/failure signal for the current
     init-container bridge path

Файлы/каталоги:

- `plans/active/phase2-model-distribution-architecture/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/cmd/ai-models-controller/*`
- docs/evidence surfaces if the runtime signal contract changes

Проверки:

- all follow-up slices have scope, non-goals, validations, rollback points
- `cd images/controller && go test ./internal/controllers/catalogstatus ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery ./cmd/ai-models-controller`
- `make verify`

Артефакт результата:

- ready-to-execute backlog without architectural ambiguity.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: новые границы и vocabulary уже
зафиксированы, но implementation roadmap ещё не привёл к runtime churn.

После Slice 3 можно безопасно остановиться, если:

- `DMZ` architecture agreed;
- `DMCR direct-upload v2` landed в live controller/dmcr коде;
- current init-container delivery path ещё не затронут.

Это всё ещё не ломает current init-container delivery path и не требует
полуготового node-cache runtime.

## 5. Final validation

- manual consistency review between:
  - new bundle
  - `plans/archive/2026/phase2-runtime-followups/*`
  - `plans/archive/2026/publication-storage-hardening/*`
- if docs are updated:
  - `make helm-template`
  - `make kubeconform`
- before any implementation slice lands:
  - narrow `go test` by touched packages
  - `make verify`
