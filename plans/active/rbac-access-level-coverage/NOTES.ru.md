## Deckhouse reference

Source repo:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
- revision: `f13d67cf51ecd6127a7bdd03c25e1ff7060c2da2`

Legacy accessLevel модель:

- `modules/140-user-authz/templates/cluster-roles.yaml` задаёт базовые роли:
  `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`,
  `ClusterAdmin`, `SuperAdmin`.
- `modules/*/templates/user-authz-cluster-roles.yaml` добавляют module-local
  fragments с annotations `user-authz.deckhouse.io/access-level`.
- `modules/140-user-authz/templates/cluster-role-bindings.yaml` bind-ит
  `AuthorizationRule` / `ClusterAuthorizationRule` к базовой роли и custom
  module fragments для выбранного access level.

rbacv2 модель:

- `templates/rbacv2/use/view.yaml` и `use/edit.yaml` описывают прикладное
  использование module capabilities.
- `templates/rbacv2/manage/view.yaml` и `manage/edit.yaml` описывают
  управление module/subsystem.
- Aggregation идёт через labels:
  - `rbac.deckhouse.io/kind: use|manage`;
  - `rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer|manager|admin`;
  - `rbac.deckhouse.io/aggregate-to-<subsystem>-as: viewer|manager`;
  - `rbac.deckhouse.io/level: module`;
  - `rbac.deckhouse.io/namespace: <module namespace>` для manage permissions.

## ai-models initial state before this slice

- Есть только service-account RBAC:
  - controller;
  - DMCR;
  - node-cache runtime.
- Не было user-facing RBAC fragments для `models` / `clustermodels`.
- Controller ClusterRole очень широкий и должен остаться SA-only.

## Implemented post-hardening RBAC matrix

| Access level | `models` | `clustermodels` |
|---|---|---|
| `User` | `get,list,watch` | `get,list,watch` |
| `PrivilegedUser` | `get,list,watch` | `get,list,watch` |
| `Editor` | `get,list,watch,create,update,patch,delete,deletecollection` | `get,list,watch` |
| `Admin` | same as `Editor` | `get,list,watch` |
| `ClusterEditor` | same as `Editor` for namespaced models | `get,list,watch,create,update,patch,delete,deletecollection` |
| `ClusterAdmin` | same as `Editor` for namespaced models | `get,list,watch,create,update,patch,delete,deletecollection` |
| `SuperAdmin` | global wildcard | global wildcard |

Subresources:

- end-user roles: no verbs on `models/status`, `clustermodels/status`,
  `models/finalizers`, `clustermodels/finalizers`;
- controller SA only: status/finalizer verbs.

## API hardening blockers

- `ai.deckhouse.io/cleanup-handle` annotation exposes backend artifact and
  upload-staging details to anyone who can read public resources.
- `ClusterModel.spec.source.authSecretRef.namespace` enables cross-namespace
  Secret consumption and blocks safe `ClusterEditor` write.
- `status.upload.tokenSecretRef` is public status; with Deckhouse default
  Secret grants, `PrivilegedUser` can read the active upload token Secret.
- Some condition reasons expose internal publication stages and should be
  reviewed separately.

These are blocking for shipping broad user-facing RBAC grants. RBAC template
implementation must start only after the blocking public-surface issues are
closed or explicitly accepted by `api_designer`.

Implemented hardening result:

- cleanup handle перенесён во внутренний controller-owned Secret;
- public `status.upload.tokenSecretRef` удалён;
- `ClusterModel.spec.source.authSecretRef` запрещён;
- runtime-stage condition reasons сведены к stable public reason `Publishing`.

## Deny paths

Do not add module-local user grants for:

- Secrets;
- pods/log;
- pods/exec;
- pods/attach;
- pods/portforward;
- services/ingresses of module internals;
- controller/DMCR/node-cache runtime resources;
- upload-session state or projected registry credentials;
- status/finalizers.
