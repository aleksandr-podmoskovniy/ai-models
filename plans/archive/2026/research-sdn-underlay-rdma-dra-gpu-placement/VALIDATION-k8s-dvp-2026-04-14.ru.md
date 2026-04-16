# Live validation on `k8s-dvp.apiac.ru`

## Scope

- cluster: `k8s-dvp.apiac.ru`
- nodes:
  - `k8s-dvp-w1-gpu.apiac.ru`
  - `k8s-dvp-w3-gpu.apiac.ru`

## Important correction

Первичный negative result был снят по неверному device path:

- на `w1` был активен `mlx5_bond_0`, привязанный к `bond0` на базе
  `enp65s0f0np0`/`enp65s0f1np1`;
- это не отдельный direct 100GbE CX5 path, а другой сетевой контур;
- direct Mellanox ConnectX-5 path на `w1` живёт на:
  - `enp193s0np0` -> `mlx5_0` -> `0000:c1:00.0`
  - `enp130s0np0` -> `mlx5_1` -> `0000:82:00.0`
- direct Mellanox ConnectX-5 path на `w3`:
  - `enp2s0np0` -> `mlx5_0` -> `0000:02:00.0`

## Baseline state before temporary test config

### `k8s-dvp-w1-gpu.apiac.ru`

- оба direct CX5 порта физически присутствуют и поддерживают 100GbE;
- оба порта отсутствуют в `/etc/netplan/50-cloud-init.yaml`;
- `enp193s0np0` и `enp130s0np0` были в admin `DOWN`;
- после чтения из pod:
  - `ethtool` показывал `Link detected: no`;
  - `rdma link show` показывал `mlx5_0` и `mlx5_1` как `DOWN`.

### `k8s-dvp-w3-gpu.apiac.ru`

- direct CX5 `enp2s0np0` уже был `UP/LOWER_UP`;
- `ethtool` показывал `Speed: 100000Mb/s`;
- в host config был описан только management `enp6s18`;
- отдельного IP-конфига для `enp2s0np0` не было.

## Temporary validation actions

Для доказательства работоспособности именно direct CX5 path были выполнены
временные действия:

1. На `w1` direct порты переведены в admin `UP`.
2. Подтвержден carrier и `100000Mb/s` на:
   - `enp193s0np0`
   - `enp130s0np0`
3. Для point-to-point проверки назначались временные IP на direct линке:
   - channel A:
     - `w1/enp193s0np0` -> `172.31.100.1/30`
     - `w3/enp2s0np0` -> `172.31.100.2/30`
   - channel B:
     - `w1/enp130s0np0` -> `172.31.101.1/30`
     - `w3/enp2s0np0` -> `172.31.101.2/30`
4. На обоих концах для обоих тестов появился IPv4-based RoCE GID index `3`.

## RDMA result on all 3 Mellanox 100GbE cards

Проверены все три direct Mellanox endpoints:

- `w1 / mlx5_0 / enp193s0np0`
- `w1 / mlx5_1 / enp130s0np0`
- `w3 / mlx5_0 / enp2s0np0`

### Channel A

- server:
  - node: `k8s-dvp-w1-gpu.apiac.ru`
  - device: `mlx5_0`
  - netdev: `enp193s0np0`
  - gid index: `3`
- client:
  - node: `k8s-dvp-w3-gpu.apiac.ru`
  - device: `mlx5_0`
  - netdev: `enp2s0np0`
  - gid index: `3`

Connectivity:

- ICMP:
  - `w1 -> w3`: OK
  - `w3 -> w1`: OK

Bandwidth test:

```bash
ib_write_bw -d mlx5_0 -x 3 -F --report_gbits
```

Result:

- message size: `65536`
- iterations: `5000`
- peak BW: `90.45 Gb/sec`
- average BW: `49.08 Gb/sec`
- MsgRate: `0.093622 Mpps`

### Channel B

- server:
  - node: `k8s-dvp-w1-gpu.apiac.ru`
  - device: `mlx5_1`
  - netdev: `enp130s0np0`
  - gid index: `3`
