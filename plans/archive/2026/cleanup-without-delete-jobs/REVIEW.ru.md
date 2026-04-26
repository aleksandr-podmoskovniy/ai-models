# Review gate

## Findings

Критичных замечаний по текущему slice нет.

## Проверено

- Per-delete `Job` path удалён из `catalogcleanup`: нет build/observe
  Kubernetes Job, нет `CleanupJobName`, нет controller RBAC на `batch/jobs`.
- Delete lifecycle сохраняет idempotency через
  `ai.deckhouse.io/cleanup-completed-at` в cleanup Secret.
- Повторный reconcile после completed cleanup не повторяет cleanup operation.
- Transient cleanup error не ставит completion marker, оставляет finalizer и
  requeue через failed delete status.
- Controller Deployment получает internal OCI write auth/CA для in-process
  cleanup.
- Render guardrail запрещает `--cleanup-job-*` и `resources: ["jobs"]`.

## Validations

- `go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/adapters/k8s/cleanupstate ./internal/dataplane/artifactcleanup ./internal/bootstrap ./cmd/ai-models-controller ./internal/support/resourcenames`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Residual risks

- In-process cleanup now depends on controller env-mounted DMCR write
  credentials and CA. Render covers this, but live rollout still needs cluster
  smoke validation on model delete.
- Cleanup itself is synchronous inside reconcile. Current OCI delete has a
  short verification timeout and S3 deletes are bounded by request context, but
  a future large prefix cleanup may need explicit per-operation timeout if RGW
  stalls.
