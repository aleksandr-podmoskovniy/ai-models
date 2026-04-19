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

- `rg -n "materialize-artifact|KitOps|DMCR|modeldelivery|source mirror|cache" images/controller`

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

### Slice 3. Закрыть verdict по stream-to-OCI publication

Цель:

- определить, можно ли честно делать `HF`/`Upload` -> OCI publish без full
  local staging, и что для этого реально нужно.

Нужно зафиксировать:

- current blockers в `KitOps` adapter:
  - local `ModelDir`
  - temp config/context dirs
  - pack/push shell over filesystem context;
- minimum contract для нового stream-capable publisher:
  - direct blob/layer writer;
  - manifest/config assembly;
  - digest-first publication semantics;
  - no false "streaming" via hidden temp dirs;
- relation к source mirror:
  - stream from HF/upload source directly;
  - or stream from durable mirror objects;
  - or reject both as non-goal for now.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/kitops/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase2-model-distribution-architecture/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/... ./internal/dataplane/publishworker/...`

Артефакт результата:

- written technical verdict:
  - `KitOps` path stays filesystem-based;
  - or new publisher slice is justified and scoped.

### Slice 4. Спроектировать node-local cache mount service

Цель:

- определить replacement/extension для current init-container delivery path,
  где node daemon может:
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

- target design for node-local cache service without leaking storage-specific
  details into `Model.spec`.

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
2. `stream-capable-modelpack-publisher`
   - only if Slice 3 proves it is defendable
3. `node-cache-runtime-delivery`
   - daemon/CSI-like mount service
   - fallback coexistence with init-container path
4. `runtime-distribution-observability`
   - metrics/events/status for distribution lag and node cache health

Файлы/каталоги:

- `plans/active/phase2-model-distribution-architecture/*`

Проверки:

- all follow-up slices have scope, non-goals, validations, rollback points

Артефакт результата:

- ready-to-execute backlog without architectural ambiguity.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: новые границы и vocabulary уже
зафиксированы, но implementation roadmap ещё не привёл к runtime churn.

После Slice 3 можно безопасно остановиться, если:

- `DMZ` architecture agreed;
- streaming verdict written;
- current `KitOps` path intentionally remains baseline.

Это всё ещё не ломает current init-container delivery path и не требует
полуготового node-cache runtime.

## 5. Final validation

- manual consistency review between:
  - new bundle
  - `plans/active/phase2-runtime-followups/*`
  - `plans/archive/2026/publication-storage-hardening/*`
- if docs are updated:
  - `make helm-template`
  - `make kubeconform`
- before any implementation slice lands:
  - narrow `go test` by touched packages
  - `make verify`
