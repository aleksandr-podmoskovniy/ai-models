# REVIEW: debug-live-cluster-startup

## Scope

- live cluster diagnostics для `d8-ai-models`
- fix init/runtime/security path в `templates/backend/configmap.yaml`
- helper wiring в `templates/_helpers.tpl`
- semantic guard в `tools/helm-tests/validate-renders.py`

## Findings

- Критичных замечаний к текущему fix нет.

## Validation

- `make helm-template`
- `make verify`

## Residual risks

- cluster-side retry после нового deploy ещё не подтверждён; следующий реальный
  сигнал надо снимать уже по новым логам `backend`;
- fix опирается на upstream-internal `mlflow.store.db.utils._safe_initialize_tables`
  и `_upgrade_db`; при смене major/minor backend release этот contract надо
  перепроверять при rebase.
