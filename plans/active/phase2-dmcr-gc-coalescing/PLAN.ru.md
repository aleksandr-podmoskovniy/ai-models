# Plan

## Current phase

Этап 2: `Model` / `ClusterModel`, controller publication/deletion flow и
внутренний `DMCR` publication backend уже live, нужен runtime follow-up по
удалению без per-delete `dmcr` rollout.

## Orchestration

`solo`

Boundary уже ясна и остаётся внутри текущего controller/hook/`dmcr-cleaner`
lifecycle. Отдельные read-only subagents не используются: задача не требует
нового module layout или нового public API, а текущий runtime/tool policy
позволяет закрыть этот slice локально.

## Slices

### Slice 1. Reframe delete decision around queued GC

- Цель:
  - убрать ожидание completed physical GC из `FinalizeDelete`;
  - сделать GC request enqueue terminal step для backend artifact delete path.
- Файлы:
  - `images/controller/internal/application/deletion/*`
  - `images/controller/internal/controllers/catalogcleanup/*`
- Проверки:
  - `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`
- Артефакт:
  - controller delete flow снимает finalizer после enqueue request и остаётся
    idempotent на retry.

### Slice 2. Coalesce GC in always-on dmcr-cleaner loop

- Цель:
  - сделать `dmcr-cleaner` постоянным loop sidecar;
  - отделить queued request от switched active GC request;
  - запускать maintenance cycle только после debounce window.
- Файлы:
  - `images/dmcr/internal/garbagecollection/*`
  - `images/dmcr/cmd/dmcr-cleaner/*`
  - `templates/dmcr/deployment.yaml`
- Проверки:
  - `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- Артефакт:
  - queued requests копятся без немедленного GC, active switch поднимается
    только из always-on loop, sidecar command больше не зависит от GC mode.

### Slice 3. Keep hook as maintenance-mode switch only

- Цель:
  - оставить hook ответственным только за enable/disable readonly mode по
    active switched requests.
- Файлы:
  - `images/hooks/pkg/hooks/dmcr_garbage_collection/*`
- Проверки:
  - `go test ./images/hooks/pkg/hooks/dmcr_garbage_collection/...`
- Артефакт:
  - hook больше не воспринимает queued request как сигнал для немедленного
    `dmcr` rollout.

### Slice 4. Align docs with deferred GC lifecycle

- Цель:
  - описать deferred/coalesced DMCR GC вместо synchronous per-delete GC.
- Файлы:
  - `images/dmcr/README.md`
  - `docs/CONFIGURATION.ru.md`
  - `docs/CONFIGURATION.md`
- Проверки:
  - `rg -n "maintenance/read-only|garbage collection|dmcr-cleaner" images/dmcr/README.md docs/CONFIGURATION.ru.md docs/CONFIGURATION.md`
- Артефакт:
  - runtime docs не обещают per-delete `dmcr` recreation.

## Rollback point

После Slice 1 можно безопасно остановиться и откатиться к текущему
request-and-wait flow без partially migrated `dmcr-cleaner` semantics. После
начала Slice 2 controller, hook и cleaner должны оставаться согласованными
вместе.

## Final validation

- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- `go test ./images/hooks/pkg/hooks/dmcr_garbage_collection/...`
- `make verify`
