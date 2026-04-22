## 1. Current phase

Этап 1 corrective closure.

Publication baseline уже работает локально и проходит repo-level quality gates,
но до production-ready phase-1 результата всё ещё не хватает:

- productized stale sweep по паттерну `virtualization`;
- fast sealing без второго полного read на controller-owned publication path.

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
    - fast sealing is trusted controller-owned path.

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

### Slice 4. Убрать full-object reread из trusted direct-upload complete path

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
  - complete path seal'ит physical object без backend reread и без full copy,
    а trust boundary задокументирована явно.

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

## 4. Rollback point

После Slice 2 можно безопасно остановиться:

- stale discovery и CLI уже отделены от runtime wiring;
- direct-upload sealing ещё не тронут;
- module public contract ещё не partially migrated.

После Slice 4 controller and `DMCR` direct-upload contract должны оставаться
согласованными вместе, поэтому rollback уже делается целиком по sealing slice.

## 5. Final validation

- `make fmt`
- `make test`
- `make verify`
