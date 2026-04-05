# PLAN

## Slice 1. Current-surface cleanup

Файлы:

- `images/controller/README.md`
- `images/controller/internal/publication/snapshot_test.go`
- `images/controller/internal/runtimedelivery/plan_test.go`
- `images/controller/internal/cleanupjob/job_test.go`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

Действия:

- убрать ссылки на уже удалённый `uploadsession`;
- заменить старые payload-registry-specific test URIs и wording;
- выровнять docs на текущий backend artifact direction.

Проверка:

- `go test ./internal/publication ./internal/runtimedelivery ./internal/cleanupjob`

## Slice 2. Live publication operation for HuggingFace

Файлы:

- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/artifactbackend/*`
- новый `images/controller/internal/sourcepublishjob/*`
- `images/controller/internal/app/*`
- `images/controller/cmd/ai-models-controller/run.go`

Действия:

- добавить backend publication Job materialization;
- перевести `publicationoperation` на Job lifecycle вместо immediate fail;
- держать `HTTP` и `Upload` в explicit not-implemented failure;
- декодировать durable worker result и конвертировать его в operation result.

Проверка:

- `go test ./internal/publicationoperation ./internal/modelpublish ./internal/app`

## Slice 3. Backend publish / cleanup workers

Файлы:

- новый `images/backend/scripts/ai-models-backend-artifact-publish.py`
- `images/backend/scripts/ai-models-backend-artifact-cleanup.py`
- `images/backend/scripts/ai_models_backend_runtime.py`
- `images/backend/scripts/smoke-runtime.sh`
- `images/backend/Dockerfile.local`
- `images/backend/werf.inc.yaml`

Действия:

- реализовать `HuggingFace -> tar.gz artifact -> object storage -> result`;
- включить digest/size/basic resolved profile;
- сделать live cleanup по cleanup handle;
- обновить installed entrypoints и smoke checks.

Проверка:

- `python3 -m py_compile images/backend/scripts/ai-models-backend-artifact-publish.py images/backend/scripts/ai-models-backend-artifact-cleanup.py images/backend/scripts/ai_models_backend_runtime.py`

## Slice 4. Controller template wiring

Файлы:

- `templates/controller/deployment.yaml`
- при необходимости `templates/controller/rbac.yaml`
- при необходимости `templates/_helpers.tpl`

Действия:

- прокинуть object-storage env в controller;
- обеспечить pass-through env для publication/cleanup jobs.

Проверка:

- `make helm-template`
- `make kubeconform`

## Repo checks

- `go test ./...` in `images/controller`
- `python3 -m py_compile ...`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
