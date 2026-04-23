## 1. Current phase

Этап 1 corrective closure.

Publication baseline уже работает локально и проходит repo-level quality gates.

Изначальные phase-1 долги по productized stale sweep и fast sealing уже
закрыты в этом bundle. Текущий continuation slice добивает следующий
production gap:

- HA-safe executor ownership для `dmcr-cleaner gc run` при `DMCR` HA > 1.

## 2. Orchestration

`solo`

Причина:

- задача меняет public module contract, `dmcr-cleaner`, `DMCR` direct-upload
  contract, RBAC/templates и docs;
- по repo rules такая задача normally тянет read-only delegation, но в текущем
  runtime это допустимо только по явному запросу пользователя;
- поэтому архитектурные выводы фиксируются прямо в bundle и проверяются
  полноценным `make verify`.

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

## 5. Final validation

- `make fmt`
- `make test`
- `make verify`
