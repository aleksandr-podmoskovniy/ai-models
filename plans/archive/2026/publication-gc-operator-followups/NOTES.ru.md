# NOTES: Publication / GC / RBAC operator follow-ups

## RBAC v2 ClusterModel decision

Проверен Deckhouse RBAC experimental pattern:

- `use` roles расширяются для namespaced user resources;
- пример в Deckhouse docs прямо говорит, что `RoleBinding` на `d8:use:role:manager` дает доступ к namespaced `PodLoggingConfig`, но не дает cluster-wide `ClusterLoggingConfig` / `ClusterLogDestination`;
- manage roles остаются cluster/module configuration path.

Решение для `ai-models`:

- `templates/rbacv2/use/*` оставляют только `models`;
- `templates/rbacv2/manage/*` оставляют `models` и `clustermodels`;
- legacy `user-authz` cluster roles не менялись.

## Проверки slice

Успешно:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/adapters/k8s/sourceworker`
- `cd images/controller && go test ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./internal/garbagecollection`
- `make helm-template`
- `make kubeconform`
- `git diff --check`
- `make verify`

## Render evidence

`tools/kubeconform/renders/helm-template-managed-baseline.yaml`:

- `d8:use:capability:module:ai-models:view` -> `resources: ["models"]`
- `d8:use:capability:module:ai-models:edit` -> `resources: ["models"]`
- `d8:manage:permission:module:ai-models:*` still includes `resources: ["models", "clustermodels"]`

## Review gate

- Critical findings: none.
- Retained publish pod is guarded by owner generation annotation; stale retained pod cannot satisfy a newer generation.
- Missing owner generation annotation on pre-upgrade active pod is tolerated for generation `1`; for newer generation or succeeded pod it is treated as stale.
- RBAC deny paths unchanged: no human role gets module-local `Secret`, `pods/log`, `pods/exec`, `pods/attach`, `pods/portforward`, `status`, `finalizers` or internal runtime objects.
