# Workaround: RDMA for KubeRay + vLLM on current `sdn`

## 1. Решение

Для текущей базы `sdn` самый реалистичный workaround под `KubeRay + vLLM` такой:

- не пытаться сразу делать pod-level direct RDMA через `UnderlayNetwork` в сами
  Ray worker pods;
- использовать `sdn` на уровне **ноды**, а не pod:
  - `UnderlayNetwork` как underlay inventory;
  - `SystemNetwork` как node-level RDMA/RoCE сеть с IPAM;
- запускать **worker** pods KubeRay в `hostNetwork: true`;
- давать контейнерам доступ к:
  - GPU через NVIDIA stack;
  - `/dev/infiniband`;
  - `/dev/shm`;
- заставлять NCCL/Gloo использовать node-level RDMA interface через env.

Это решение сознательно обходное. Оно выбрано потому что текущий `sdn` лучше
подходит для node-level RDMA connectivity, чем для clean pod-level RoCE data
plane у Ray/vLLM.

## 2. Почему выбран именно этот путь

### Что в текущем `sdn` уже хорошо работает

- `UnderlayNetwork` умеет выбирать PF/VF и управлять SR-IOV.
- `SystemNetwork` умеет поднимать service network на нодах поверх underlay и
  выдавать ей IP через `ClusterIPAddressPool`.
- Для `KubeRay + vLLM` это достаточно, чтобы каждая GPU-нода получила свою
  отдельную RoCE/RDMA сеть на хосте.

### Что мешает использовать direct pod attachment как основной workaround

- `UnderlayNetwork` в pod сейчас не делает полноценный pod-level IPAM для
  underlay links.
- Для RoCE-v2 это критично, потому что data-plane обычно требует IP на
  интерфейсе.
- У `sdn` нет отдельного `bindingMode: RDMA`; сегодня есть только `NetDev`,
  `VFIO-PCI`, `DPDK`.

Итог: pod-level underlay path можно пробовать только как отдельный
экспериментальный сценарий с ручной настройкой IP внутри pod. Для `KubeRay +
vLLM` это хуже node-level workaround.

## 3. Целевая схема

### Control plane

- Ray head pod:
  - обычный pod network;
  - без RDMA.
- Worker pods:
  - `hostNetwork: true`;
  - один worker pod на node;
  - vLLM/Ray control traffic идёт по обычному node IP;
  - NCCL/Gloo data-plane уходит через выделенный RDMA/RoCE interface на node.

### RDMA network

- на каждой GPU node выделяется отдельный Mellanox interface/port;
- `sdn` оформляет его как `UnderlayNetwork`;
- поверх него создаётся `SystemNetwork` c `type: Access` и IPAM;
- worker pod в `hostNetwork` использует этот интерфейс как host interface.

### Почему `Access`, а не `SRIOVVirtualFunction`

Для первого рабочего workaround `Access` проще:

- меньше moving parts;
- не нужен дополнительный VF lifecycle для pod;
- легче дебажить QoS/PFC/ECN и host-side RDMA stack.

Если позже понадобится многопользовательский shared node-level path, можно
перейти к `SystemNetwork` c `type: SRIOVVirtualFunction`.

## 4. Подготовка ноды

Ниже опираюсь на приложенный файл
`/Users/myskat_90/Downloads/Настройка Mellanox и Nvidia на Astra (1) (1).docx`.
Он полезен как локальный operational note, но не как vendor contract. Версии и
пакеты из него надо адаптировать под ваш образ/ядро.

### 4.1. ОС и базовое обновление

На Astra 1.8:

1. Обновить пакетную базу и систему.
2. Перезагрузить node.

### 4.2. NVIDIA driver + CUDA

Цель:

- рабочий CUDA driver на host;
- `nvidia-smi` на ноде должен быть зелёным до Kubernetes.

По приложенной заметке коллег:

- у них рабочий путь был через `.run` installer CUDA/NVIDIA;
- repo-based путь на тот момент ломался из-за DKMS/version skew.

Практическое требование для workaround:

- на node должны работать:
  - `nvidia-smi`
  - CUDA runtime
  - GPUDirect-related kernel stack, если вы хотите не просто RDMA, а именно
    GPUDirect RDMA.

### 4.3. Blacklist `nouveau`

До production rollout:

- выключить `nouveau`;
- пересобрать initramfs;
- перезагрузить node.

### 4.4. Mellanox OFED / RDMA stack

По заметке коллег:

- они ставили `MLNX_OFED` под Debian/Astra-compatible baseline;
- после установки проверяли:
  - `ibstatus`
  - `ibv_devinfo`
  - `ibdev2netdev`

Практическое требование для workaround:

- `ibv_devinfo` должен видеть HCA;
- `ibdev2netdev` должен однозначно сопоставлять `mlx5_X` и Linux interface;
- если нужен RoCE-v2, карта должна быть в Ethernet mode.

### 4.5. Переключение Mellanox в Ethernet mode для RoCE

Если вы идёте именно в RoCE-v2:

- проверьте текущий link type;
- при необходимости переключите `LINK_TYPE_P* = 2` и перезагрузите карту/нодy.

Если у вас настоящая InfiniBand fabric, этот шаг не нужен.

### 4.6. IP на RDMA interface

До Kubernetes убедитесь, что сама node умеет ходить по этому интерфейсу:

- поднимите IP на RDMA/RoCE interface;
- проверьте L2/L3 reachability между нодами.

### 4.7. QoS/PFC/ECN

Для RoCE это не optional tuning, а часть работоспособности.

По заметке коллег:

- они настраивали:
  - `mlnx_qos --trust-dscp`
  - PFC по нужному приоритету
  - `traffic_class`
  - `rdma_cm` TOS
- и проверяли:
  - `ethtool -S <if> | grep prio`
  - `rdma statistic`
  - `mlnx_qos -a`

Практическое требование:

- согласовать QoS profile с сетевой командой и коммутатором;
- не копировать blindly DSCP/PFC values из заметки;
- использовать их только если ваш fabric настроен точно так же.

### 4.8. GPUDirect RDMA prerequisites

Если цель именно GPUDirect RDMA, а не просто RDMA:

- проверьте `gdscheck -p`;
- проверьте peer direct stack;
- по заметке коллег они дополнительно собирали `gdrcopy`.

Важно:

- отключение IOMMU/ACS из заметки возможно даст performance benefit, но это
  platform-wide decision с security/operability последствиями;
- делать это только как отдельное согласованное решение, не как “по умолчанию”.

### 4.9. Node-level sanity before Kubernetes

Минимальный чек:

- `nvidia-smi`
- `ibv_devinfo`
- `ibdev2netdev`
- `ib_read_bw` / `ib_read_lat` или эквивалентный perftest

Если это не работает на host, идти в Kubernetes рано.

## 5. Kubernetes prerequisites

### 5.1. GPU exposure

На кластере нужен рабочий NVIDIA Kubernetes stack.

Практически:

- либо NVIDIA GPU Operator;
- либо минимум NVIDIA device plugin.

Если драйвер уже стоит на хосте, GPU Operator можно разворачивать в режиме без
host driver management. NVIDIA прямо поддерживает pre-installed driver
scenario:

- <https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html>
- <https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/>

### 5.2. KubeRay

Для Ray на Kubernetes upstream рекомендует KubeRay:

- <https://docs.ray.io/en/latest/cluster/kubernetes/>

Для worker groups используйте pod template, nodeSelector и tolerations:

- <https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html>

Ray docs также рекомендуют pattern “one large Ray pod per node”, что идеально
совпадает с этим workaround:

- <https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html>

### 5.3. Kubelet resource managers

Для latency-sensitive GPU/RDMA workloads рекомендованы:

- Topology Manager policy `single-numa-node` или как минимум `restricted`;
- CPU Manager policy `static` для Guaranteed pods;
- при необходимости Memory Manager.

Официальные ссылки:

- <https://kubernetes.io/docs/tasks/administer-cluster/topology-manager/>
- <https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/>
- <https://kubernetes.io/docs/tasks/administer-cluster/memory-manager/>

