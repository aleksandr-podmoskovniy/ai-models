## Decisions — 2026-04-24

### 1. RBAC templates отделены от service-account RBAC

Controller, DMCR и node-cache runtime ClusterRoles остаются только для service
accounts. Их нельзя aggregate-ить в человеческие роли.

### 2. Conservative `ClusterModel` write

`integration_architect` указал стандартный Deckhouse pattern, где
cluster-wide module CRD может получать write через `ClusterEditor`.

`api_designer` нашёл blocking API risk:

- `ClusterModel.spec.source.authSecretRef.namespace` допускает
  cross-namespace Secret consumption;
- `status.upload.tokenSecretRef` публичен;
- cleanup-handle сейчас хранится на публичном объекте.

Decision:

- user-facing RBAC grants are blocked until cleanup-handle and active upload
  token exposure are hardened or explicitly accepted by `api_designer`;
- первый RBAC template slice должен держать `ClusterModel` write на
  `ClusterAdmin+`;
- `ClusterEditor` получает read на `clustermodels`;
- расширение `ClusterEditor` до write возможно только после отдельного API
  hardening decision.

### 3. No module-local sensitive grants

`ai-models` не добавляет локальные grants на Secrets, pod logs, exec,
attach, port-forward, proxy или internal module services. Эти capabilities
должны оставаться глобальной Deckhouse RBAC политикой.

### 4. Hardening gate для текущего implementation slice

Read-only `api_designer` и `integration_architect` подтвердили, что RBAC
grants можно добавлять только после закрытия четырёх public-surface утечек.

Decision для реализации:

- `cleanup-handle` переносится с public `Model` / `ClusterModel` metadata в
  controller-owned internal Secret в module namespace, keyed by owner UID;
- public `status.upload.tokenSecretRef` удаляется; token handoff Secret
  остаётся internal controller/runtime state;
- `ClusterModel.spec.source.authSecretRef` запрещается целиком на phase-1,
  чтобы исключить arbitrary cross-namespace Secret consumption;
- runtime-specific condition reasons схлопываются в public reason
  `Publishing`; stage-level details остаются только в progress/message/logs;
- human RBAC templates дают доступ только к public `models` /
  `clustermodels`, без status/finalizers, Secrets, pod subresources и internal
  runtime objects.

### 5. `ClusterEditor` after hardening

После закрытия hardening blockers `ClusterModel` больше не может ссылаться на
Secret через `spec.source.authSecretRef`, а public status больше не раскрывает
upload token Secret. Поэтому legacy `ClusterEditor` получает write на public
`clustermodels` как на ограниченный cluster-wide прикладной объект, без
status/finalizers и без grants на internal runtime resources.
