## 1. Current phase

Этап 1 corrective closure.

Publication baseline уже работает локально и проходит repo-level quality gates.

Изначальные phase-1 долги по productized stale sweep и fast sealing уже
закрыты в этом bundle. Текущий continuation slice добивает следующий
production gap:

- HA-safe executor ownership для `dmcr-cleaner gc run` при `DMCR` HA > 1.
- orphan `direct-upload` physical residue под
  `_ai_models/direct-upload/objects/<session-id>/data`, который не покрыт
  ownership-based stale sweep.

## 2. Orchestration

`full`

Причина:

- задача меняет storage/GC boundary внутри `DMCR` и требует безопасно
  отличить published physical blobs от orphan direct-upload residue;
- пользователь явно разрешил delegation;
- перед реализацией используются read-only subagents:
  - `backend_integrator` по safe ownership модели orphan direct-upload cleanup;
  - `integration_architect` по fit с productized DMCR/virtualization GC
    semantics.

Текущие зафиксированные выводы subagents:

- orphan direct-upload cleanup должен оставаться третьей internal DMCR-owned
  stale-sweep категорией внутри `images/dmcr/internal/garbagecollection`, а не
  controller path и не generic storage janitor;
- published physical blobs должны защищаться только `.dmcr-sealed` reference
  inventory; orphan sweep не трогает canonical blob paths, repository links и
  не конкурирует с registry `garbage-collect`;
- discovery идёт в два прохода:
  - scan `.dmcr-sealed` metadata и build protected physical-path set;
  - scan `_ai_models/direct-upload/objects/*` и брать только unreferenced
    prefixes старше bounded stale-age;
- deletion обязана быть fail-closed: при ошибке metadata inventory или age
  evaluation весь orphan direct-upload report/cleanup slice останавливается
  ошибкой и не пытается делать age-only deletion;
- public contract не меняется, кроме уточнения docs/CLI semantics:
  `gc check` / `auto-cleanup` теперь видят orphan direct-upload prefixes как
  отдельную категорию stale sweep.

## 3. Slices

### Slice 1. Зафиксировать phase-1 GC и fast-seal contract

- Цель:
  - явно определить user-facing GC surface и trusted publication sealing
    boundary без phase drift.
- Файлы:
  - `plans/active/phase1-gc-sweep-and-fast-seal/*`
  - при необходимости `images/dmcr/README.md`
  - при необходимости `docs/CONFIGURATION*.md`
- Проверки:
  - `rg -n "gc.schedule|gc check|auto-cleanup|seal|trusted" plans/active images/dmcr docs openapi`
- Артефакт:
  - один явный contract:
    - stale sweep is productized;
    - direct-upload sealing avoids full-object copy;
    - digest verification continues in `dmcr-zero-trust-ingest`.

### Slice 2. Добавить stale discovery и operator CLI в dmcr-cleaner

- Цель:
  - получить `gc check` и `gc auto-cleanup` по ownership model `Model` /
    `ClusterModel`.
- Файлы:
  - `images/dmcr/cmd/dmcr-cleaner/*`
  - `images/dmcr/internal/garbagecollection/*`
  - при необходимости helper packages внутри `images/dmcr/internal/*`
