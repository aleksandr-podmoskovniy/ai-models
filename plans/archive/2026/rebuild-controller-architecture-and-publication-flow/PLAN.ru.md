# Plan

## Current phase

Этап 2. `Model` / `ClusterModel`, контроллер публикации и platform UX поверх internal backend.

## Orchestration

- mode: `full`
- read-only subagents before code changes:
  - virtualization-pattern audit;
  - current controller drift audit;
- final substantial review:
  - `review-gate`
  - `reviewer`

## Slices

### Slice 1. Rebaseline controller boundaries

Цель:
- зафиксировать и реализовать новую internal structure controller-а;
- убрать fat live reconciler как центр архитектуры.

Файлы/каталоги:
- `images/controller/internal/app/*`
- `images/controller/internal/modelpublish/*`
- новые bounded packages под `images/controller/internal/*`
- `plans/active/rebuild-controller-architecture-and-publication-flow/*`

Проверки:
- `go test ./...` в `images/controller`

Артефакт:
- controller structure с явными lifecycle/execution/cleanup границами.

### Slice 2. Durable publication operation baseline

Цель:
- вынести `source -> publish -> inspect -> result` в отдельный operation contract;
- заменить brittle handoff path на durable internal result channel.

Файлы/каталоги:
- `images/controller/internal/*publication*`
- worker/result contracts
- controller runtime wiring

Проверки:
- `go test ./...` в `images/controller`

Артефакт:
- bounded publication operation, на который lifecycle reconciler может опираться.

### Slice 3. Source-first live implementation baseline

Цель:
- продвинуть working path для `spec.source`;
- минимум один live scenario остаётся рабочим уже на новой архитектуре.

Файлы/каталоги:
- `api/core/v1alpha1/*` при необходимости
- `images/controller/internal/*`
- `templates/controller/*`

Проверки:
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `go test ./...` в `api`
- `go test ./...` в `images/controller`

Артефакт:
- рабочий publish path `source -> artifact -> resolved -> ready`.

### Slice 4. Backend/access follow-up

Цель:
- подготовить clean internal path для object storage и OCI;
- зафиксировать controller-owned auth/access shape.

Файлы/каталоги:
- `images/controller/internal/managedbackend/*`
- `images/controller/internal/payloadregistry/*`
- `images/controller/internal/runtimedelivery/*`
- docs

Проверки:
- `go test ./...` в `images/controller`
- `make helm-template`

Артефакт:
- clean internal contracts для дальнейшего dual-backend rollout.

## Rollback point

После Slice 1 структура controller-а должна уже быть чище и безопаснее, даже если дальнейший live publication rollout будет остановлен.

## Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
