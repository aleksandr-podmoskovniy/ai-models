# Findings

## 1. Как `sdn` сейчас пробрасывает устройство в pod

Текущий underlay flow в `sdn` выглядит так:

1. Пользователь указывает в pod annotation `network.deckhouse.io/networks-spec`
   объект `{"type":"UnderlayNetwork","name":"..."}`.
2. Admission webhook добавляет в pod `spec.resourceClaims` и
   `containers[].resources.claims` для соответствующего `ResourceClaimTemplate`.
3. `UnderlayNetwork` controller создаёт:
   - `DeviceClass` с CEL selector по `underlayNetwork`;
   - `ResourceClaimTemplate` с `deviceClassName=d8-sdn-<underlay>`.
4. Node-local DRA controller публикует `resource.k8s.io` devices из matched PF/VF
   в `ResourceSlice`.
5. DRA kubelet plugin в `PrepareResourceClaims`:
   - находит выбранные `NodeNetworkInterface`;
   - валидирует `bindingMode`;
   - переключает binding через `interface-syncer`;
   - формирует CDI spec;
   - либо сохраняет VFIO allocation, либо netdev handoff для CNI.
6. CNI-side reconciler:
   - для `NetDev`/Mellanox `DPDK` двигает netdev в pod netns;
   - для userspace/VFIO path пишет status annotation с VFIO metadata.

Ключевой вывод: scheduler extender `sdn` сейчас не участвует в выборе
`UnderlayNetwork`. Он фильтрует обычные `Network`-attachments и прямо исключает
`UnderlayNetwork` из логики. Для underlay selection проект уже опирается на
native DRA scheduling.

## 2. Что уже есть для RDMA

`sdn` нативно не имеет отдельного `bindingMode: RDMA`, но для Mellanox уже есть
важный baseline:

- `bindingMode: DPDK` для Mellanox не переводит устройство в `vfio-pci`, а
  оставляет `mlx5_core`;
- в pod попадает сам netdev;
- в CDI уже монтируются `uverbs` device files из `/dev/infiniband/*`;
- в pod монтируется PCI sysfs path устройства.

Это означает:

- для Mellanox текущий `DPDK` режим уже ближе к userspace RDMA/verbs path, чем к
  классическому VFIO-only DPDK path;
- попытка сделать RDMA через `vfio-pci` для Mellanox была бы неверным default:
  VFIO полезен для userspace PCI ownership, но не является естественным способом
  дать контейнеру verbs/RDMA CM interface.

## 3. Что не хватало и что я изменил

Для verbs-based workloads на Mellanox часто нужен не только `uverbsX`, но и
глобальный control device `rdma_cm`.

В этом срезе я сделал bounded prototype в `sdn`:

- [`handlers.go`](/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn/images/agent/src/internal/dra-plugin/driver/handlers.go:263)
  теперь оппортунистически добавляет `/dev/infiniband/rdma_cm` в CDI device
  nodes, если:
  - claim уже использует InfiniBand verbs files;
  - такой device существует на host.
- Обновил краткое описание режима в
  [docs/README.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn/docs/README.md:63)
  и
  [docs/README.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn/docs/README.ru.md:63),
  чтобы зафиксировать Mellanox `DPDK` path как experimental RDMA/verbs-capable
  baseline.

Это не вводит новый API и не ломает текущую модель `NetDev` / `VFIO-PCI` /
`DPDK`.

## 4. Почему я не стал вводить отдельный `bindingMode: RDMA`

Отдельный RDMA mode уже не является "маленькой" правкой. Он потребовал бы:

- расширить API/CRD enum;
- договориться, чем `RDMA` семантически отличается от Mellanox `DPDK`, если
  реальный kernel driver в обоих случаях `mlx5_core`;
- научиться стабильно отражать этот режим в status/discovery, а не только в
  desired spec;
- определить exact mount contract:
  - только `uverbs` + `rdma_cm`;
  - или ещё `issm/umad`;
  - нужен ли отдельный sysfs/class contract.

Итого: отдельный `RDMA` binding mode я считаю отдельным follow-up workstream, а
не безопасным incidental change.

## 5. Практический verdict по RDMA

Если цель — как можно быстрее проверить RDMA на текущем `sdn`, то defendable
путь такой:

- Mellanox only;
- `UnderlayNetwork` + `bindingMode: DPDK`;
- `Shared` mode для VF-based isolation или `Dedicated` для целого PF;
- текущий prototype даёт `uverbs*` + `rdma_cm` + netdev + sysfs;
- на узлах отдельно должны быть готовы RDMA stack / OFED prerequisites.

Если цель — GPUDirect RDMA, этого недостаточно само по себе. Нужны ещё:

- совместимая GPU/NIC topology;
- host-side GPU driver integration (`nvidia-peermem` / equivalent stack);
- корректный RDMA driver stack на host.

## 6. Что делать для будущего `GPU + RDMA NIC` matching

### Базовый вывод

Чистый DRA хорошо решает:

- публикацию devices;
- per-pod claims;
- device filtering по attributes/CEL;
- fallback selection через prioritized subrequests;
- логические/composite devices через partitionable devices.

Но если GPU и NIC остаются двумя независимыми драйверами/классами, то одного
DRA недостаточно, чтобы надёжно гарантировать "эта GPU должна ехать именно с
этим NIC по topology/pairing policy".

### Что я рекомендую

Предпочтительный порядок решений:

1. Сначала попытаться выразить pairing внутри resource model, а не во внешнем
   extender.
   - Вариант A: один higher-level driver публикует composite `GPU+NIC bundle`
     devices.
   - Вариант B: один coordinating allocator формирует logical devices/claims
     поверх двух inventories.
2. Если этого недостаточно, использовать не scheduler extender, а
   scheduler framework plugin или отдельный secondary scheduler.
3. Topology Manager на узле включать всё равно:
   - `pod` scope;
   - `single-numa-node` или как минимум `restricted`, если workload реально
     latency-sensitive.

### Почему не extender-first

- upstream scheduler extender умеет только filter/prioritize;
- reserve / prebind / richer in-tree extension points через webhook недоступны;
- если логика паринга GPU+NIC станет stateful и topology-heavy, extender быстро
  упрётся в собственные границы.

## 7. Market / upstream references

### Kubernetes upstream

- DRA terminology, `DeviceClass`, `ResourceClaimTemplate`, `ResourceSlice`, CEL
  selectors и per-pod claims:
  <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
- DRA prioritized subrequests и node scoring by higher-ranked alternative:
  <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
- DRA partitionable/logical devices:
  <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
- Scheduler extension guidance: scheduler framework / multiple schedulers /
  extenders only filter+prioritize:
  <https://kubernetes.io/docs/concepts/extend-kubernetes>
  <https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/>
- Topology Manager `pod` scope and `single-numa-node` / `restricted` policies:
  <https://kubernetes.io/docs/tasks/administer-cluster/topology-manager/>

### NVIDIA references

- GPU Operator DRA driver and higher-level `ComputeDomain` abstraction:
  <https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/dra-intro-install.html>
  <https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/25.10/dra-cds.html>
- Network Operator as current RDMA market baseline:
  shared RDMA device plugin, SR-IOV, secondary networks, exclusive RDMA mode:
  <https://docs.nvidia.com/networking/display/kubernetes2610/deployment-guide-kubernetes.html>
- GPUDirect RDMA host-side prerequisites and `nvidia-peermem`:
  <https://docs.nvidia.com/cuda/gpudirect-rdma/>
- Dynamo / KAI Scheduler / topology-aware scheduling as reference for
  topology-driven AI placement:
  <https://docs.nvidia.com/dynamo/dev/kubernetes-deployment/multinode/topology-aware-scheduling>

### OpenShift / Red Hat reference

- NUMA Resources Operator / secondary NUMA-aware scheduler / NodeResourceTopology:
  <https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/scalability_and_performance/cnf-numa-aware-scheduling>

## 8. Recommended next bundle

Если продолжать эту тему в `sdn`, следующий bounded bundle должен быть уже не
research, а implementation-oriented:

1. Зафиксировать exact experimental RDMA contract для Mellanox:
   - required device nodes;
   - required sysfs;
   - example pod manifest.
2. Добавить explicit e2e/smoke path:
   - verbs container without GPU;
   - optional GPUDirect validation on prepared hardware.
3. Отдельно решить архитектурно:
   - composite `GPU+NIC` DRA driver;
   - или scheduler framework / secondary scheduler поверх topology inventory.
