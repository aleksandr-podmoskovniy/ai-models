# RDMA Validation Toolkit

Это bundle-local build context для self-service smoke container. Цель простая:

- не собирать `perftest` вручную в каждом debug pod;
- получить один образ, где уже есть:
  - `iperf3`
  - `sockperf`
  - `ib_write_bw`
  - `ib_send_lat`
  - CUDA-aware `perftest`
- запускать одинаковый smoke для:
  - TCP throughput/latency
  - plain RDMA throughput/latency
  - `GPUDirect RDMA` throughput

## Что внутри

- [Containerfile](./Containerfile)
  - базируется на `nvidia/cuda:12.2.2-devel-ubuntu22.04`
  - ставит network/RDMA userspace tools
  - собирает `perftest 24.10.0-0.66` с `CUDA_H_PATH=/usr/local/cuda/include/cuda.h`
- [bin/rdma-validate](./bin/rdma-validate)
  - один entrypoint/runner
  - умеет inventory, IP bind/cleanup, TCP/RDMA/GPUDirect modes
  - пишет raw logs и `results.jsonl`
- [manifests/rdma-validation-pod.yaml](./manifests/rdma-validation-pod.yaml)
  - минимальный plain RDMA pod template
- [manifests/gpudirect-validation-pod.yaml](./manifests/gpudirect-validation-pod.yaml)
  - pod template для `GPU + RDMA`

## Как собрать образ

Из директории toolkit:

```bash
cd plans/active/research-sdn-underlay-rdma-dra-gpu-placement/toolkit/rdma-validation-toolkit

docker build -t <registry>/rdma-validation-toolkit:0.2.0 -f Containerfile .
```

Или через `podman`:

```bash
podman build -t <registry>/rdma-validation-toolkit:0.2.0 -f Containerfile .
```

Потом запушить:

```bash
docker push <registry>/rdma-validation-toolkit:0.2.0
```

## Как использовать внутри pod

### 1. Inventory

```bash
rdma-validate inventory
```

Что это даёт:

- `ip -br link`
- `ip -br addr`
- `/dev/infiniband`
- `ibv_devinfo`
- `nvidia-smi -L`
- наличие CUDA flags у `perftest`
- `rdma link show`

### 2. Назначить временный direct IP

```bash
export DIRECT_IFACE=enp193s0np0
export DIRECT_IP=172.31.141.1/30

rdma-validate bind-ip
```

После smoke:

```bash
rdma-validate cleanup-ip
```

### 3. TCP throughput

Server:

```bash
export DIRECT_IP=172.31.141.1/30
rdma-validate tcp-bw-server
```

Client:

```bash
export DIRECT_IP=172.31.141.2/30
export PEER_IP=172.31.141.1
rdma-validate tcp-bw-client
```

Если нужен более агрессивный non-RDMA baseline:

```bash
export TCP_STREAMS=8
export DURATION=30
rdma-validate tcp-bw-client
```

### 4. TCP latency

Server:

```bash
export DIRECT_IP=172.31.141.1/30
rdma-validate tcp-lat-server
```

Client:

```bash
export DIRECT_IP=172.31.141.2/30
export PEER_IP=172.31.141.1
rdma-validate tcp-lat-client
```

RTT variant:

```bash
export DIRECT_IP=172.31.141.2/30
export PEER_IP=172.31.141.1
rdma-validate tcp-rtt-client
```

### 5. RDMA throughput

Server:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=3
rdma-validate rdma-bw-server
```

Client:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=3
export PEER_IP=172.31.141.1
rdma-validate rdma-bw-client
```

Для line-rate smoke имеет смысл явно держать `QPS`, `SIZE` и `MTU`:

```bash
export QPS=8
export SIZE=65536
export MTU=4096
rdma-validate rdma-bw-client
```

### 6. RDMA latency

Server:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=3
rdma-validate rdma-lat-server
```

Client:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=3
export PEER_IP=172.31.141.1
rdma-validate rdma-lat-client
```

### 7. GPUDirect RDMA throughput

Server:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=3
export GPU_INDEX=0
rdma-validate gpudirect-bw-server
```

Client:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=3
export PEER_IP=172.31.130.1
export GPU_INDEX=0
rdma-validate gpudirect-bw-client
```

Если надо жёстко прибить к конкретной карте:

```bash
export GPU_BUS_ID=00000000:C2:00.0
rdma-validate gpudirect-bw-client
```

## Сводка и автоматические пороги

После серии client-side прогонов toolkit может собрать короткую сводку:

```bash
rdma-validate report
```

В summary будут:

- `tcp_bw_gbps`
- `tcp_latency_usec`
- `tcp_rtt_usec`
- `rdma_bw_gbps`
- `rdma_latency_usec`
- `rdma_over_tcp_ratio`
- `gpudirect_bw_gbps`
- `gpudirect_over_tcp_ratio`
- `gpudirect_cuda_used`
- `gpudirect_gpu_name`

Если нужен automated smoke с non-zero exit:

```bash
export MIN_TCP_BW_GBPS=10
export MIN_RDMA_BW_GBPS=80
export MAX_TCP_LAT_USEC=80
export MAX_RDMA_LAT_USEC=10
export MIN_RDMA_OVER_TCP_RATIO=3
export MIN_GPUDIRECT_BW_GBPS=50

rdma-validate check
```

`check` завершится `RC=1`, если пороги не выполняются или если для
`GPUDirect` в raw `perftest` output не найдены явные CUDA markers.

## Логи и summary

Toolkit пишет:

- raw logs: `${LOG_DIR:-/tmp/rdma-validation}/*.log`
- json summaries: `${LOG_DIR:-/tmp/rdma-validation}/results.jsonl`

Пример:

```json
{"ts":"2026-04-15T07:00:00+03:00","toolkit_version":"0.2.0","test_name":"rdma_bw","mode":"client","direct_iface":"enp193s0np0","direct_ip":"172.31.141.2/30","peer_ip":"172.31.141.1","rdma_dev":"mlx5_0","gid_index":3,"summary":{"unit":"Gb/sec","average_gbps":"97.24","msg_rate":"0.185469","msg_rate_unit":"Mpps"}}
```

Это удобно для:

- ручного чтения;
- артефактов в CI;
- простого `jq`/`grep` парсинга;
- `report/check` без отдельного glue-кода.

## Что этот toolkit ловит

Он помогает быстро отделить классы проблем:

- `inventory` падает:
  - нет devices / нет CUDA / нет verbs userspace
- `tcp-*` идут, а `rdma-*` нет:
  - проблема в verbs/RDMA path
- plain `rdma-*` идут, а `gpudirect-*` нет:
  - проблема уже в `GPU memory` path:
    `nvidia-peermem`, CUDA-aware `perftest`, DRA GPU claim, topology

## Что toolkit сознательно не делает

- не orchestrates server/client автоматически между двумя pod;
- не назначает `GID index` сам;
- не создаёт Kubernetes objects;
- не подменяет полноценный e2e harness.

Его задача уже проще:

- дать один стандартный image;
- дать один стандартный CLI;
- привести smoke к одному operational contract;
- дать минимальный pass/fail guardrail для regression smoke.
