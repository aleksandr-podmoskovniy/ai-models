# Prod preflight hardening 2: Kubernetes/API security convention pass

## Контекст

Первый preflight slice закрыл самый явный runtime identity риск:
publication worker больше не наследует controller ClusterRole, а runtime
containers получили restricted securityContext. Перед production rollout нужен
ещё один системный проход по оставшимся Kubernetes/API convention поверхностям.

## Постановка задачи

Найти и исправить подтверждённые production-risk дефекты безопасности,
Kubernetes API conventions и template/render drift без расширения пользовательской
функциональности.

## Scope

- ServiceAccount token usage, automount behavior and least-privilege runtime
  identity.
- Pod/container securityContext, volumes, hostPath, privileged boundaries.
- RBAC wildcards, accidental human aggregation and module-local runtime access.
- Public API/CRD/status fields for Secret/token leakage and spec/status split.
- Render guardrails that prevent regression of found defects.

## Non-goals

- Не менять public UX или values knobs.
- Не менять e2e сценарии, кроме записи найденного coverage gap.
- Не переделывать node-cache CSI privileged design без отдельного storage slice.
- Не менять user-facing RBAC personas, если нет подтверждённого дефекта.

## Затрагиваемые области

- `templates/`
- `api/`, `crds/`, `openapi/`
- `images/controller/internal/*`
- `tools/helm-tests/*`
- текущий plan bundle

## Критерии приёмки

- Подтверждённые дефекты исправлены узкими patch'ами.
- Runtime ServiceAccount scopes объяснимы и не агрегируются в human roles.
- Intentional privileged/hostPath paths явно остаются только в CSI/node runtime.
- Public API не раскрывает новые Secret/token fields в `status`.
- Есть render/unit guardrail на исправленные дефекты.
- Проходят релевантные `go test`, `make helm-template`, `make kubeconform`,
  `make verify` или явно указан блокер.

## RBAC coverage

- Human roles не расширяются этим slice.
- Проверяются только module-owned ServiceAccount/RBAC и отсутствие accidental
  aggregate labels.
- Sensitive deny paths остаются без изменений: Secret read/write для
  пользователей, `pods/exec`, `pods/attach`, `pods/portforward`, `status`,
  `finalizers`, internal runtime resources.

## Риски

- Перед prod нельзя делать скрытую смену behavior. Любое исправление должно быть
  narrow, покрыто тестом/render guardrail и иметь понятный rollback.
