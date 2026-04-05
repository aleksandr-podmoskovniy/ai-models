# Plan

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication plane и platform UX.

## Orchestration

- mode: `full`
- read-only subagents before code changes:
  - virtualization pod/session pattern audit
  - current controller job-to-pod refactor risk audit
  - kitops/oci direction audit from existing repo context
- final substantial review:
  - `review-gate`
  - `reviewer`

## Subagent conclusions

- virtualization pattern подтверждён как правильный ориентир: worker lifecycle
  должен жить за service boundary, worker resource должен иметь controller
  owner, а `Upload` нельзя тащить как ещё один bare batch worker;
- smallest-safe corrective slice: сделать `HuggingFace` единственным live source
  на pod-based path, `HTTP` и `Upload` оставить controlled failure до следующих
  safe slices, вычистить legacy `Job` path и перевести docs/RBAC на pod
  semantics;
- текущий public contract можно оставить neutral:
  `status.artifact.kind=OCI|ObjectStorage`, а OCI/KitOps hooks в API не
  выбрасывать в этом corrective slice.

## Slice 1. Rebase Execution Primitive

Цель:

- убрать batch `Job` как основной publication primitive;
- перевести current live `HuggingFace` execution на worker `Pod`;
- выключить unsafe live `HTTP` path;
- сделать controller wording и runtime shell честными относительно нового
  направления.

Файлы/каталоги:

- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/app/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `images/controller/internal/sourcepublish*`
- `images/backend/scripts/ai-models-backend-source-publish.py`
- `templates/controller/*`
- `images/controller/README.md`
- `docs/CONFIGURATION*`

Проверки:

- `go test ./...` в `images/controller`
- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`

Артефакт:

- live `HuggingFace` path через worker `Pod`, а не `Job`;
- `HTTP` переведён в controlled failure до safe reimplementation;
- active docs больше не закрепляют batch jobs как canonical phase-2 direction;
- worker lifecycle вынесен из reconciler в `sourcepublishpod.Service`;
- publication worker `Pod` получает controller owner на operation `ConfigMap`,
  а `publicationoperation` controller просыпается по owner `Pod` events.

## Slice 2. Upload Session And OCI Direction

Цель:

- спроектировать и начать внедрение controller-owned upload session;
- ввести internal contracts под `KitOps` packaging и OCI artifact refs.

Файлы/каталоги:

- `images/controller/internal/*`
- `images/backend/scripts/*`
- `api/core/v1alpha1/*` при необходимости
- docs / active bundle

Проверки:

- узкие unit tests по новым contracts
- `make helm-template`

Артефакт:

- следующий safe base под `Upload session + KitOps/OCI` без возврата к batch
  publication semantics.

## Rollback point

После Slice 1 live publication уже не будет строиться вокруг `Job`-based
execution, а unsafe `HTTP` path будет отключён без затрагивания public object
ownership.

## Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Execution status

- Slice 1 completed.
- Slice 2 not started in code in this task bundle.