- client:
  - node: `k8s-dvp-w3-gpu.apiac.ru`
  - device: `mlx5_0`
  - netdev: `enp2s0np0`
  - gid index: `3`

Connectivity:

- ICMP:
  - `w1 -> w3`: OK
  - `w3 -> w1`: OK

Bandwidth test:

```bash
ib_write_bw -d mlx5_1 -x 3 -F --report_gbits
```

Result:

- message size: `65536`
- iterations: `5000`
- peak BW: `84.95 Gb/sec`
- average BW: `42.50 Gb/sec`
- MsgRate: `0.081059 Mpps`

### Verdict

Это подтверждает:

- все 3 Mellanox 100GbE карты в целевой паре нод могут работать как RDMA
  endpoints;
- direct 100GbE Mellanox path между `w1` и `w3` существует по обоим CX5 на
  `w1`;
- RDMA verbs по этому direct path реально работают;
- исходная проблема была не в отсутствии RDMA как такового, а в выборе
  неправильного physical path и отсутствии host-side config на `w1` для direct
  CX5.

## Interim cleanup after initial test

После первичной проверки временные `/30` адреса удалялись.

Дальше, по явной просьбе пользователя, direct порты на `w1` больше не
опускались и были оставлены в `UP`.

## `sdn` module enablement

### What failed first

Первое включение было сделано через:

- `ModuleConfig/sdn`
- `spec.source: gpu-fox`

Это оказалось неверным для данного кластера:

- `deckhouse` логировал `UNAUTHORIZED` при попытке скачать
  `dev-registry.deckhouse.io/sys/deckhouse-oss/modules/sdn/release:alpha`.

### Correct enablement

Корректный source для этого кластера:

- `deckhouse`

Итог:

- `ModuleConfig/sdn`:
  - `enabled: true`
  - `source: deckhouse`
- `ModuleRelease/sdn-v0.5.3`:
  - `phase: Deployed`
- namespace:
  - `d8-sdn`
- workloads:
  - `controller` ready
  - `scheduler` ready
  - `agent` daemonset ready on all nodes

## MTU 9000 fix on direct RDMA interfaces

После подтверждения RDMA на `1500 MTU` был сделан отдельный operational slice:
поднять jumbo MTU только на direct Mellanox 100GbE path, не затрагивая
management fabric.

### `k8s-dvp-w1-gpu.apiac.ru`

Сеть на ноде ведёт `systemd-networkd`; direct CX5 порты остаются unmanaged для
IP-конфига, поэтому для persistence был выбран link-level механизм:

- `/etc/systemd/network/10-rdma-enp193s0np0.link`
- `/etc/systemd/network/10-rdma-enp130s0np0.link`

Оба файла содержат:

```ini
[Match]
OriginalName=<target-interface>

[Link]
MTUBytes=9000
```

Runtime state после применения:

- `enp193s0np0`: `UP`, `LOWER_UP`, `mtu 9000`
- `enp130s0np0`: `UP`, `LOWER_UP`, `mtu 9000`

### `k8s-dvp-w3-gpu.apiac.ru`

Сеть на ноде ведёт `NetworkManager`; direct CX5 порт жил как external runtime
connection без persistent keyfile. Для persistence создан отдельный connection:

- `connection.id: enp2s0np0-rdma`
- `connection.interface-name: enp2s0np0`
- `connection.autoconnect: yes`
- `802-3-ethernet.mtu: 9000`
- `ipv4.method: disabled`
- `ipv6.method: ignore`

Runtime state после применения:

- `enp2s0np0`: `UP`, `LOWER_UP`, `mtu 9000`

### Validation after MTU change

Для проверки jumbo path были временно назначены `/30` адреса:

- `w1/enp193s0np0` -> `172.31.100.1/30`
- `w1/enp130s0np0` -> `172.31.101.1/30`
- `w3/enp2s0np0` -> `172.31.100.2/30`
- `w3/enp2s0np0` -> `172.31.101.2/30`

Jumbo ping:

- `w1/enp193s0np0 -> w3/enp2s0np0`:
  - `ping -I enp193s0np0 -M do -s 8972 -c 3 172.31.100.2`
  - `3/3 received`, `0% packet loss`
- `w1/enp130s0np0 -> w3/enp2s0np0`:
  - `ping -I enp130s0np0 -M do -s 8972 -c 3 172.31.101.2`
  - `3/3 received`, `0% packet loss`

После `mtu 9000` RoCE verbs path перешёл на:

- `mlx5_0 active_mtu: 4096`
- `mlx5_1 active_mtu: 4096`
- `w3 mlx5_0 active_mtu: 4096`

RDMA smoke после смены MTU:

- channel A:
  - server: `w1 mlx5_0`
  - client: `w3 mlx5_0`
  - command: `ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 10`
  - result: `BW average 83.72 Gb/sec`
- channel B:
  - server: `w1 mlx5_1`
  - client: `w3 mlx5_0`
  - command: `ib_write_bw -d mlx5_1 -x 3 -m 4096 -F --report_gbits -D 10`
  - result: `BW average 84.16 Gb/sec`

После проверки временные `/30` адреса были удалены. MTU `9000`, persistent
host config и `UP` состояние direct RDMA портов оставлены на месте.

## Practical conclusion

Для дальнейшего `sdn`/Underlay/DRA workstream надо ориентироваться именно на:

- `w1`:
  - `enp193s0np0`
  - `enp130s0np0`
- `w3`:
  - `enp2s0np0`

а не на bonded management/Access path через `bond0` и `mlx5_bond_0`.

Текущее рабочее состояние для direct RDMA path:

- `w1/enp193s0np0`: `UP`, `mtu 9000`
- `w1/enp130s0np0`: `UP`, `mtu 9000`
- `w3/enp2s0np0`: `UP`, `mtu 9000`

## `sdn` pod-level validation after enabling guest `vIOMMU` on `w3`

### VM prerequisite on `k8s-dvp-w3-gpu.apiac.ru`

`w3` живёт как VM на PVE. До включения `vIOMMU` у Mellanox PF
`0000:02:00.0` в госте не было `iommu_group`, и `sdn/agent` зависал на
`NodePrepareResources`.

После включения `vIOMMU` на PVE и добавления в guest kernel cmdline:

- `intel_iommu=on`
- `iommu=pt`

подтверждено:

- `/sys/bus/pci/devices/0000:02:00.0/iommu_group -> /sys/kernel/iommu_groups/12`
- `dmesg` содержит `DMAR: IOMMU enabled`
- `dmesg` содержит `Intel(R) Virtualization Technology for Directed I/O`

### `sdn` path on `w3`

После этого validation pod `rdma-pod-w3` смог пройти полный DRA prepare path:

- pod scheduled на `k8s-dvp-w3-gpu.apiac.ru`
- `ResourceClaim` перешёл в `allocated,reserved`
- `d8-sdn/agent` на `w3` записал:
  - `bindingArtifacts: &{IOMMUGroup:12 VFIODeviceFile: InfinibandFile:uverbs0}`
  - `Adding net device enp2s0np0`
  - `Prepared CDI spec`
  - `Claim ... preparation completed`

Это подтверждает, что прежний blocker на `w3` был снят именно включением
guest-visible IOMMU metadata.

### Two `sdn` pods with RDMA inside

Подняты два pod через `UnderlayNetwork rdma-pf-a` и `bindingMode: DPDK`:

- `sdn-rdma-test/rdma-pod-w1` on `k8s-dvp-w1-gpu.apiac.ru`
- `sdn-rdma-test/rdma-pod-w3` on `k8s-dvp-w3-gpu.apiac.ru`

Внутри `rdma-pod-w1` подтверждено:

- `enp193s0np0`
- `/dev/infiniband/uverbs0`
- `ibv_devices -> mlx5_0`
- `ibv_devinfo`:
  - `PORT_ACTIVE`
  - `link_layer: Ethernet`
  - `active_mtu: 4096`

Внутри `rdma-pod-w3` подтверждено:

- `enp2s0np0`
- `/dev/infiniband/uverbs0`
- `ibv_devices -> mlx5_0`
- `ibv_devinfo`:
  - `PORT_ACTIVE`
  - `link_layer: Ethernet`
  - `active_mtu: 4096`

### Pod-to-pod connectivity and RDMA bandwidth

Для direct underlay path временно назначены адреса:

- `rdma-pod-w1/enp193s0np0` -> `172.31.120.1/30`
- `rdma-pod-w3/enp2s0np0` -> `172.31.120.2/30`

ICMP между pod:

- `w1 -> w3`: OK
- `w3 -> w1`: OK

GID after cleanup of stale test IP:

- `rdma-pod-w1`: RoCE v2 IPv4 `GID index 5` for `172.31.120.1`
- `rdma-pod-w3`: RoCE v2 IPv4 `GID index 3` for `172.31.120.2`

RDMA test `w3 -> w1`:

- server: `rdma-pod-w1`
- client: `rdma-pod-w3`
- command:
  - server: `ib_write_bw -d mlx5_0 -x 5 -m 4096 -F --report_gbits -D 5`
  - client: `ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 172.31.120.1`
- result:
  - `BW average 85.26 Gb/sec`

RDMA test `w1 -> w3`:

- server: `rdma-pod-w3`
- client: `rdma-pod-w1`
- command:
  - server: `ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5`
  - client: `ib_write_bw -d mlx5_0 -x 5 -m 4096 -F --report_gbits -D 5 172.31.120.2`
- result:
  - `BW average 90.57 Gb/sec`

### Remaining non-automatic part

Даже после того как `w3` стал рабочим, полностью автоматическим flow это пока
не делает:

- `UnderlayNetworkController` по-прежнему генерирует невалидный
  `ResourceClaimTemplate` для `resource.k8s.io/v1`
- в кластере используется manual workaround:
  - `sdn-rdma-test/ResourceClaimTemplate d8-sdn-rdma-pf-a`

Итог:

- host-level RDMA между `w1` и `w3` подтверждён
- pod-level RDMA через `sdn` подтверждён
- end-to-end `sdn` pod-to-pod RDMA подтверждён
- текущий remaining product issue:
  - auto-generation `ResourceClaimTemplate`

## Expansion to 3 NIC for 3-pod baseline

После этого underlay был расширен с 2 до 3 Mellanox PF:

- `w1 / 0000:c1:00.0 / enp193s0np0`
- `w1 / 0000:82:00.0 / enp130s0np0`
- `w3 / 0000:02:00.0 / enp2s0np0`

Текущее состояние `UnderlayNetwork rdma-pf-a`:

- `generation: 2`
- `InterfacesAvailable=True`
- `message: All 3 member node network interface selectors have matches`

Live DRA inventory после patch:

- `w1` network `ResourceSlice` публикует 2 устройства underlay `rdma-pf-a`
- `w3` network `ResourceSlice` публикует 1 устройство underlay `rdma-pf-a`

### Important operational nuance

Если просто добавить третью карту в уже живой underlay и не пересоздать
старые test pod, можно поймать конфликтную аллокацию поверх старых claims.

На практике:

- in-place expansion поверх уже работающих pod сначала привела к неудачному
  allocation для нового `rdma-pod-w1b`
- после полного удаления старых test pod/claims и fresh create всё
  разложилось корректно

### Fresh 3-pod allocation result

После clean recreate были подняты:

- `rdma-pod-w1a` on `k8s-dvp-w1-gpu.apiac.ru`
- `rdma-pod-w1b` on `k8s-dvp-w1-gpu.apiac.ru`
- `rdma-pod-w3` on `k8s-dvp-w3-gpu.apiac.ru`

Подтверждённое распределение:

- `rdma-pod-w1a`:
  - `enp193s0np0`
  - `mlx5_0`
- `rdma-pod-w1b`:
  - `enp130s0np0`
  - `mlx5_1`
- `rdma-pod-w3`:
  - `enp2s0np0`
  - `mlx5_0`

`NodeNetworkInterface` status после этого:

- `k8s-dvp-w1-gpu.apiac.ru-nic-fc886e8e65ce` -> `takenOverByPod=sdn-rdma-test/rdma-pod-w1a`
- `k8s-dvp-w1-gpu.apiac.ru-nic-82b5c4697c97` -> `takenOverByPod=sdn-rdma-test/rdma-pod-w1b`
- `k8s-dvp-w3-gpu.apiac.ru-nic-c6aeb0867b1f` -> `takenOverByPod=sdn-rdma-test/rdma-pod-w3`

### RDMA smoke for the second card on `w1`

Чтобы отдельно подтвердить вторую карту `w1` уже в pod:

- `rdma-pod-w1b/enp130s0np0` -> `172.31.121.1/30`
- `rdma-pod-w3/enp2s0np0` -> `172.31.121.2/30`

Проверка:

- ICMP `w1b -> w3`: OK
- `rdma-pod-w1b` GID:
  - `172.31.121.1` -> RoCE v2 `GID index 3`
- `rdma-pod-w3` GID:
  - `172.31.121.2` -> RoCE v2 `GID index 3`

RDMA test:

- server: `rdma-pod-w3`, `mlx5_0`
- client: `rdma-pod-w1b`, `mlx5_1`
- command:
  - server: `ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5`
  - client: `ib_write_bw -d mlx5_1 -x 3 -m 4096 -F --report_gbits -D 5 172.31.121.2`
- result:
  - `BW average 92.01 Gb/sec`

## Migration to 3 deterministic underlay pairs

После этого тестовый контур был переведён с одного общего `UnderlayNetwork` на
3 отдельные one-NIC underlay, чтобы зафиксировать pairing под будущий
`GPU + NIC` layout:

- `rdma-w1-pair80`
  - node: `k8s-dvp-w1-gpu.apiac.ru`
  - NIC: `0000:82:00.0`
  - netdev: `enp130s0np0`
  - verbs device in pod: `mlx5_1`
- `rdma-w1-pairc0`
  - node: `k8s-dvp-w1-gpu.apiac.ru`
  - NIC: `0000:c1:00.0`
  - netdev: `enp193s0np0`
  - verbs device in pod: `mlx5_0`
- `rdma-w3-pair00`
  - node: `k8s-dvp-w3-gpu.apiac.ru`
  - NIC: `0000:02:00.0`
  - netdev: `enp2s0np0`
  - verbs device in pod: `mlx5_0`

Поднятые pod:

- `rdma-pod-w1-80`
- `rdma-pod-w1-c0`
- `rdma-pod-w3-00`

Подтверждённый deterministic mapping:

- `rdma-pod-w1-80` -> `enp130s0np0`, `mlx5_1`
- `rdma-pod-w1-c0` -> `enp193s0np0`, `mlx5_0`
- `rdma-pod-w3-00` -> `enp2s0np0`, `mlx5_0`

Это уже не “любой свободный NIC из общего пула”, а жёсткая one-NIC network per
worker path.

### RDMA smoke on the new deterministic pair `w1-pair80 <-> w3-pair00`

Temporary IP:

- `rdma-pod-w1-80/enp130s0np0` -> `172.31.140.1/30`
- `rdma-pod-w3-00/enp2s0np0` -> `172.31.140.2/30`

GID:

- `rdma-pod-w1-80` -> RoCE v2 `GID index 3`
- `rdma-pod-w3-00` -> RoCE v2 `GID index 3`

Connectivity:

- `ping w1-80 -> w3-00`: OK
- `ping w3-00 -> w1-80`: OK

RDMA test:

- server: `rdma-pod-w3-00`, `mlx5_0`
- client: `rdma-pod-w1-80`, `mlx5_1`
- command:
  - server: `ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5`
  - client: `ib_write_bw -d mlx5_1 -x 3 -m 4096 -F --report_gbits -D 5 172.31.140.2`
- result:
  - `BW average 90.51 Gb/sec`

Итог:

- deterministic one-NIC underlay path подтверждён
- это лучший текущий baseline для будущих fixed worker group в `KubeRay`
