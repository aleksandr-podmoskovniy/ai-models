# Plan

## Current phase

Операционный cleanup вокруг runtime/storage baseline. Product code не меняется.

## Orchestration

`solo`: задача live-ops, главный риск управляется проверками PV/PVC,
VolumeAttachment, RBD watchers/locks и host maps перед удалением.

## Slices

1. Inventory
   - Проверить contexts в `/Users/myskat_90/.kube/k8s-config`.
   - Собрать Released/Terminating RBD PV по `k8s-dvp` и `k8s-main`.
   - Сопоставить old claimRef UID с live PVC.

2. Safety checks
   - Проверить VolumeAttachment для candidate PV.
   - Проверить current pods/PVC с совпадающими claim names.
   - Проверить RBD status/lock/list mapped через Ceph CSI tools.

3. Cleanup
   - Для stale images без watchers/host maps снять stale `rbd_lock` через
     `rados lock break`.
   - Удалить Released PV или дождаться external-provisioner retry.
   - Проверить, что CSI выполнил `DeleteVolume` и удалил image/OMAP.

4. Systemic conclusion
   - Зафиксировать, почему появляется stale lock.
   - Сформулировать permanent fix отдельно от разового cleanup.

## Rollback point

До удаления PV и снятия RBD lock остановиться можно без изменений. После снятия
lock rollback не нужен, если image не имеет watchers/maps и PV уже Released.

## Final validation

- `kubectl get pv` по candidate PV возвращает `NotFound`.
- `rbd info` по candidate images возвращает `No such file or directory`.
- `rbd trash ls` не содержит candidate images.
- Live PVC remain `Bound`, связанные pods remain `Running`.