- Проверки:
  - `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- Артефакт:
  - CLI умеет report/delete stale published repository/raw prefixes и запускать
    registry GC.

### Slice 3. Добавить public schedule и template/RBAC wiring

- Цель:
  - вывести stale sweep в stable module setting и wired sidecar runtime.
- Файлы:
  - `openapi/config-values.yaml`
  - `openapi/values.yaml`
  - `templates/dmcr/deployment.yaml`
  - `templates/dmcr/rbac.yaml`
  - `images/hooks/pkg/hooks/dmcr_garbage_collection/*` если потребуется
    согласование с maintenance mode
- Проверки:
  - `make test`
  - `make verify`
- Артефакт:
  - schedule wired into runtime without leaking internal secret choreography
    into public contract.

### Slice 4. Убрать full-object copy из direct-upload complete path

- Цель:
  - сделать direct-upload completion fast for controller-owned publisher.
- Файлы:
  - `images/dmcr/internal/directupload/*`
  - `images/controller/internal/adapters/modelpack/oci/*` если complete
    payload/contract изменится
- Проверки:
  - `cd images/dmcr && go test ./internal/directupload/...`
  - `cd images/controller && go test ./internal/adapters/modelpack/oci/...`
- Артефакт:
  - complete path seal'ит physical object без full copy;
  - последующий continuation `dmcr-zero-trust-ingest` заменяет trusted digest
    на `DMCR`-owned verification read.

### Slice 5. Синхронизировать docs и финальные проверки

- Цель:
  - не оставить в repo старую narrative про controller-driven-only GC и heavy
    `PublicationSealing`.
- Файлы:
  - `images/dmcr/README.md`
  - `docs/CONFIGURATION.ru.md`
  - `docs/CONFIGURATION.md`
  - `plans/active/phase1-gc-sweep-and-fast-seal/*`
- Проверки:
  - `make fmt`
  - `make test`
  - `make verify`
- Артефакт:
  - code/docs/plan surface согласованы и не спорят о GC/seal contract.

### Slice 6. Добавить lease-based executor ownership для dmcr-cleaner

- Цель:
  - гарантировать single executor для scheduled stale sweep и active GC cycle
    между несколькими replica `DMCR`.
- Файлы:
  - `images/dmcr/internal/garbagecollection/*`
  - `images/dmcr/cmd/dmcr-cleaner/*` если потребуется wiring/defaults
  - `templates/dmcr/rbac.yaml`
  - `plans/active/phase1-gc-sweep-and-fast-seal/*`
- Проверки:
  - `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
  - `make verify`
- Артефакт:
  - `gc run` использует bounded internal lease и non-holder replicas не
    мутируют cleanup state.

### Slice 7. Закрыть orphan direct-upload stale sweep

- Цель:
  - добавить bounded cleanup для completed direct-upload physical objects,
    которые не имеют active sealed reference и достаточно стары, чтобы не
    пересечься с живой публикацией.
- Файлы:
  - `images/dmcr/internal/garbagecollection/*`
  - при необходимости `images/dmcr/internal/sealedblob/*`
  - `images/dmcr/README.md`
  - `docs/CONFIGURATION.ru.md`
  - `docs/CONFIGURATION.md`
  - `plans/active/phase1-gc-sweep-and-fast-seal/*`
- Проверки:
  - `cd images/dmcr && go test ./internal/garbagecollection`
  - `make verify`
- Артефакт:
  - `gc check` показывает orphan direct-upload prefixes отдельно от stale
    repository/raw prefixes;
  - `gc auto-cleanup` удаляет только orphan direct-upload prefixes без sealed
    reference и старше bounded stale-age threshold;
  - published physical blobs продолжают удаляться только через registry
    `garbage-collect`.

### Slice 8. Сделать fast-seal checksum path explainable

- Цель:
  - убрать production-непрозрачность вокруг долгого `PublicationSealing` и
    зафиксировать safe checksum policy для generic S3-compatible backend;
  - сохранить recreate/resume path после interrupted `source-worker` во время
    `Sealing`.
- Файлы:
  - `images/dmcr/internal/directupload/*`
  - `images/controller/internal/adapters/k8s/sourceworker/*`
  - `images/controller/cmd/ai-models-artifact-runtime/*`
  - `plans/active/phase1-gc-sweep-and-fast-seal/*`
- Проверки:
  - `cd images/dmcr && go test ./internal/directupload/...`
  - `cd images/controller && go test ./internal/adapters/k8s/sourceworker/... ./cmd/ai-models-artifact-runtime`
  - `make verify`
- Артефакт:
  - `dmcr-direct-upload` явно логирует verification path:
    - trusted backend `full-object sha256`;
    - fallback reread from object storage;
  - fallback reason и backend checksum shape видны в логах без утечки secret
    data;
  - long reread large object даёт bounded progress/throughput logs;
  - `PublicationSealing` status message прямо говорит про verify+seal, а не
    выглядит как продолжающийся upload;
  - interrupted `source-worker` больше не переводит direct-upload state в
    terminal `Failed` на `context canceled`/`deadline exceeded`, поэтому
    controller сохраняет возможность recreate/resume;
  - generic S3 backend остаётся best-effort по checksum metadata и не обещает
    portable multipart `full-object sha256`.

## 4. Rollback point

После Slice 2 можно безопасно остановиться:

- stale discovery и CLI уже отделены от runtime wiring;
- direct-upload sealing ещё не тронут;
- module public contract ещё не partially migrated.

После Slice 4 controller and `DMCR` direct-upload contract должны оставаться
согласованными вместе, поэтому rollback уже делается целиком по sealing slice.

После Slice 6 rollback снова упрощается:

- lease-based ownership изолирован внутри `dmcr-cleaner gc run`;
- stale sweep / fast-seal contract уже остаются productized и не требуют
  отката вместе с executor coordination.

После Slice 7 rollback снова локален:

- orphan direct-upload sweep остаётся изолирован внутри `dmcr-cleaner`
  inventory/report/delete path;
- direct-upload publish contract не меняется;
- published blob cleanup всё ещё принадлежит registry `garbage-collect`.

После Slice 8 rollback тоже локален:

- checksum-path diagnostics изолированы внутри `dmcr-direct-upload`;
- public API / values contract не меняется;
- safe reread fallback остаётся тем же даже при откате observability slice.

## 5. Final validation

- `make fmt`
- `make test`
- `make verify`
