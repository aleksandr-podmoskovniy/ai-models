# Pod-level RDMA through `sdn` on `k8s-dvp.apiac.ru`

## Цель

Зафиксировать текущий рабочий контракт для `sdn -> UnderlayNetwork -> pod ->
RDMA` на кластере `k8s-dvp.apiac.ru` и отделить:

- что уже работает;
- что требует временного workaround;
- где текущая база `sdn` ломает fully-automatic flow.

## Что уже доказано на живом кластере

### `sdn` видит нужные Mellanox PF

На целевых нодах `NodeNetworkInterface` уже публикует прямые Mellanox PF:

- `k8s-dvp-w1-gpu.apiac.ru`
  - `enp193s0np0` -> `0000:c1:00.0` -> `binding: DPDK` -> `driver: mlx5_core`
  - `enp130s0np0` -> `0000:82:00.0` -> `binding: NetDev` -> `driver: mlx5_core`
- `k8s-dvp-w3-gpu.apiac.ru`
  - `enp2s0np0` -> `0000:02:00.0` -> `binding: DPDK` -> `driver: mlx5_core`

Практический вывод:

- `UnderlayNetwork` можно селектить по:
  - `network.deckhouse.io/node-name`
  - `network.deckhouse.io/nic-pci-bus-info`
- для Mellanox usable RDMA baseline надо вести через `bindingMode: DPDK`.

### Pod-side RDMA surface реально доезжает

На `rdma-pod-w1` подтверждено:

- webhook добавил `spec.resourceClaims` и `resources.claims`;
- `sdn` перевёз в pod netdev `enp193s0np0`;
- внутри pod есть verbs device `mlx5_0`;
- `ibv_devinfo` показывает:
  - `PORT_ACTIVE`
  - `link_layer: Ethernet`
  - `active_mtu: 4096`
- в pod видны:
  - `/dev/infiniband/uverbs0`
  - `/sys/bus/pci/devices/0000:c1:00.0`

Это доказывает, что текущий Mellanox `DPDK` path в `sdn` уже даёт usable
verbs/RoCE surface внутри pod.

### End-to-end `sdn` pod-to-pod path работает

После включения `vIOMMU` на `k8s-dvp-w3-gpu.apiac.ru` второй `sdn` pod тоже
поднимается штатно:

- `rdma-pod-w1` на `k8s-dvp-w1-gpu.apiac.ru`
- `rdma-pod-w3` на `k8s-dvp-w3-gpu.apiac.ru`

Подтверждено на живом кластере:

- в обоих pod есть direct netdev:
  - `w1`: `enp193s0np0`
  - `w3`: `enp2s0np0`
- в обоих pod есть:
  - `mlx5_0`
  - `/dev/infiniband/uverbs0`
  - `PORT_ACTIVE`
  - `active_mtu: 4096`
- direct ICMP между pod по underlay path проходит
- `ib_write_bw` pod-to-pod проходит в обе стороны:
  - `w3 -> w1`: `BW average 85.26 Gb/sec`
  - `w1 -> w3`: `BW average 90.57 Gb/sec`

## Как это правильно настраивать сейчас

### 1. Включить `sdn`

Для этого кластера рабочий source:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sdn
spec:
  enabled: true
  source: deckhouse
```

Проверка:

```bash
kubectl --context=k8s-dvp.apiac.ru get modules sdn -o wide
kubectl --context=k8s-dvp.apiac.ru -n d8-sdn get pods -o wide
```

Ожидание:

- `modules/sdn`: `Ready`, `ENABLED=True`
- в `d8-sdn` запущены:
  - `controller`
  - `scheduler`
  - `agent`

### 2. Убедиться, что `NodeNetworkInterface` видит нужные PF

Команда:

```bash
kubectl --context=k8s-dvp.apiac.ru get nodenetworkinterfaces.network.deckhouse.io -o wide
```

Для direct RDMA path у нас важны:

- `k8s-dvp-w1-gpu.apiac.ru-nic-fc886e8e65ce`
  - `IFNAME=enp193s0np0`
  - `VENDOR=Mellanox`
  - `BINDING=DPDK`
- `k8s-dvp-w3-gpu.apiac.ru-nic-c6aeb0867b1f`
  - `IFNAME=enp2s0np0`
  - `VENDOR=Mellanox`
  - `BINDING=DPDK`

Если нужного PF нет в `NodeNetworkInterface`, дальше в `UnderlayNetwork`
идти бессмысленно.

### 3. Создать namespace для теста

Нужны:

- namespace-level opt-in для underlay;
- привилегированная pod policy, потому что внутри pod нужны сетевые проверки.

Пример:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: sdn-rdma-test
  labels:
    direct-nic-access-enable.network.deckhouse.io/rdma-pf-a: ""
    security.deckhouse.io/pod-policy: privileged
```

Что делает `sdn`:

- `NamespaceController` сам добавляет:
  - `direct-nic-access.network.deckhouse.io/enabled=""`

Проверка:

```bash
kubectl --context=k8s-dvp.apiac.ru get ns sdn-rdma-test --show-labels
```

### 4. Создать `UnderlayNetwork`

Для pod-level direct PF path сейчас рабочий baseline такой:

- `mode: Dedicated`
- `autoBonding: false`
- точечный `labelSelector` по `node-name + nic-pci-bus-info`

Пример:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: UnderlayNetwork
metadata:
  name: rdma-pf-a
spec:
  mode: Dedicated
  autoBonding: false
  memberNodeNetworkInterfaces:
  - labelSelector:
      matchLabels:
        network.deckhouse.io/node-name: k8s-dvp-w1-gpu.apiac.ru
        network.deckhouse.io/nic-pci-bus-info: 0000-c1-00.0
  - labelSelector:
      matchLabels:
        network.deckhouse.io/node-name: k8s-dvp-w3-gpu.apiac.ru
        network.deckhouse.io/nic-pci-bus-info: 0000-02-00.0
```

Проверка:

```bash
kubectl --context=k8s-dvp.apiac.ru get underlaynetwork rdma-pf-a -o yaml
kubectl --context=k8s-dvp.apiac.ru get deviceclass d8-sdn-rdma-pf-a
kubectl --context=k8s-dvp.apiac.ru get resourceslices.resource.k8s.io
```

Ожидание:

- `status.conditions`:
  - `InterfacesAvailable=True`
- `DeviceClass d8-sdn-rdma-pf-a` существует
- в `ResourceSlice` появились устройства `network.deckhouse.io` с
  `underlayNetwork=rdma-pf-a`

### 5. Важный баг: `ResourceClaimTemplate` сейчас не создаётся автоматически

Штатно `sdn` должен сам создать namespace-scoped template
`d8-sdn-rdma-pf-a`.

Но на Kubernetes `resource.k8s.io/v1` текущий controller пишет старую DRA
схему и ломается на валидации. Поэтому до фикса controller нужен временный
manual workaround:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: d8-sdn-rdma-pf-a
  namespace: sdn-rdma-test
  labels:
    network.deckhouse.io/underlay-network: rdma-pf-a
spec:
  metadata:
    labels:
      network.deckhouse.io/underlay-network: rdma-pf-a
  spec:
    devices:
      requests:
      - name: nic
        exactly:
          deviceClassName: d8-sdn-rdma-pf-a
          count: 1
```

Проверка:

```bash
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test get resourceclaimtemplate d8-sdn-rdma-pf-a -o yaml
```

### 6. Создать validation pod

Для Mellanox RDMA baseline в pod annotation нужно просить именно
`bindingMode: DPDK`.

Пример pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: rdma-pod-w1
  namespace: sdn-rdma-test
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "rdma-pf-a",
          "bindingMode": "DPDK"
        }
      ]
spec:
  nodeSelector:
    kubernetes.io/hostname: k8s-dvp-w1-gpu.apiac.ru
  tolerations:
  - key: dedicated.apiac.ru
    operator: Equal
    value: w-gpu
    effect: NoExecute
  restartPolicy: Never
  containers:
  - name: tools
    image: ubuntu:22.04
    command:
    - bash
    - -lc
    - |
      set -euo pipefail
      export DEBIAN_FRONTEND=noninteractive
      apt-get update >/dev/null
      apt-get install -y iproute2 iputils-ping ibverbs-utils perftest rdma-core ethtool >/dev/null
      sleep infinity
    securityContext:
      runAsUser: 0
      capabilities:
        add:
        - NET_ADMIN
        - NET_RAW
        - IPC_LOCK
```

Что должно произойти автоматически:

- pod webhook добавит:
  - `spec.resourceClaims`
  - `containers[].resources.claims`
- появится `ResourceClaim`
- kubelet позовёт `NodePrepareResources`
- `sdn/agent` подготовит CDI и netdev handoff

Проверка:

```bash
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test get pod rdma-pod-w1 -o yaml
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test get resourceclaim
```

## Как проверять внутри pod

### Presence checks

```bash
ip -br link
rdma link show
ibv_devices
ibv_devinfo -v
ls -la /dev/infiniband
ls -la /sys/bus/pci/devices/<PCI_BUS_INFO>
```

Ожидание для Mellanox PF:

- в pod есть обычный cluster interface `eth0`;
- в pod приехал direct интерфейс, например `enp193s0np0`;
- `ibv_devices` показывает `mlx5_0`;
- `ibv_devinfo` показывает:
  - `PORT_ACTIVE`
  - `link_layer: Ethernet`
  - `active_mtu: 4096`
- в `/dev/infiniband` есть `uverbs0`

### Runtime metadata checks

```bash
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test get pod rdma-pod-w1 -o jsonpath='{.metadata.annotations.network\.deckhouse\.io/networks-status}'
```

Ожидание:

- `type=UnderlayNetwork`
- `bindingMode=DPDK`
- `netDevInterfaces[].name=enp193s0np0`
- `conditions`:
  - `Configured=True`
  - `Negotiated=True`

## Как проверять connectivity и bandwidth

### Pod-to-pod smoke, который уже доказан

Оба validation pod должны быть `Running` и сидеть на нужных GPU-нодах.

Назначаем временные IP на direct интерфейсы:

```bash
# w1
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w1 -- \
  bash -lc 'ip addr replace 172.31.120.1/30 dev enp193s0np0'

