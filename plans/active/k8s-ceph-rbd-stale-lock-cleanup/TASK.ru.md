# Stale Ceph RBD locks cleanup in live clusters

## Контекст

В `k8s-main` уже обнаружены Released/Terminating RBD PV, которые не удалялись
из-за stale `exclusive-lock` на RBD images после неудачного `rbd map`.
Похожая проблема есть в `k8s-dvp`: при удалении PVC/PV возникают `Access denied`
и накапливаются Released PV.

## Постановка задачи

Безопасно очистить stale Released/Terminating Ceph RBD PV в `k8s-dvp` и
повторно проверить `k8s-main`, чтобы старые Kubernetes PV и соответствующие
Ceph RBD images/trash были удалены.

## Scope

- Использовать kubeconfig `/Users/myskat_90/.kube/k8s-config`.
- Работать только с PV, у которых старый `claimRef`, статус Released или
  Terminating, и нет живого PVC с тем же UID.
- Для Ceph RBD проверять image, watchers, locks и trash.
- Снимать только stale RBD locks, у которых нет host map/current workload usage.
- Дать CSI штатно выполнить `DeleteVolume`, чтобы он зачистил RBD image и CSI
  OMAP metadata.
- Проверить, что текущие Bound PVC и pod workloads не затронуты.

## Non-goals

- Не удалять Bound PVC/PV.
- Не менять StorageClass, Ceph auth secrets, Deckhouse templates или модульный
  код без отдельного architectural slice.
- Не править Ceph caps без явного доступа к Ceph admin и отдельного решения.

## Затрагиваемые области

- Live clusters: `k8s-dvp`, `k8s-main`.
- Kubernetes PV/PVC/VolumeAttachment.
- Ceph CSI namespace and RBD pool state.

## Критерии приёмки

- Все найденные stale Released/Terminating RBD PV в `k8s-dvp`, безопасные для
  удаления, удалены из Kubernetes.
- Соответствующие RBD images отсутствуют в pool и не лежат в trash.
- В `k8s-main` нет оставшихся stale Released/Terminating RBD PV с lock-related
  delete failures.
- Текущие Bound PVC и pods, которые используют новые PV, остаются Running/Bound.
- Если найден системный дефект, он сформулирован отдельно: причина, текущий
  workaround и какой permanent fix нужен.

## Риски

- Ошибочное удаление Bound/current PV приведёт к потере данных.
- Снятие lock с реально используемого image может повредить активный workload.
- У CSI user может не быть Ceph caps для blocklist/lock removal; тогда нужен
  либо `rados lock break`, либо Ceph admin-side fix.
