## 1. Current phase

Это сознательный phase reset.

Текущие repo docs и shell всё ещё отражают старый этап:

- internal managed backend around `MLflow`;
- phase-2 catalog layered on top.

Запрос пользователя переводит baseline к новому центру:

- ai-models-owned publication/runtime architecture;
- no `MLflow` live surface;
- no fallback `KitOps` publisher.

Значит в рамках этого workstream нужно не “подправить phase-1”, а переписать
phase narrative и runtime shell под новый reality.

## 2. Orchestration

Overall workstream: `full`

Почему:

- задача затрагивает module layout, build shell, values/OpenAPI, docs, runtime,
  tooling и product baseline;
- это не один refactor, а coordinated removal plus replacement;
- forward-only migration без fallback требует особенно аккуратной slicing
  discipline.

Execution mode для уже landed slices:

- Slice 1, render/OpenAPI contraction и landed backend-shell deletion slice
  допустимы в `solo`, потому что это bounded removal of stale surfaces без
  нового runtime/API design choice;
- перед native publisher cutover workstream возвращается в `full`.

Read-only reviews, которые должны быть закрыты до execution slices:

- `repo_architect`
  - проверить новый module shape после удаления `images/backend` /
    `templates/backend`;
- `integration_architect`
  - проверить, что storage/auth/build/runtime contracts после вырезания
    `MLflow` не распадаются на ad-hoc wiring;
- `api_designer`
  - проверить, что reset не тащит backend/runtime internals в public model API;
- `backend_integrator`
  - проверить, какие backend-specific surfaces реально ещё живые и что должно
    уйти без остатка.

## 3. Slices

### Slice 1. Freeze the new baseline and remove narrative ambiguity

Цель:

- зафиксировать, что repo больше не строится вокруг `MLflow` backend;
- обновить phase docs, чтобы дальнейшие code slices не противоречили repo-local
  правилам и README narrative.

Файлы/каталоги:

- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/development/TZ.ru.md`
- `docs/development/PHASES.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- `rg -n "MLflow|mlflow" README.md README.ru.md docs docs/development`

Артефакт результата:

- docs baseline, который больше не утверждает, что `MLflow` — live center of
  the module.

### Slice 2. Define ai-models-owned `ModelPack` publisher contract

Цель:

- заменить brand-specific `KitOps` contract на ai-models-owned publisher
  boundary.

Нужно зафиксировать:

- first native cutover shape:
  worker-local checkpoint directory -> single controller-owned OCI artifact;
- OCI artifact layout and digest rules;
- manifest/config/blob ownership;
- publication inputs from source mirror / upload staging;
- what remains internal and what is observable in status.

Файлы/каталоги:

- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/modelpack/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/ports/modelpack ./internal/adapters/modelpack/...`

Артефакт результата:

- explicit native publisher design plus bounded implementation target:
  single tar weight layer under `model/`, live-materializer-compatible
  manifest/config shape, binary-free registry push/remove contract.

### Slice 3. Land native publisher and cut over publication path

Цель:

- сделать ai-models-owned publisher live;
- в том же slice удалить `KitOps` live usage, а не оставлять fallback.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/publicationartifact/*`
- `images/controller/werf.inc.yaml`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/... ./internal/dataplane/publishworker/... ./cmd/ai-models-artifact-runtime`
- `cd images/controller && go test ./internal/adapters/modelpack/oci`

Артефакт результата:

- publication path, который больше не вызывает `KitOps`, а runtime image больше
  не тащит `kitops.lock` / `install-kitops.sh` / external publisher binary.

### Slice 4. Delete `MLflow` backend runtime shell

Цель:

- убрать `images/backend/*`, `templates/backend/*` and related scripts/tests as
  live repo surface.

Already landed in this slice:

- удалены `templates/backend/*`, `templates/auth/dex-client.yaml` и
  `templates/module/backend-*`;
- убран backend-only `openapi` contract (`auth`, `artifacts.pathPrefix`,
  internal `backend`/`auth`);
- вычищены legacy render checks и второй managed-postgres auth DB.

Current landing for the same slice:

- удалить `images/backend/*`, backend build/smoke targets в `Makefile` и legacy
  import/cleanup tools;
- синхронизировать docs/module narrative с тем, что historical backend shell
  уже реально удалён из live repo.

Файлы/каталоги:

- `images/backend/*`
- `templates/backend/*`
- `tools/*`
- `tools/helm-tests/*`
- CI / build files

Проверки:

- `rg -n "images/backend|run_hf_import_job|upload_hf_model|run_model_cleanup_job|libai_models_job|backend-build|backend-shell-check|backend-oidc-auth" . -g '!plans/archive/**'`
- `rg -n "MLflow|mlflow" README.md README.ru.md docs docs/development Makefile module.yaml -g '!plans/archive/**'`
- `make helm-template`
- `make kubeconform`

Артефакт результата:

- repo without live backend runtime shell or legacy import/build helpers.

### Slice 5. Delete retired PostgreSQL metadata shell

Цель:

- убрать из live repo оставшийся metadata-database shell, который был нужен
  только historical backend baseline.

Файлы/каталоги:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/database/*`
- `templates/_helpers.tpl`
- `fixtures/module-values.yaml`
- `fixtures/render/*`
- `tools/helm-tests/*`
- `tools/kubeconform/*`
- `docs/CONFIGURATION*`
- `DEVELOPMENT.md`
- `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- `python3 -m py_compile tools/helm-tests/validate-renders.py`
- `make helm-template`
- `make kubeconform`
- `rg -n "postgres|Postgres|postgresql|managed-postgres|postgresClass" README.md README.ru.md DEVELOPMENT.md docs openapi templates fixtures tools .github/workflows module.yaml -g '!plans/archive/**'`
- `make verify`

Артефакт результата:

- repo without live PostgreSQL config/template/render shell.

### Slice 6. Finish repo contraction and proof

Цель:

- добить остаточные historical references и убедиться, что repo shape уже
  согласован после удаления backend shell и retired PostgreSQL shell.

Файлы/каталоги:

- all touched live surfaces

Проверки:

- `make lint-docs`
- `make helm-template`
- `make kubeconform`
- `make verify`

Артефакт результата:

- coherent repo baseline with no live backend shell and no stale build/docs
  claims about it.

### Slice 7. Stream the native publisher layer upload

Цель:

- убрать последнюю full-size локальную tar-копию из native publisher path;
- сохранить уже landed OCI manifest/config/materializer contract unchanged.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/... ./cmd/ai-models-artifact-runtime`
- `make verify`

Артефакт результата:

- layer bytes stream directly from `checkpointDir` tar writer into registry blob
  upload protocol;
- worst-case local full-size copies shrink from `checkpointDir + temp tar` to
  just `checkpointDir` on the bounded worker volume/PVC.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: новый narrative уже зафиксирован,
но runtime still unchanged.

После Slice 2 можно безопасно остановиться: native publisher contract defined,
но old runtime still present.

После Slice 3 rollback уже становится дорогим, поэтому Slice 3 is the first
true cutover point: it must land with working publication and without fallback.

## 5. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
