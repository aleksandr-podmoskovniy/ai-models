# REVIEW: move-backend-db-upgrade-wrapper-into-image

## Scope

- runtime DB init/upgrade wrapper under `images/backend`
- `werf` and local Docker runtime image wiring
- backend ConfigMap simplification

## Findings

- Критичных замечаний к текущему slice нет.

## Validation

- `python3 -m py_compile images/backend/scripts/ai-models-backend-db-upgrade.py`
- `make helm-template`
- `make verify`

## Residual risks

- cluster-side retry после нового image/bundle deploy ещё не подтверждён;
- wrapper опирается на upstream-internal `mlflow.store.db.utils` contract и при
  future rebase его надо перепроверять.
