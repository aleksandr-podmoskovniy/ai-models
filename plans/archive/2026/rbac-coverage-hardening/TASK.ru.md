# RBAC coverage hardening

## 1. Заголовок

Дожать RBAC/user-authz/rbacv2 покрытие `ai-models` до явной матрицы по
Deckhouse pattern.

## 2. Контекст

Нужно не только помнить, что e2e должен проверить роли, а иметь
machine-checkable guardrails:

- user-authz v1 через `user-authz.deckhouse.io/access-level`;
- rbacv2 `use` / `manage`;
- явные deny paths для Secrets, pod subresources, status/finalizers и
  internal runtime objects.

Сверка с Deckhouse показала, что custom module ClusterRoles учитываются
`user-authz` только для `User`, `PrivilegedUser`, `Editor`, `Admin`,
`ClusterEditor`, `ClusterAdmin`. `SuperAdmin` — глобальная Deckhouse persona,
а не module-local custom role fragment.

## 3. Scope

- Усилить static/render checks для user-authz and rbacv2.
- Зафиксировать e2e matrix для всех effective access levels, включая global
  `SuperAdmin`.
- Не расширять права human-facing ролей.

## 4. Non-goals

- Не менять service-account RBAC controller/DMCR/upload runtime.
- Не выдавать людям доступ к Secret, pod logs/exec/attach/port-forward,
  status/finalizers или internal runtime objects.
- Не создавать module-local `SuperAdmin` fragment, потому что Deckhouse
  `user-authz` его не агрегирует как custom ClusterRole.

## 5. Acceptance Criteria

- Static check требует все six Deckhouse custom access-level fragments.
- Static check явно запрещает module-local `SuperAdmin` fragment.
- Static check проверяет rbacv2 `use` and `manage` role names, aggregate
  labels, resources and verbs.
- E2E runbook содержит can-i матрицу для six custom levels plus SuperAdmin.
- `make helm-template` and diff checks pass.

## 6. RBAC/Exposure

Задача только ужесточает coverage. Новых privileges быть не должно.
