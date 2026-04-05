# Plan

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication plane и platform UX.

## Orchestration

- mode: `full`
- read-only subagents before code changes:
  - virtualization backend pattern audit
  - current controller payload/mlflow drift audit
- final substantial review:
  - `review-gate`
  - `reviewer`

## Subagent findings

### Virtualization backend pattern audit

- source-first API по паттерну `dataSource`;
- controller-owned upload/auth distribution;
- namespaced и cluster scope;
- cleanup copied auth secrets после import;
- runtime consumption не должен тащить source/backend semantics в основной workload.
- storage choice `S3` vs `PVC` должен жить в module-level backend config, а не в `Model` / `ClusterModel`.
- safe first live slice после этого corrective rebase: `HuggingFace -> backend artifact plane`, затем `HTTP`, затем `Upload`.

### Current drift audit

- `payload-registry` и `managedbackend/mlflow` всё ещё прошиты в `app`, `run`, `publicationoperation`, `modelpublish`, tests и README;
- delete path всё ещё остаётся `mlflow`-shaped через `cleanuphandle` / `cleanupjob` и тянет старую phase-2 semantics;
- текущий live path нельзя дальше наращивать как canonical phase-2 architecture.
- самый опасный drift сейчас не в public `spec.source`, а в live execution path `mlflow-first` и upload/access plane `payload-registry-first`.
- в active phase-2 tree ещё остаются legacy packages, которые уже не нужны ни runtime, ни следующему implementation slice.

## Slice 1. Corrective controller/runtime rebase

Цель:

- убрать central coupling на `payload-registry` и `managedbackend publication`;
- перевести cleanup/delete contracts на backend-neutral форму;
- перевести identity/scope contracts на backend-neutral форму;
- убрать product-specific naming из current phase-2 direction;
- удалить legacy packages, которые только мешают следующему implementation slice;
- подготовить safe base для следующего live backend slice.

Файлы/каталоги:

- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/app/*`
- `images/controller/internal/publication/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `images/controller/internal/*`
- `images/controller/README.md`
- `docs/CONFIGURATION*`

Проверки:

- `go test ./...` в `images/controller`

Артефакт:

- controller runtime без payload/mlflow publication wiring;
- operation contracts и lifecycle reconciler с backend-neutral scope/identity.
- delete lifecycle с backend-neutral cleanup handle и backend cleanup entrypoint.
- legacy packages удалены из active phase-2 surface.
- publication operation больше не притворяется `mlflow` executor-ом и честно
  помечает live backend publication как следующий implementation slice.
- delete path больше не завязан на MLflow-shaped cleanup payload и использует
  generic backend cleanup handle/job boundary.

## Slice 2. Live source -> backend publication baseline

Цель:

- сделать первые реальные path для `HuggingFace` и `HTTP` в backend artifact plane;
- исправить execution contract между controller publication Job и backend worker;
- писать `status.artifact` как backend-owned saved artifact reference;
- поднимать calculated `status.resolved`.
- держать current backend storage implementation на module-owned `ObjectStorage`, не протаскивая её детали в public contract.

Файлы/каталоги:

- `images/controller/internal/*`
- `images/backend/*` только если нужен bounded importer/runtime helper
- `api/core/v1alpha1/*` при необходимости
- `docs/CONFIGURATION*`

Проверки:

- `go test ./...` в `images/controller`
- релевантные точечные проверки worker/runtime helper
- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`

Артефакт:

- working baseline `HuggingFace|HTTP -> backend artifact -> resolved -> ready`.
- bounded publication worker и durable result handoff без возврата к fat reconciler или `mlflow` semantics.
- publication Job и backend worker используют один и тот же исполняемый contract без рассинхрона по args/env.

## Slice 3. Upload/auth and runtime consumption

Цель:

- перенести virtualization-style upload/auth flow;
- подготовить agent/materializer contract для downstream runtime.

Файлы/каталоги:

- `images/controller/internal/*upload*`
- `images/controller/internal/*access*`
- `images/controller/internal/runtimedelivery/*`

Проверки:

- `go test ./...` в `images/controller`
- `make helm-template`

Артефакт:

- controller-owned upload handoff и runtime delivery contract поверх backend artifact plane.

## Rollback point

После Slice 1 controller shell и contracts уже не будут закреплять ошибочное направление `payload-registry + mlflow publication`, а legacy phase-2 пакеты уйдут из active tree.

## Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
