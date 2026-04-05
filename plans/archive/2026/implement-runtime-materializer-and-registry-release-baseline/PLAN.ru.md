# PLAN

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication, runtime delivery to
`ai-inference`.

## Режим orchestration

- mode: `full`
- read-only inputs before code changes:
  - current repo audit of runtime/materializer gaps;
  - external runtime delivery references (`ModelPack` implementations such as
    `KitOps` and `Modctl`, KServe patterns, `DVCR` upload/auth/copy
    discipline);
  - read-only subagents:
    - runtime seam audit;
    - legacy drift audit.

## Архитектурные acceptance criteria

- use case / port / adapter split for any new controller runtime code;
- no new fat reconciler growth;
- non-test controller files obey current LOC / complexity gates;
- runtime materialization internals stay behind public `Model` /
  `ClusterModel` contract;
- runtime/materializer code sees only `OCI from registry`; backend storage
  details hidden under `DVCR` stay out of runtime/public contracts;
- concrete `ModelPack` tooling stays behind ports/adapters and must be
  replaceable without changing public or domain contracts;
- `openapi/config-values.yaml` must stay stable and short like in
  virtualization; runtime/materializer adapter details belong only to internal
  values, templates/helpers, and code;
- tests include branch/state evidence for new runtime decision logic.

## Slice 1. OCI-only contract cleanup and legacy drift removal

Цель:

- вычистить old artifact/public contract drift before adding materializer.

Изменения:

- `api/core/v1alpha1/*`
- `crds/*`
- `images/controller/internal/publication/*`
- docs wording that still advertises old backend-neutral artifact shape

Результат:

- API and internal publication snapshot align with the current OCI-first live
  path;
- stale `ObjectStorage` path is removed or isolated from the current release
  baseline.

Проверки:

- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `go test ./...` in `api`
- targeted `go test ./...` in `images/controller`

## Slice 2. Runtime materialization contract

Цель:

- ввести explicit runtime/materializer contract for init-container based model
  handoff.

Изменения:

- `images/controller/internal/domain/*`
- `images/controller/internal/application/*`
- `images/controller/internal/ports/*`
- runtime contract docs

Результат:

- explicit immutable artifact -> local path contract;
- clear separation between publication and runtime materialization;
- virtualization-style invariant: registry target is the only runtime input,
  regardless of storage backend hidden under the registry.

Проверки:

- targeted controller tests
- controller quality gates

## Slice 3. v0 `ModelPack` init-adapter wiring

Цель:

- добавить v0 runtime wiring for init-container based local materialization.

Изменения:

- controller/runtime adapter code:
  - adapter-side init renderer only;
  - no workload mutation yet;
- templates / values / OpenAPI
- docs

Результат:

- module/runtime wiring for the current upstream init adapter using shared
  volume/PVC and immutable digest refs, without freezing the contract to a
  single `ModelPack` implementation brand.

Проверки:

- targeted controller tests
- `make helm-template`
- `make kubeconform`

## Slice 4. Runtime-specific release hygiene

Цель:

- закрыть только runtime-specific hygiene вокруг current materialization path.

Изменения:

- runtime-specific docs and leftover seams around replaced materialization
  assumptions
- active bundle sync for this runtime workstream only
- generic cleanup/archive ownership stays in
  `reconcile-chat-decisions-and-cleanup-phase2-runtime`

Результат:

- repo is cleaner than before and the release path is explicit.

Проверки:

- `make verify`
- `git diff --check`

## Rollback point

После Slice 1 репозиторий уже должен быть чище и согласованнее по current
publication contract even if runtime materializer slices stop there.

## Final validation

- controller quality gates
- `go test ./...` in `api`
- `go test ./...` in `images/controller`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
