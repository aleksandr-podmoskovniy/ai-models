# REVIEW

## Findings

- Блокирующих замечаний по самому HTTP live slice не найдено.

## Validation

Успешно:

- `go test ./...` in `images/controller`
- `python3 -m py_compile images/backend/scripts/ai-models-backend-hf-import.py`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `git diff --check`

С ограничением:

- `make verify`
  - падает на existing backend render assertions внутри `tools/helm-tests/validate-renders.py`:
    - `helm-template-managed-sso-baseline.yaml: rendered output must include the internal ai-models backend crypto Secret`
    - `helm-template-managed-sso-baseline.yaml: rendered output must include the internal ai-models backend auth Secret`
    - `helm-template-managed-sso-baseline.yaml: backend auth Secret must expose machineUsername and machinePassword`
    - `helm-template-managed-sso-baseline.yaml: rendered output must include a DexClient for browser SSO`
  - это не выглядело связанным с текущим HTTP publication slice, потому что
    отдельные `make helm-template` и `make kubeconform` прошли, а changed areas
    этого slice не трогают backend SSO secret rendering.

Не запускалось:

- `python3 images/backend/scripts/ai-models-backend-hf-import.py --help`
  - локальный environment не содержит `mlflow`, поэтому для syntax smoke был
    использован `py_compile`, а runtime smoke покрыт через repo wiring
    (`smoke-runtime.sh`) и image install files.

## Residual risks

- HTTP path сейчас intentionally narrow:
  - source должен быть archive-based;
  - после распаковки должен получаться Hugging Face-compatible checkpoint
    directory.
- `http.authSecretRef` поддерживает только минимальный worker-side contract:
  - `authorization`, или
  - `username` + `password`.
- `Upload` по virtualization-pattern всё ещё отдельный future slice.
- Runtime materializer / agent для `status.artifact -> local PVC path` ещё не
  реализован.