HugePages для этого workaround не являются обязательными. Они критичны скорее
для DPDK workloads, а не для NCCL/vLLM-over-RDMA.

## 6. Настройка `sdn`

### 6.1. Подготовьте node labels

На всех RDMA/GPU worker nodes:

```bash
kubectl label node <node> workload=ray-vllm-rdma
```

### 6.2. Убедитесь, что `NodeNetworkInterface` видит Mellanox port

Например:

```bash
kubectl get nni -l network.deckhouse.io/nic-pci-type=PF
kubectl get nni <name> -o yaml
```

Нужный интерфейс должен быть:

- PF;
- на нужной node;
- с Mellanox vendor;
- с правильным `ifName`.

### 6.3. Пометьте интерфейсы под RDMA workstream

Пример:

```bash
kubectl label nni worker-01-nic-0000:17:00.0 nic-group=rdma
kubectl label nni worker-02-nic-0000:17:00.0 nic-group=rdma
```

### 6.4. Создайте `UnderlayNetwork`

Рекомендуемый вариант для workaround:

- `mode: Dedicated`
- один выделенный PF per node

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: UnderlayNetwork
metadata:
  name: ray-rdma-underlay
spec:
  mode: Dedicated
  autoBonding: false
  memberNodeNetworkInterfaces:
    - labelSelector:
        matchLabels:
          nic-group: rdma
```

Проверка:

```bash
kubectl get underlaynetwork ray-rdma-underlay -o yaml
```

### 6.5. Создайте `ClusterIPAddressPool`

Под node-level RDMA subnet:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterIPAddressPool
metadata:
  name: ray-rdma-pool
spec:
  leaseTTL: 24h
  pools:
    - network: 198.18.0.0/24
      ranges:
        - 198.18.0.101-198.18.0.199
```

### 6.6. Создайте `SystemNetwork`

Для первого прохода:

- `type: Access`
- IPAM от `ClusterIPAddressPool`

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: SystemNetwork
metadata:
  name: ray-rdma-system
spec:
  type: Access
  underlayNetworkName: ray-rdma-underlay
  ipam:
    clusterIPAddressPoolName: ray-rdma-pool
```

Проверка:

```bash
kubectl get systemnetwork ray-rdma-system -o yaml
kubectl get nodenetworkstatus -A -o yaml
```

Что должно получиться:

- на каждой worker node есть host-side interface в RDMA subnet;
- у него есть IP из `ray-rdma-pool`;
- fabric reachability между nodes проходит.

## 7. Контейнерный образ для Ray + vLLM

Для KubeRay image должен:

- содержать ту же версию Ray, что и `spec.rayVersion`;
- содержать `wget` для новых KubeRay operator versions;
- содержать vLLM;
- содержать userspace RDMA libs.

KubeRay требования к образу:

- <https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html>

Минимальная идея Dockerfile:

```dockerfile
FROM vllm/vllm-openai:latest

