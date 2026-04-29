# Prod preflight: service account token mount hardening

## Контекст

Продолжаем production preflight по Kubernetes security/API conventions. После
RBAC split нужно убрать неявное поведение Kubernetes default для
`automountServiceAccountToken`: сейчас часть pod templates получает token mount
только потому, что default равен `true`. Для prod это плохой контракт: по
манифесту должно быть видно, какой runtime действительно использует Kubernetes
API.

## Постановка задачи

Сделать token automount явным для module-owned Pod surfaces:

- `controller` Deployment использует Kubernetes API, поэтому должен явно иметь
  `automountServiceAccountToken: true`;
- `upload-gateway` Deployment использует Kubernetes API для session/storage
  state, поэтому должен явно иметь `automountServiceAccountToken: true`;
- publication worker Pod использует Kubernetes API для storage accounting/state,
  поэтому его generated PodSpec должен явно иметь token automount enabled;
- node-cache runtime Pod использует Kubernetes API для desired-artifacts/usage
  reporting, поэтому его generated PodSpec должен явно иметь token automount
  enabled.

## Scope

- `templates/controller/deployment.yaml`
- `templates/upload-gateway/deployment.yaml`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- render/unit guardrails

## Non-goals

- Не менять RBAC verbs.
- Не отключать token у компонентов, которым Kubernetes API реально нужен.
- Не мутировать пользовательский `automountServiceAccountToken` в workload
  delivery path.
- Не менять DMCR: там token уже явно disabled.

## Acceptance criteria

- Module-owned API clients have explicit `automountServiceAccountToken: true`.
- Components without API access keep explicit `false` where already present.
- Render validator catches missing explicit automount for controller and
  upload-gateway Deployments.
- Unit tests cover generated publication worker and node-cache runtime PodSpec.
- `make verify` passes.

## RBAC coverage

Human RBAC не меняется. ServiceAccount токены не расширяют права сами по себе;
изменение делает existing service-account usage явным и проверяемым.

## Риски

- False-positive hardening: нельзя ставить `false` на controller/upload/runtime
  Pods, пока они используют Kubernetes API for state, watches or leader election.
