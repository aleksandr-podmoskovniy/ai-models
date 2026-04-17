## MLflow surface inventory at reset start

На момент старта reset workstream `MLflow`/historical backend surface ещё живёт
в следующих местах:

### 1. Runtime shell

- `templates/backend/*`
- `images/backend/*`
- backend-related build and smoke targets in `Makefile`

### 2. Config and docs

- `openapi/config-values.yaml`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- historical mentions in `README*`, `docs/README*`, `docs/development/*`

### 3. Tooling and tests

- `tools/upload_hf_model.py`
- `tools/run_hf_import_job.sh`
- `tools/run_model_cleanup_job.sh`
- `tools/helm-tests/validate-renders.py`

### 4. Current reset stance

- `MLflow` is no longer treated as a live architectural center;
- slice 1 rewrites narrative and repo baseline around controller-owned
  publication/runtime flow;
- later slices must delete the actual runtime shell and stale render/test
  assumptions, not keep them as hidden fallback.

## Render-shell cut landed

В текущем execution slice уже удалены:

- `templates/backend/*`;
- `templates/auth/dex-client.yaml`;
- `templates/module/backend-*`;
- user-facing `auth` contract и backend-only `artifacts.pathPrefix`;
- second managed-postgres auth database;
- backend-oriented helm render checks.

## Backend-shell deletion landed

Следующим slice уже удалены:

- `images/backend/*`;
- backend build/smoke targets в `Makefile`;
- legacy import/cleanup tools, которые обслуживали historical backend shell.

Итоговая живая форма repo после этого cut:

- retired backend/auth shell больше не рендерится и не собирается;
- legacy import helpers больше не висят в repo как скрытый fallback;
- оставшийся reset scope теперь смещён с удаления shell на native publisher
  cutover и финальное сжатие residual docs/tooling.

## Validation-shell cleanup landed

После удаления backend shell вычищен и residual validation drift:

- `tools/kubeconform/kubeconform.sh` больше не знает про `DexClient` как про
  живой schema-skip;
- `tools/helm-tests/validate-renders.py` больше не использует `DexClient` как
  legacy render marker;
- render fixtures renamed away from `managed-sso-*` wording, потому что live
  сценарий здесь уже не про historical SSO/backend shell, а про generic module
  baseline и discovered Dex CA trust input.

## PostgreSQL shell deletion landed

PostgreSQL больше не рассматривается как часть live module contract:

- user-facing `aiModels.postgresql` removed from OpenAPI and module defaults;
- `templates/database/*` deleted;
- render fixtures and validation no longer exercise or tolerate `Postgres` /
  `PostgresClass`;
- docs and repo-layout guidance no longer describe managed-postgres as part of
  the ai-models baseline.

## Native publisher cutover contract

Следующий execution slice фиксируется как bounded replacement, а не как
финальная stream architecture:

- вход native publisher остаётся текущим `checkpointDir` из publication worker;
- `KitOps` binary и shell удаляются полностью;
- ai-models-owned publisher сам пишет registry blobs и manifest по live OCI
  contract, который уже читает `internal/adapters/modelpack/oci`;
- первый cut публикует один weight layer tar под contract path `model/`;
- materializer, inspect и runtime delivery shape не меняются в этом slice.

## Native publisher cutover landed

Текущий live publication path больше не зависит от `KitOps`:

- `internal/adapters/modelpack/oci` теперь владеет controller-side
  publish/remove плюс consumer-side inspect/materialize;
- publish path пишет registry blobs и OCI manifest напрямую по HTTP без
  external binary;
- первый cut публикует один weight-layer tar rooted at `model/`, что уже
  доказано round-trip тестом `Publish -> Materialize -> Remove`;
- worst-case byte path в landed cut всё ещё materialized, не streaming:
  `checkpointDir` plus один full tar рядом с ним на том же bounded worker
  volume/PVC, затем published blob в registry;
- `images/controller/werf.inc.yaml` больше не тащит отдельный publisher
  artifact stage, а `images/controller/kitops.lock` и
  `images/controller/install-kitops.sh` удалены;
- `internal/adapters/modelpack/kitops/*` удалён из live repo.

## Streaming publisher follow-up

Следующий bounded slice на том же native publisher path:

- не меняет OCI artifact contract и не трогает consumer-side materializer;
- заменяет temp tar file на streaming layer upload через OCI blob upload
  protocol;
- целевой worst-case local copy count:
  только `checkpointDir` на bounded worker volume/PVC плюс streamed network
  bytes, без второй full-size tar-копии на диске.

## Streaming publisher landed

Native publisher теперь stream'ит weight layer напрямую в registry upload flow:

- layer bytes идут `tar writer -> PATCH upload -> PUT finalize`;
- config blob и manifest по-прежнему остаются small in-memory payloads;
- local worst-case copy count сократился до одного full-size `checkpointDir`
  на bounded worker volume/PVC;
- end-to-end round-trip test теперь явно доказывает не только
  `Publish -> Materialize -> Remove`, но и сам streaming PATCH path.
