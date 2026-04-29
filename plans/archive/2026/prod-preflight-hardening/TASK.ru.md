# Prod preflight hardening: security and Kubernetes API conventions

## Контекст

Перед production rollout нужен ещё один системный проход по live module
surface: API/CRD, templates, RBAC, runtime entrypoints, Secret exposure,
Pod security и Kubernetes convention compliance. Предыдущий slice закрыл upload
secret URL contract; этот bundle ищет следующий слой дефектов без расширения
функциональности.

## Постановка задачи

Найти и исправить подтверждённые дефекты безопасности и Kubernetes/API
конвенций, которые могут попасть в prod rollout.

## Scope

- Public API/CRD/status naming and Secret exposure.
- ServiceAccount/RBAC minimization for module-owned runtime components.
- Pod/Job/Deployment/DaemonSet securityContext and token mounting.
- Ingress/TLS and upload/DMCR exposure safety where expressed in templates.
- Controller/runtime code paths that create public status or module-private
  Secrets related to these surfaces.

## Non-goals

- Не менять пользовательскую функциональность и runtime topology.
- Не начинать новый CLI/SDK/upload UX.
- Не менять e2e сценарии, кроме фиксации найденного coverage gap.
- Не делать wholesale cleanup ради сокращения LOC без security/API finding.

## Затрагиваемые области

- `api/`, `crds/`, `openapi/`
- `templates/`
- `images/controller/internal/*`
- `images/dmcr/internal/*`
- docs/plans only where needed to record prod evidence

## Критерии приёмки

- Нет новых публичных SecretRef/token fields в `spec/status`.
- User-facing API follows Kubernetes naming conventions and generated CRDs.
- ServiceAccount RBAC keeps least-privilege posture; no accidental human
  aggregation.
- Runtime Pods keep explicit securityContext and avoid unnecessary token mount.
- Any intentional sensitive exposure has bounded scope and documented rationale.
- Relevant tests and render/kubeconform/verify checks pass.

## RBAC coverage

- Human access levels are not changed by this bundle.
- Service-account RBAC is checked for upload gateway, controller, DMCR and
  runtime helpers.
- Sensitive deny paths remain intentional: Secret read/write for users,
  `pods/exec`, `pods/attach`, `pods/portforward`, `status`, `finalizers` and
  internal runtime objects.

## Риски

- Pre-prod changes can affect live rollout behavior. Each fix must be narrow,
  covered by tests or render evidence, and reversible.
