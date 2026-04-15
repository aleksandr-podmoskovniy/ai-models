# Recommended GPU/RDMA pairing for KubeRay on `k8s-dvp.apiac.ru`

## Goal

Нужно получить не просто 3 RDMA-capable pod, а 3 worker group для
распределённого inference так, чтобы у каждого worker была:

- одна конкретная GPU;
- одна конкретная Mellanox NIC;
- максимально локальная PCI topology для будущего GPUDirect RDMA.

## Why one shared `UnderlayNetwork` is not enough

Один общий `UnderlayNetwork` с несколькими PF работает для сценария
“дай любую свободную карту”, но не даёт жёсткого `GPU -> NIC` pairing.

Причина:

- GPU DRA и network DRA сейчас независимы;
- один generic GPU class и один generic network class не гарантируют, что
  scheduler/allocator выберет локальную пару;
- на `w1` это может случайно дать либо хорошую пару, либо cross-root pair.

Для обычного RDMA smoke это терпимо. Для GPUDirect RDMA и distributed inference
так делать не стоит.

## Actual topology on this cluster

### `k8s-dvp-w1-gpu.apiac.ru`

Host `lspci -tv` и `nvidia-smi topo -m` показывают:

- root complex `0000:80`
  - GPU `00000000:81:00.0`
  - NIC `0000:82:00.0` / `enp130s0np0` / `mlx5_1`
- root complex `0000:c0`
  - GPU `00000000:c2:00.0`
  - NIC `0000:c1:00.0` / `enp193s0np0` / `mlx5_0`

Topology quality:

- `GPU 81:00.0 <-> NIC 82:00.0` = `PHB`
- `GPU c2:00.0 <-> NIC c1:00.0` = `PHB`
- cross pairs on `w1` = `SYS`

Практический вывод:

- хороших pair на `w1` ровно две;
- их надо закреплять явно.

### `k8s-dvp-w3-gpu.apiac.ru`

Host `lspci -tv` и `nvidia-smi topo -m` показывают:

- root complex `0000:00`
  - GPU `00000000:01:00.0`
  - NIC `0000:02:00.0` / `enp2s0np0` / `mlx5_0`

Topology quality:

- `GPU 01:00.0 <-> NIC 02:00.0` = `PHB`

## Recommended current design

Для текущего baseline лучше сделать не один общий underlay, а 3 фиксированные
пары:

1. `w1/root80`
   - GPU `81:00.0`
   - NIC `82:00.0`
2. `w1/rootc0`
   - GPU `c2:00.0`
   - NIC `c1:00.0`
3. `w3/root00`
   - GPU `01:00.0`
   - NIC `02:00.0`

Технически это выражается так:

- 3 отдельных `UnderlayNetwork`, каждый выбирает ровно одну PF;
- 3 отдельных manual `ResourceClaimTemplate` для сети;
- 3 отдельных GPU `DeviceClass`, каждый выбирает ровно одну GPU;
- 3 worker group в `RayCluster`, каждый с `replicas: 1`.

## Current live network implementation

В тестовом контуре уже переведено на 3 отдельные network pair:

- `rdma-w1-pair80` -> `enp130s0np0` / `mlx5_1`
- `rdma-w1-pairc0` -> `enp193s0np0` / `mlx5_0`
- `rdma-w3-pair00` -> `enp2s0np0` / `mlx5_0`

Подтверждённые pod:

- `rdma-pod-w1-80` -> `enp130s0np0`, `mlx5_1`
- `rdma-pod-w1-c0` -> `enp193s0np0`, `mlx5_0`
- `rdma-pod-w3-00` -> `enp2s0np0`, `mlx5_0`

То есть deterministic NIC mapping через отдельные underlay уже работает.

## Recommended GPU classes

С учётом текущего GPU DRA логичнее делать classes по конкретным physical GPU, а
не один общий `nvidia-v100-physical`.

Рабочая идея:

- `nvidia-v100-w1-81`
  - selector by `gpu.deckhouse.io/pciAddress == "00000000:81:00.0"`
- `nvidia-v100-w1-c2`
  - selector by `gpu.deckhouse.io/pciAddress == "00000000:c2:00.0"`
- `nvidia-v100-w3-01`
  - selector by `gpu.deckhouse.io/pciAddress == "00000000:01:00.0"`

Это даёт детерминированность уже на уровне GPU allocation.

Дополнительно worker template всё равно должен быть pinned на нужную node:

- `w1-80` group -> `k8s-dvp-w1-gpu.apiac.ru`
- `w1-c0` group -> `k8s-dvp-w1-gpu.apiac.ru`
- `w3-00` group -> `k8s-dvp-w3-gpu.apiac.ru`

## Recommended KubeRay layout

Лучший текущий operational вариант:

- 1 head pod без RDMA NIC;
- 3 worker group с `replicas: 1`:
  - `worker-w1-80`
  - `worker-w1-c0`
  - `worker-w3-00`

Для каждого worker group:

- свой `nodeSelector`;
- свой GPU class / claim template;
- свой `UnderlayNetwork` в `network.deckhouse.io/networks-spec`;
- свои env для RDMA/NCCL;
- позже свой `runtimeClass`/security tuning если понадобится.

## Why this is better than “3 groups over one shared underlay”

Потому что такая схема:

- детерминирована;
- воспроизводима;
- уже сейчас реализуема текущими модулями без scheduler extender;
- соответствует реальной PCI topology;
- не требует угадывать, какую карту выберет network allocator на `w1`.

## Limitation of the current approach

Это всё ещё static pairing, а не настоящая topology-aware composite allocation.

Ограничения:

- много ручных сущностей;
- scale-out beyond fixed known pairs неудобен;
- GPU и NIC всё ещё аллоцируются разными драйверами;
- system не умеет сам выбрать “лучшую локальную пару” из абстрактного пула.

## Better long-term design

Для будущего generalized solution лучше уходить в одну из двух сторон:

1. composite DRA allocation для `GPU+NIC` pair
2. topology-aware scheduler/plugin, который учитывает GPU и NIC одновременно

Но это уже следующий workstream.

## Short verdict

Для текущей цели:

- да, `3 underlay по 1 карте` — это лучше, чем один общий underlay;
- да, надо делать 3 GPU class и 3 worker group;
- это сейчас самый практичный способ обеспечить стабильный root-complex-aware
  layout и сохранить рабочий RDMA path.
