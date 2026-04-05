# Plan

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication plane и platform UX.

## Orchestration

- mode: `full`
- read-only subagents before code changes:
  - virtualization upload/session mapping audit
  - current ai-models controller/API gap audit for `Upload`
- final substantial review:
  - `review-gate`
  - `reviewer`

## Subagent conclusions

- smallest safe mapping from virtualization: owner-owned `Pod + Service + auth
  Secret`, deterministic naming, `WaitForUpload` only after session becomes
  runnable, and no return to batch import semantics;
- `status.upload` должен оставаться user helper поверх stable
  phase/conditions, а не протаскивать raw backend internals;
- минимальный live scope на текущем backend plane: `Upload` только для
  `expectedFormat=HuggingFaceDirectory`, а `ModelKit` — explicit controlled
  failure до `KitOps`/OCI slice.

## Slice 1. Upload Session Supplements And Status

Цель:

- завести controller-owned upload session boundary;
- научить `Upload` переходить в `WaitForUpload`;
- дать working `status.upload.command`.

Файлы/каталоги:

- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `images/controller/internal/uploadsession/*`
- `images/controller/internal/app/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/controller/*`
- `images/controller/README.md`
- `docs/CONFIGURATION*`

Проверки:

- `go test ./internal/uploadsession ./internal/publicationoperation ./internal/modelpublish ./internal/app` в `images/controller`

Артефакт:

- upload session resources owned by publication operation;
- `phase=WaitForUpload`, `UploadReady=True`, `status.upload.command`;
- no return to batch execution.

## Slice 2. Live Upload To Current Artifact Plane

Цель:

- сделать upload server runtime;
- довести `expectedFormat=HuggingFaceDirectory` до current object-storage-backed
  publication result;
- `ModelKit` оставить explicit controlled failure.

Файлы/каталоги:

- `images/backend/scripts/*`
- `images/backend/Dockerfile.local`
- `images/backend/werf.inc.yaml`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/uploadsession/*`
- docs / active bundle

Проверки:

- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py`
- `python3 -m py_compile images/backend/scripts/ai-models-backend-upload-session.py`
- `go test ./...` в `images/controller`

Артефакт:

- live upload session for `HuggingFaceDirectory`;
- explicit failure for `ModelKit` until `KitOps`/OCI slice.

## Rollback point

После Slice 1 public contract для `Upload` уже становится честным и controller
начинает владеть session resources, даже если final publication after upload ещё
не закрыт.

## Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Execution status

- Slice 1 completed.
- Slice 2 completed for `expectedFormat=HuggingFaceDirectory`.
- `ModelKit` remains intentionally blocked for a later `KitOps`/OCI slice.