# w3
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w3 -- \
  bash -lc 'ip addr replace 172.31.120.2/30 dev enp2s0np0'
```

Проверяем связность:

```bash
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w1 -- \
  bash -lc 'ping -I enp193s0np0 -c 3 172.31.120.2'

kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w3 -- \
  bash -lc 'ping -I enp2s0np0 -c 3 172.31.120.1'
```

Потом обязательно смотрим локальные GID, потому что индекс зависит от набора IP
в конкретном pod:

```bash
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w1 -- \
  bash -lc 'show_gids 2>/dev/null || ibv_devinfo -v | sed -n "/GID\\[/,/^$/p"'

kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w3 -- \
  bash -lc 'show_gids 2>/dev/null || ibv_devinfo -v | sed -n "/GID\\[/,/^$/p"'
```

На live-проверке после зачистки старых test IP получили:

- `rdma-pod-w1`: IPv4 RoCE v2 `GID index 5` для `172.31.120.1`
- `rdma-pod-w3`: IPv4 RoCE v2 `GID index 3` для `172.31.120.2`

Тест `w3 -> w1`:

```bash
# server on w1
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec -i rdma-pod-w1 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 5 -m 4096 -F --report_gbits -D 5'

# client on w3
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w3 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 172.31.120.1'
```

Результат:

- `BW average: 85.26 Gb/sec`

Тест `w1 -> w3`:

```bash
# server on w3
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec -i rdma-pod-w3 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5'

# client on w1
kubectl --context=k8s-dvp.apiac.ru -n sdn-rdma-test exec rdma-pod-w1 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 5 -m 4096 -F --report_gbits -D 5 172.31.120.2'
```

Результат:

- `BW average: 90.57 Gb/sec`

## Что пока остаётся нештатным

### Blocker 1. `ResourceClaimTemplate` schema drift

Текущий `sdn controller` генерирует template по старой схеме DRA.

Симптомы в логах controller:

- `unknown field "spec.spec.devices.requests[0].count"`
- `unknown field "spec.spec.devices.requests[0].deviceClassName"`
- `exactly one of exactly or firstAvailable is required`

Практический эффект:

- `DeviceClass` создаётся;
- `ResourceClaimTemplate` автоматически не создаётся корректно;
- без manual workaround pod-level flow не стартует штатно.

### Nuance 2. Для VM-ноды `w3` нужен guest-visible `iommu_group`

`k8s-dvp-w3-gpu.apiac.ru` живёт как VM на PVE. До включения guest `vIOMMU` у
PF `0000:02:00.0` в госте не было `iommu_group`, и `sdn/agent` зависал на
`NodePrepareResources`.

После включения `vIOMMU` и kernel args:

- `intel_iommu=on`
- `iommu=pt`

в госте появился:

- `/sys/bus/pci/devices/0000:02:00.0/iommu_group -> /sys/kernel/iommu_groups/12`

и `PrepareResourceClaims` на `w3` стал завершаться штатно.

Практический эффект:

- для bare metal path это не всплывало;
- для VM на PVE этот prerequisite надо явно включать в инструкцию;
- сам баг в `sdn`, что отсутствие `iommu_group` превращалось в hard hang,
  всё равно остаётся отдельным engineering issue.

## Рекомендация для следующего шага

Для полной инструкции двигаться в таком порядке:

1. Host prerequisites на нодах:
   - Mellanox/RDMA/MTU/QoS
   - GPU driver/operator
2. `sdn` enablement и проверка `NodeNetworkInterface`
3. `UnderlayNetwork` для нужной пары PF
4. Manual `ResourceClaimTemplate` workaround до фикса controller
5. Validation pod с `bindingMode: DPDK`
6. Явная `toleration` для GPU-ноды:
   - `dedicated.apiac.ru=w-gpu:NoExecute`
7. Для VM-ноды дополнительно:
   - `vIOMMU` на гипервизоре
   - `intel_iommu=on iommu=pt` в guest
8. Inside-pod verbs/netdev checks
9. `pod -> pod` smoke с временными `/30` на direct netdev
10. Отдельный follow-up:
   - fix DRA template generation
   - убрать жёсткую зависимость `sdn` от `iommu_group` для Mellanox `DPDK`
