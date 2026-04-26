# Notes

## 2026-04-26 live cleanup

### `k8s-dvp.apiac.ru`

- Найден один stale Released RBD PV:
  `pvc-72d9af32-5b94-4b54-8226-073612f49713`.
- Старый PVC UID не совпадал с текущим live PVC:
  `d8-monitoring/prometheus-longterm-db-prometheus-longterm-0`.
- RBD image:
  `rbd-k8s-main-nvme/csi-vol-a9195944-0ec1-4acc-a3ad-62fa87ad2b01`.
- Watchers отсутствовали, host map старого image отсутствовал.
- Висел stale `rbd_lock`:
  `client.395890091`, cookie `auto 18446462598732842451`,
  address `192.168.2.151:0/1965304996`.
- Lock снят через `rados lock break` по
  `rbd_header.72dadf58578665`.
- После `kubectl delete pv` CSI удалил PV и RBD image.
- `rbd trash ls` по pool `rbd-k8s-main-nvme` пустой.

### `k8s-main`

- Released/Terminating RBD PV не найдено.
- Старые PV из предыдущего cleanup отсутствуют.
- `upmeter-0` и `trivy-server-0` имели старые `FailedMount` events, но на
  момент проверки оба pod Running.
- Их RBD images mapped на целевых нодах и имеют matching watcher/lock:
  это active exclusive-lock, не stale lock. Такие locks не трогались.

## Systemic finding

Симптом повторяется после неудачного `rbd map`:

- kubelet/ceph-csi получает `rbd: sysfs write failed` /
  `rbd: map failed: (13) Permission denied`;
- image не остаётся mapped на host;
- в Ceph может остаться `rbd_lock`;
- CSI `DeleteVolume` позже падает с `rbd: ret=-16, Device or resource busy`;
- `rbd lock remove`/`rbd rm` пытается break lock через blocklist и может
  упереться в Ceph auth/caps;
- `rados lock break` по конкретному `rbd_header.<imageID>` снимает stale lock
  без удаления CSI OMAP вручную, после чего CSI штатно удаляет image.

Permanent fix должен быть на стороне storage/Ceph integration, не в
`ai-models`:

- проверить caps для `client.k8s.main` и привести их к Deckhouse/Ceph-CSI
  supported RBD profile с правами, достаточными для stale lock recovery;
- отдельно расследовать, почему kernel `rbd map` получает sysfs
  `Permission denied` после node/CSI restart;
- держать runbook/automation для безопасного cleanup: candidate PV должен быть
  Released/Terminating, без live PVC UID, без VolumeAttachment, без watcher и
  без host map; только после этого допустим `rados lock break`.
