## Review

### Scope result

- Public hardening blockers закрыты до выдачи human-facing RBAC:
  `status.upload.tokenSecretRef` удалён из публичного status, cleanup handle
  перенесён во внутренний controller-owned Secret, `ClusterModel` больше не
  поддерживает `spec.source.authSecretRef`, runtime-stage condition reasons
  сведены к стабильному `Publishing`.
- Legacy `user-authz` roles добавлены по Deckhouse pattern
  `user-authz.deckhouse.io/access-level`.
- `ClusterEditor` получает write на public `clustermodels` после hardening,
  без status/finalizers и без sensitive/internal grants.
- `rbacv2/use` и `rbacv2/manage` roles добавлены с aggregate labels.
- Human-facing roles не grant-ят `Secret`, `pods/log`, `pods/exec`,
  `pods/attach`, `pods/portforward`, `pods/proxy`, `services/proxy`,
  `status`, `finalizers` или internal controller/DMCR/node-cache runtime
  resources.

### Validation

- `bash api/scripts/update-codegen.sh` — passed.
- `bash api/scripts/verify-crdgen.sh` — passed.
- `go test ./core/...` in `api` — passed.
- `go test ./internal/adapters/k8s/cleanupstate ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/sourceworker ./internal/domain/publishstate ./internal/application/publishobserve ./internal/controllers/catalogstatus ./internal/controllers/catalogcleanup ./internal/support/modelobject` in `images/controller` — passed.
- `go test ./internal/garbagecollection` in `images/dmcr` — passed.
- `make helm-template` — passed.
- `make kubeconform` — passed.
- Rendered RBAC matrix check over `tools/kubeconform/renders/*.yaml` — passed:
  60 rendered ClusterRole instances, 10 unique ai-models human-facing roles.
- `make test` — passed.
- `make verify` — passed.

### Residual risks

- Cluster-backed `kubectl auth can-i` не выполнялся из-за отсутствия live
  Deckhouse cluster/persona bindings в локальной среде; проверка выполнена на
  rendered RBAC contract.
- `cleanuphandle` helper остаётся как internal codec для controller/runtime
  state and tests, но production path больше не пишет backend cleanup handle в
  public `Model` / `ClusterModel` metadata.
- Public upload token discovery теперь намеренно отсутствует; будущий
  credential issuance UX должен быть отдельным API/RBAC решением, а не
  возвратом Secret reference в status.

### Final reviewer follow-up

Финальный read-only `reviewer` не нашёл implementation findings. Найденный
bundle drift по `ClusterEditor`/`ClusterModel` matrix исправлен в
`PLAN.ru.md` и `NOTES.ru.md`.
