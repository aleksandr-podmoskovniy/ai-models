## Access-review matrix

Цель: RBAC implementation must prove both allowed and denied paths.

### Personas

- `User`
- `PrivilegedUser`
- `Editor`
- `Admin`
- `ClusterEditor`
- `ClusterAdmin`
- `SuperAdmin`

### Allowed paths

- `User` / `PrivilegedUser`:
  - can `get,list,watch` `ai.deckhouse.io/models`;
  - can `get,list,watch` `ai.deckhouse.io/clustermodels`.
- `Editor` / `Admin`:
  - can `get,list,watch,create,update,patch,delete,deletecollection`
    `ai.deckhouse.io/models`;
  - can only `get,list,watch` `ai.deckhouse.io/clustermodels`.
- `ClusterEditor`:
  - can use namespaced `models` according to Editor semantics when bound
    cluster-wide;
  - can `get,list,watch,create,update,patch,delete,deletecollection`
    `ai.deckhouse.io/clustermodels` after hardening removed public Secret
    references from `ClusterModel`.
- `ClusterAdmin`:
  - can manage namespaced `models`;
  - can `get,list,watch,create,update,patch,delete,deletecollection`
    `ai.deckhouse.io/clustermodels`.
- `SuperAdmin`:
  - covered by global platform wildcard, not module-local wildcard.

### Denied paths

Every non-controller human role must be denied:

- `get,update,patch` on `models/status` and `clustermodels/status`;
- `update` on `models/finalizers` and `clustermodels/finalizers`;
- module-local Secret access for:
  - source auth Secrets referenced by `spec.source.authSecretRef`;
  - upload token Secrets referenced by `status.upload.tokenSecretRef`;
  - DMCR auth/TLS/CA Secrets;
  - projected runtime pull Secrets;
  - upload-session state Secrets;
- module-local pod exec/attach/port-forward/proxy grants;
- module-local `pods/log` grants;
- controller, DMCR, source-worker, cleanup-job, node-cache runtime internal
  resources unless covered by existing global Deckhouse roles outside this
  module.

### Implemented evidence

Выполненные проверки:

- `make helm-template` — passed;
- `make kubeconform` — passed;
- rendered RBAC matrix check over `tools/kubeconform/renders/*.yaml` — passed:
  60 rendered ClusterRole instances, 10 unique ai-models human-facing roles;
- `make verify` — passed.

Cluster-backed `kubectl auth can-i` не выполнялся: в локальной среде нет live
cluster/persona bindings для Deckhouse access levels. Вместо этого проверен
rendered RBAC contract: состав ролей, verbs/resources и отсутствие grants на
explicit denied paths.