RUN apt-get update && apt-get install -y \
    wget \
    rdma-core \
    ibverbs-providers \
    infiniband-diags \
    && rm -rf /var/lib/apt/lists/*

RUN pip install "ray[default]==2.54.0"
```

Подберите версии Ray/vLLM под ваш baseline. Не смешивайте случайные nightly.

## 8. `RayCluster` workaround

### 8.1. Принципы

- head pod:
  - без GPU;
  - без `hostNetwork`;
  - живёт на обычной cluster network.
- worker pods:
  - GPU nodes only;
  - `hostNetwork: true`;
  - `dnsPolicy: ClusterFirstWithHostNet`;
  - `podAntiAffinity` на hostname;
  - один pod на node.

### 8.2. Важные env vars

По vLLM docs:

- сетевые env надо задавать при создании кластера, чтобы они попали на все nodes;
- при множественных IP надо явно выровнять выбор IP.

Ссылки:

- <https://docs.vllm.ai/en/v0.14.0/serving/parallelism_scaling/>
- <https://docs.vllm.ai/en/v0.10.1/serving/distributed_troubleshooting.html>

Для workaround:

- `VLLM_HOST_IP`: взять из `status.podIP`
  - для `hostNetwork: true` это node primary IP;
  - это нужно, чтобы vLLM видел тот же IP, что и Ray.
- `NCCL_SOCKET_IFNAME`: host RDMA interface, например `ens8np0`
- `GLOO_SOCKET_IFNAME`: тот же interface
- `NCCL_IB_HCA`: `mlx5`
- `NCCL_DEBUG`: сначала `INFO`, при отладке `TRACE`
- `NCCL_IB_GID_INDEX`: задавайте только если реально нужен фиксированный GID
  index для вашего RoCE fabric.

Если names интерфейсов различаются по нодам:

- либо выровнять udev naming;
- либо сделать отдельные worker groups на наборы нод с одинаковыми ifnames.

### 8.3. Пример `RayCluster`

```yaml
apiVersion: ray.io/v1
kind: RayCluster
metadata:
  name: vllm-rdma
spec:
  rayVersion: "2.54.0"
  headGroupSpec:
    rayStartParams:
      dashboard-host: "0.0.0.0"
      num-cpus: "0"
    template:
      spec:
        nodeSelector:
          node-role.kubernetes.io/control-plane: "false"
        containers:
          - name: ray-head
            image: registry.example.com/ray-vllm-rdma:latest
            resources:
              requests:
                cpu: "4"
                memory: "16Gi"
              limits:
                cpu: "4"
                memory: "16Gi"
            ports:
              - containerPort: 6379
                name: gcs
              - containerPort: 8265
                name: dashboard
              - containerPort: 10001
                name: client
              - containerPort: 8000
                name: serve
  workerGroupSpecs:
    - groupName: gpu-rdma
      replicas: 2
      minReplicas: 2
      maxReplicas: 2
      rayStartParams: {}
      template:
        spec:
          hostNetwork: true
          dnsPolicy: ClusterFirstWithHostNet
          nodeSelector:
            workload: ray-vllm-rdma
          tolerations:
            - key: "nvidia.com/gpu"
              operator: "Exists"
              effect: "NoSchedule"
          affinity:
            podAntiAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                - labelSelector:
                    matchExpressions:
                      - key: ray.io/group
                        operator: In
                        values: ["gpu-rdma"]
                  topologyKey: kubernetes.io/hostname
          containers:
            - name: ray-worker
              image: registry.example.com/ray-vllm-rdma:latest
              securityContext:
                capabilities:
                  add: ["IPC_LOCK"]
              env:
                - name: VLLM_HOST_IP
                  valueFrom:
                    fieldRef:
                      fieldPath: status.podIP
                - name: NCCL_SOCKET_IFNAME
                  value: "ens8np0"
                - name: GLOO_SOCKET_IFNAME
                  value: "ens8np0"
                - name: NCCL_IB_HCA
                  value: "mlx5"
                - name: NCCL_DEBUG
                  value: "INFO"
              volumeMounts:
                - name: dshm
                  mountPath: /dev/shm
                - name: infiniband
                  mountPath: /dev/infiniband
                - name: sys-infiniband
                  mountPath: /sys/class/infiniband
                  readOnly: true
              resources:
                requests:
                  cpu: "32"
                  memory: "180Gi"
                  nvidia.com/gpu: "8"
                limits:
                  cpu: "32"
                  memory: "180Gi"
                  nvidia.com/gpu: "8"
          volumes:
            - name: dshm
              emptyDir:
                medium: Memory
                sizeLimit: 16Gi
            - name: infiniband
              hostPath:
                path: /dev/infiniband
                type: Directory
            - name: sys-infiniband
              hostPath:
                path: /sys/class/infiniband
                type: Directory
```

## 9. Запуск vLLM

### 9.1. Сначала проверьте сам Ray cluster

```bash
kubectl get raycluster
kubectl get pods -l ray.io/cluster=vllm-rdma -o wide
```

### 9.2. Проверьте worker pods

Внутри worker pod:

```bash
ls -l /dev/infiniband
ibv_devinfo
env | egrep 'NCCL|GLOO|VLLM'
```

### 9.3. Запустите vLLM с Ray backend

По vLLM docs типичный multi-node pattern такой:

- `tensor_parallel_size = GPUs per node`
- `pipeline_parallel_size = number of nodes`
- `--distributed-executor-backend ray`

Источник:

- <https://docs.vllm.ai/en/v0.14.0/serving/parallelism_scaling/>

Пример запуска с head pod:

```bash
kubectl exec -it <ray-head-pod> -- bash

vllm serve /models/<model> \
  --tensor-parallel-size 8 \
  --pipeline-parallel-size 2 \
  --distributed-executor-backend ray \
  --host 0.0.0.0 \
  --port 8000
```

Если модель из Hugging Face, лучше заранее положить её локально на все nodes
или на shared storage. vLLM сам это рекомендует для multi-node path.

## 10. Как понять, что RDMA реально используется

### На host

- `ib_read_bw`
- `ib_read_lat`
- `ethtool -S <if> | grep prio`
- `rdma statistic`

### В worker pod

- `ibv_devinfo`
- простой NCCL sanity test

### В логах vLLM/NCCL

По vLLM docs:

- если видите `NET/IB/GDRDMA`, значит работает InfiniBand/GPUDirect RDMA path;
- если видите `NET/Socket`, значит вы ушли в обычный TCP.

Источник:

- <https://docs.vllm.ai/en/v0.14.0/serving/parallelism_scaling/>

## 11. Что, скорее всего, пойдёт не так

### Проблема 1. В логах только `NET/Socket`

Проверяйте:

- host OFED/verbs stack;
- QoS/PFC/ECN;
- что `NCCL_SOCKET_IFNAME` указывает именно на RDMA interface;
- что `/dev/infiniband` реально смонтирован;
- что контейнерный образ содержит userspace RDMA libs.

### Проблема 2. vLLM и Ray не сходятся по IP

По vLLM docs это типично при multi-IP hosts.

В этом workaround:

- не пытайтесь дать Ray data-plane IP как pod IP;
- оставьте `VLLM_HOST_IP=status.podIP`, чтобы vLLM совпадал с Ray;
- data-plane переводите на RDMA interface через `NCCL_SOCKET_IFNAME` /
  `GLOO_SOCKET_IFNAME`.

### Проблема 3. Worker pod на hostNetwork видит не тот интерфейс

Нужно:

- одинаковое naming across nodes;
- либо отдельные workerGroupSpecs per NIC naming profile.

### Проблема 4. GPUDirect RDMA не включается

Смотрите:

- host driver stack;
- `gdscheck -p`;
- IOMMU/ACS policy;
- NUMA placement GPU vs NIC.

## 12. Что делать дальше, если workaround подтвердится

Если этот обходной путь заработает, следующий нормальный engineering step:

1. либо добавить в `sdn` explicit pod-level RDMA contract:
   - отдельный binding mode или explicit Mellanox verbs mode;
   - device node contract;
   - pod-level IPAM for underlay;
2. либо спроектировать более чистую GPU+NIC pairing модель.

Но для **сегодняшней** задачи этот workaround должен быть первым вариантом
проверки.

## 13. Краткий verdict

### Что рекомендую делать прямо сейчас

- поднять host RDMA/RoCE на нодах по заметке коллег;
- в Kubernetes использовать:
  - `UnderlayNetwork` + `SystemNetwork(type=Access)` для node-level RDMA сети;
  - `hostNetwork` KubeRay workers;
  - `/dev/infiniband` + `/dev/shm` + `IPC_LOCK`;
  - `NCCL_SOCKET_IFNAME` / `GLOO_SOCKET_IFNAME` / `NCCL_IB_HCA`.

### Что не рекомендую делать как основной путь

- сразу тащить `UnderlayNetwork` напрямую в worker pod ради RoCE data-plane;
- строить решение вокруг scheduler extender;
- начинать с GPUDirect tuning до того, как host RDMA и NCCL over IB уже
  подтверждены.
