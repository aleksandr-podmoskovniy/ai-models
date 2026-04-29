# Prod preflight: controller runtime RBAC hardening

## Контекст

Предыдущие preflight slices разделили runtime ServiceAccount для publication
worker и закрыли storage-accounting verb mismatch. Оставшийся высокий риск
перед prod — широкий controller ClusterRole на core `secrets`: сейчас он
позволяет не только читать source auth Secret, но и создавать/изменять/удалять
Secrets cluster-wide.

Аудит текущего delivery path показал, что cluster-wide Secret writes пока
являются частью live contract: controller проектирует registry/imagePull
Secrets в namespace workload'ов и удаляет их по UID workload'а. Kubernetes RBAC
не умеет ограничить это по имени/лейблу объекта. Поэтому этот slice не должен
делать fake hardening, которое сломает delivery; безопасное сужение здесь —
убрать лишние cluster-wide write verbs с runtime-owned Pod/PVC/Lease ресурсов.

## Постановка задачи

Сузить controller RBAC там, где это уже доказуемо безопасно: оставить
cluster-wide read/watch на Pod/PVC для наблюдения и workload delivery, но
перенести write verbs для module-owned Pod/PVC/Lease в namespaced Role модуля.
Secret projection hardening вынести в отдельный future slice с изменением
delivery-auth архитектуры.

## Scope

- `templates/controller/rbac.yaml`
- render validator checks in `tools/helm-tests/*`
- текущий task bundle evidence

## Non-goals

- Не менять public API `authSecretRef`.
- Не менять source auth UX и namespace ownership rules.
- Не менять human RBAC personas.
- Не переделывать controller-runtime cache architecture в этом slice.

## Затрагиваемые области

- Controller ServiceAccount RBAC.
- Render guardrails for broad runtime write verbs.

## Критерии приёмки

- Controller ClusterRole не выдаёт `create/update/patch/delete` на Pods,
  PersistentVolumeClaims и Leases cluster-wide.
- Module-owned Pod/PVC writes разрешены через Role только в namespace модуля.
- Leader-election Lease verbs разрешены через Role только в namespace модуля.
- Secret projection risk явно зафиксирован как отдельный architecture slice,
  без ломающего изменения текущего delivery path.
- Render validator ловит возврат broad Pod/PVC/Lease writes.
- Проходят `make helm-template`, `make kubeconform`, `make verify`.

## RBAC coverage

- Human roles не меняются.
- Controller SA:
  - cluster-wide scope: read/watch Pods/PVCs, current Secret projection access;
  - module namespace scope: Pod/PVC writes and Lease leadership.
- Sensitive human deny paths остаются без изменений.

## Риски

- Если controller-runtime cache требует `list/watch` для named Secret reads,
  cluster-wide read verbs могут остаться шире `get`. Запретить надо именно
  cluster-wide writes; дальнейшее сужение до direct uncached Secret reader —
  отдельный architecture slice.
- Cluster-wide Secret writes остаются residual risk текущей delivery-auth
  архитектуры, потому что workload namespaces произвольные. Честное исправление
  требует redesign: либо отказаться от projection Secrets, либо ввести другой
  namespace-scoped/auth handoff contract.
