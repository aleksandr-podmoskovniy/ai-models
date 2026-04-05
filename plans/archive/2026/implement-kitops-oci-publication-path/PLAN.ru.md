# PLAN

## Current phase

Этап 2. `Model` / `ClusterModel`, controller publication plane и platform UX.

## Orchestration

- mode: `full`
- read-only subagents before code changes:
  - KitOps/runtime integration audit for backend image and CLI flow
  - controller/API/config audit for OCI publication boundary
- final substantial review:
  - `review-gate`
  - reviewer-style read-only pass

## Audit conclusions

- `KitOps` CLI surface для текущего slice подтверждён как
  `kit login -> kit init -> kit pack -> kit push -> kit inspect --remote`.
- Для стабильного machine parsing нельзя оставлять update notifications в
  default config path; worker должен использовать isolated `kit --config` root
  и один раз выключать notifications через `kit version --show-update-notifications=false`.
- `Upload(ModelKit)` пока нельзя честно обещать как live path: текущий `kit`
  CLI умеет `push` из local storage tag/digest, но не даёт clean flow для
  controller-owned upload session с arbitrary uploaded archive file. Поэтому
  этот формат остаётся explicit controlled failure до отдельного ingest slice.

## Slice 1. Module Config And Controller OCI Wiring

Цель:

- завести stable module config for publication OCI registry;
- перевести controller worker destination wiring с S3 root на OCI repo prefix.

Файлы/каталоги:

- `openapi/*`
- `fixtures/module-values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `templates/module/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/app/*`
- `images/controller/internal/artifactbackend/*`
- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/uploadsession/*`

Проверки:

- `go test ./internal/artifactbackend ./internal/sourcepublishpod ./internal/uploadsession ./internal/app` в `images/controller`
- `make helm-template`

Артефакт:

- controller worker pods build OCI destination refs and receive registry auth
  Secret / optional CA wiring from module config.

## Slice 2. Backend KitOps Pack Push Inspect Path

Цель:

- установить pinned `kit` CLI в backend runtime image;
- заменить current object-storage publication flow на `kit init/pack/push/inspect`.

Файлы/каталоги:

- `images/backend/Dockerfile.local`
- `images/backend/werf.inc.yaml`
- `images/backend/scripts/ai-models-backend-source-publish.py`
- `images/backend/scripts/ai-models-backend-upload-session.py`
- `images/backend/scripts/smoke-runtime.sh`
- backend tests if needed

Проверки:

- `python3 -m py_compile images/backend/scripts/ai-models-backend-source-publish.py images/backend/scripts/ai-models-backend-upload-session.py`
- `python3 -m unittest discover -s images/backend/scripts -p 'test_*.py'`

Артефакт:

- worker result payloads use `artifact.kind=OCI`;
- `HuggingFace`, `HTTP`, and `Upload(HuggingFaceDirectory)` publish through
  KitOps/OCI.

## Slice 3. Controller Result Projection, Docs, Final Validation

Цель:

- выровнять tests and docs with the new live OCI baseline;
- прогнать repo-level validations.

Файлы/каталоги:

- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `docs/CONFIGURATION*`
- `images/controller/README.md`
- `plans/active/implement-kitops-oci-publication-path/*`

Проверки:

- `go test ./...` в `images/controller`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

Артефакт:

- current live publication baseline is OCI-backed in code, tests, and docs.

## Rollback point

После Slice 1 controller and module config уже будут готовы к OCI publication,
но object-storage backend path ещё не будет выкинут из backend runtime. Это
последняя безопасная точка перед full execution switch.
