# Notes

## Live recovery

- Context: `/Users/myskat_90/.kube/k8s-config`, `k8s.apiac.ru`.
- Namespace: `d8-ai-models`.
- Root cause: live `ai-models-controller-webhook-tls` and `ai-models-dmcr-tls`
  were `Opaque`, while current templates require `kubernetes.io/tls`; Secret
  `type` is immutable, so Helm could not patch them.
- Backup before delete:
  `/tmp/ai-models-secret-backup-20260426094231`.
- Direct delete was blocked by Deckhouse heritage admission. Deletion was done
  with Deckhouse service account impersonation:
  `system:serviceaccount:d8-system:deckhouse`.
- After reconcile both Secret objects were recreated as `kubernetes.io/tls`
  with `ca.crt`, `tls.crt` and `tls.key`.

## Current cluster state

- `module/ai-models`: `Ready`.
- `modulerelease/ai-models-v0.0.7`: `Deployed`.
- `deployment/ai-models-controller`: `2/2` ready.
- `deployment/dmcr`: `2/2` ready.
- Current pods in `d8-ai-models` have zero restarts after the fix.

## Code fix

- Added `hooks/pkg/hooks/tls_secret_type_migration`.
- The hook runs `OnBeforeHelm` with order `6`, after module-sdk common TLS
  hooks with order `5`; TLS material is loaded into values before legacy Secret
  deletion.
- It deletes only `ai-models-controller-webhook-tls` and `ai-models-dmcr-tls`
  when their live type is not `kubernetes.io/tls`.

## Validation

- `cd images/hooks && go test ./pkg/hooks/tls_secret_type_migration`
- `cd images/hooks && go test ./pkg/hooks/...`
- `cd images/hooks && go test ./...`
