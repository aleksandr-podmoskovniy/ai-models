# Prod preflight: node-cache registrar security hardening

## Контекст

Продолжаем production preflight по Kubernetes security conventions. В
node-cache runtime pod есть kubelet-facing CSI runtime container и sidecar
`node-driver-registrar`. Runtime container намеренно privileged из-за bind-mount
и CSI node-server semantics. Registrar sidecar только регистрирует CSI socket в
kubelet plugin registry и не должен получать Linux capabilities.

## Постановка задачи

Сузить security context `node-driver-registrar`:

- оставить root execution, потому что sidecar пишет в hostPath kubelet plugin
  registry;
- оставить `allowPrivilegeEscalation=false`;
- явно добавить `capabilities.drop: ["ALL"]`;
- покрыть generated PodSpec unit test, чтобы regression не вернулся.

## Scope

- `images/controller/internal/adapters/k8s/nodecacheruntime/pod.go`
- `images/controller/internal/adapters/k8s/nodecacheruntime/pod_test.go`

## Non-goals

- Не менять privileged CSI runtime container: это отдельный kubelet-facing
  контракт.
- Не менять node-cache RBAC, PVC/substrate или placement.
- Не менять rendered Helm templates: node-cache runtime pod создаётся
  controller'ом.

## Acceptance criteria

- Registrar sidecar drops all Linux capabilities.
- Existing privileged runtime container semantics remain unchanged.
- Unit tests cover registrar security context.
- Targeted tests and repo verify pass.

## RBAC coverage

Human RBAC и service-account RBAC не меняются. Slice меняет только generated pod
security context.

## Риски

- Если registrar image unexpectedly требует capabilities, node-cache pod может
  не стартовать. Это маловероятно: socket registration path требует filesystem
  access, не Linux capabilities.
