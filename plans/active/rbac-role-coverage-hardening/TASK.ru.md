### 1. Заголовок

RBAC coverage parity для `Model` / `ClusterModel`

### 2. Контекст

В модуле уже есть legacy `user-authz` fragments и `rbacv2` templates, но их
нужно ещё раз проверить против Deckhouse/virtualization паттернов: role
fragments должны быть минимальными, human-facing роли не должны получать
служебные runtime ресурсы, а `ClusterModel` должен оставаться cluster-persona
путём, а не namespaced `use` доступом.

### 3. Постановка задачи

Проверить и поправить RBAC для первой и второй модели доступа:

- legacy `user-authz.deckhouse.io/access-level`;
- `rbacv2` `use` / `manage` роли и aggregate labels.

### 4. Scope

- `templates/user-authz-cluster-roles.yaml`;
- `templates/rbacv2/**`;
- render validation guardrails;
- краткая документация RBAC coverage.

### 5. Non-goals

- Не менять публичный `Model` / `ClusterModel` API.
- Не выдавать людям module-local доступ к Secret, `pods/log`, `pods/exec`,
  `pods/attach`, `pods/portforward`, `status`, `finalizers` или internal runtime
  objects.
- Не менять service-account RBAC, если он не aggregate-ится в человеческие
  роли.

### 6. Затрагиваемые области

- Helm RBAC templates.
- Helm render validation.
- `docs/CONFIGURATION*.md`.
- Current active plan notes.

### 7. Критерии приёмки

- Legacy `user-authz` покрывает `User`, `PrivilegedUser`, `Editor`, `Admin`,
  `ClusterEditor`, `ClusterAdmin`.
- `User` получает только read-only `models` / `clustermodels`.
- `Editor` добавляет write verbs только для namespaced `models`.
- `ClusterEditor` добавляет write verbs только для cluster-wide
  `clustermodels`.
- `PrivilegedUser`, `Admin`, `ClusterAdmin` не получают лишних ai-models
  прав, если в модуле нет соответствующих безопасных дополнительных ресурсов.
- `rbacv2/use` покрывает только namespaced `models`.
- `rbacv2/manage` покрывает `models`, `clustermodels` и `ModuleConfig`
  `ai-models`.
- Human-facing RBAC не содержит `status`, `finalizers`, Secret, pod logs,
  exec/attach/port-forward или internal runtime resources.
- Render validation фиксирует эти правила.

### 8. Риски

- Можно случайно перепутать human-facing RBAC с controller/service-account
  RBAC. Проверка должна смотреть только user-facing templates.
- `ClusterModel` нельзя добавлять в `rbacv2/use`, потому что namespaced
  RoleBinding не является корректным путём для cluster-scoped ресурса.
