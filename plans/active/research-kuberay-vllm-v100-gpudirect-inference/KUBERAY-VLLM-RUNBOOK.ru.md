# KubeRay + vLLM + GPUDirect RDMA на `k8s-dvp`

## Назначение

Документ покрывает:

- раскатку `KubeRay operator` и `RayService` через `Argo CD`;
- запуск `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B` на `3x V100 32GB`;
- текущий pod-level `RDMA/GPUDirect` профиль на `k8s-dvp`;
- live-проверки `Ray`, `Serve`, OpenAI-compatible API и `RDMA`;
- типовые проблемы текущего стенда.

Это не низкоуровневая инструкция по подготовке `RED OS 8` с нуля. Для host-side
`RDMA`, `nvidia-peermem`, `perftest` и pod-level smoke уже есть отдельный
предыдущий документ:

- `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`

Текущий документ начинается с точки, в которой базовый `RDMA/GPUDirect RDMA`
на стенде уже доведён до рабочего состояния.

## Область применимости

Проверенная прикладная конфигурация:

- cluster: `k8s-dvp.apiac.ru`
- namespace для сервиса: `kuberay-projects`
- namespace для оператора: `kuberay-operator`
- `KubeRay` operator через `Argo CD`
- образ: `rayproject/ray-llm:nightly.260418.64385a-py311-cu128`
- `Ray 2.55.x` lineage
- `vLLM >= 0.19.0`
- модель: `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`
- `dtype=half`
- `tensor_parallel_size=1`
- `pipeline_parallel_size=3`
- `max_model_len=8192`
- `max_num_seqs=1`
- `reasoning_parser=deepseek_r1`
- storage class: `ceph-fs-nvme-sc`
- ingress host: `openai-api.k8s-dvp.apiac.ru`

## Как выглядел проверенный стенд

### Кластер и приложения

| Что | Значение |
| --- | --- |
| `Argo CD` app для CRD | `kuberay-operator-crds` |
| `Argo CD` app для оператора | `kuberay-operator` |
| `memlock` на GPU-нодах | `apply-containerd-memlock.sh` |
| `Argo CD` app для сервиса | `kuberay-service-v100-rdma` |
| `RayService` | `llm-v100-rdma-deepseek-r1-qwen14b` |
| стабильный API service | `llm-v100-rdma-deepseek-r1-qwen14b-serve-svc` |
| внешний API host | `openai-api.k8s-dvp.apiac.ru` |

### Три worker с `GPU + RDMA`

| Worker group | Узел | GPU claim | Underlay | Direct iface | RDMA device |
| --- | --- | --- | --- | --- | --- |
| `v100-w1-80` | `k8s-dvp-w1-gpu.apiac.ru` | `gpu-v100-w1-81-gpudirect` | `rdma-w1-pair80` | `enp130s0np0` | `mlx5_1` |
| `v100-w1-c2` | `k8s-dvp-w1-gpu.apiac.ru` | `gpu-v100-w1-c2-gpudirect` | `rdma-w1-pairc0` | `enp193s0np0` | `mlx5_0` |
| `v100-w3-01` | `k8s-dvp-w3-gpu.apiac.ru` | `gpu-v100-w3-01-gpudirect` | `rdma-w3-pair00` | `enp2s0np0` | `mlx5_0` |

### Прямая сеть для `RDMA/NCCL`

| Worker | IP на underlay | `GID index` |
| --- | --- | --- |
| `v100-w1-80` | `172.31.140.1/29` | `3` |
| `v100-w3-01` | `172.31.140.2/29` | `3` |
| `v100-w1-c2` | `172.31.140.3/29` | `3` |

Практически важна следующая модель сети:

- socket bootstrap для `Ray`, `Gloo` и control path идёт через `eth0`;
- `NCCL` data path идёт через `NET/IB` на `mlx5_*`;
- `RoCE v2 GID index` прибит к `3`.

## Где лежит source of truth

Основной GitOps-каталог живёт вне репозитория `ai-models`:

```text
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay
```

Ключевые файлы:

- `argo-app/01-helm-kuberay-operator-crds.yaml`
- `argo-app/02-helm-kuberay-operator.yaml`
- `argo-app/10-kuberay-service-v100-rdma.yaml`
- `apply-containerd-memlock.sh`
- `charts/ray-service-v100-rdma/20-v100-rdma-deepseek-r1-qwen14b-rayservice.yaml`
- `charts/ray-service-v100-rdma/12-api-ingress.yaml`
- `charts/ray-service-v100-rdma/03-hf-secret.yaml.example`
- `charts/ray-service-v100-rdma/README.ru.md`

## Что должно существовать до раскатки

### Cluster-scoped объекты

До запуска сервиса в кластере уже должны существовать:

- `UnderlayNetwork`:
  - `rdma-w1-pair80`
  - `rdma-w1-pairc0`
  - `rdma-w3-pair00`
- `DeviceClass`:
  - `nvidia-v100-w1-81-gpudirect`
  - `nvidia-v100-w1-c2-gpudirect`
  - `nvidia-v100-w3-01-gpudirect`
  - `d8-sdn-rdma-w1-pair80`
  - `d8-sdn-rdma-w1-pairc0`
  - `d8-sdn-rdma-w3-pair00`

Если этот слой ещё не готов, сначала закрывается archived `SKALA`-workstream.

### `memlock` на GPU-нодах

Для текущего `NCCL over RDMA` на `V100` нужно, чтобы worker pod поднимались с
нормальным `RLIMIT_MEMLOCK`. В `dvp` это сейчас intentionally вынесено в
локальный standalone-скрипт, а не в отдельный `Argo CD` app:

```text
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/apply-containerd-memlock.sh
```

После применения новые worker должны показывать:

```bash
ulimit -l
cat /proc/self/limits | grep 'Max locked memory'
```

Исправное состояние:

- `unlimited`
- или эквивалентный высокий лимит без `65536 bytes`.

## Что создаёт manifest-каталог сервиса

`charts/ray-service-v100-rdma` materialize-ит как plain manifests:

- namespace `kuberay-projects`
- `ServiceAccount kuberay-llm`
- `PersistentVolumeClaim model-cache-pvc`
- внешний `Redis` для `Ray` fault tolerance
- namespaced `ResourceClaimTemplate` под текущие `GPU/RDMA` пары
- `RayService llm-v100-rdma-deepseek-r1-qwen14b`
- `Ingress llm-v100-rdma-api`

Имена активных ресурсов выровнены под текущий baseline:

- `RayService llm-v100-rdma-deepseek-r1-qwen14b`
- стабильный сервис `llm-v100-rdma-deepseek-r1-qwen14b-serve-svc`

## Что с operator chart и service manifests

Каталог `charts/kuberay-operator` в `dvp` не является отдельной самодельной
форкой. Официальный chart уже скопирован в `dvp` и дальше обслуживается как
локальный source of truth.

Текущий baseline там:

- `KubeRay operator v1.6.0`
- шаблоны совпадают с официальным каталогом
- отдельный refactor operator chart здесь не нужен

При этом каталог `charts/ray-service-v100-rdma` теперь тоже не helmized:

- `Chart.yaml`, `values.yaml` и `templates/` там убраны;
- `Argo CD` читает его как обычный каталог raw manifests;
- это самостоятельный plain-manifest каталог внутри `dvp`, без внешнего
  `Argo` source path.

## Рабочая последовательность запуска

### Шаг 1. Включить `KubeRay` CRD

Через `Argo CD`:

```bash
argocd app sync kuberay-operator-crds
```

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config kubectl get crd | egrep 'rayservices|rayclusters|rayjobs'
```

Ожидается:

- `rayservices.ray.io`
- `rayclusters.ray.io`
- `rayjobs.ray.io`

### Шаг 2. Включить `KubeRay` operator

```bash
argocd app sync kuberay-operator
```

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config kubectl -n kuberay-operator get deploy,pods
```

Исправное состояние:

- deployment оператора `Available=True`;
- pod оператора `Running`.

### Шаг 3. Поднять `memlock` на GPU-нодах

Выполнить локальный скрипт из `dvp` каталога:

```bash
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/apply-containerd-memlock.sh --ssh
```

### Шаг 4. Подготовить `HF` token

Отредактировать:

```text
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/charts/ray-service-v100-rdma/03-hf-secret.yaml.example
```

И применить уже под реальным именем секрета `hf-secret`.

Ожидаемый объект:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hf-secret
  namespace: kuberay-projects
type: Opaque
stringData:
  HUGGING_FACE_HUB_TOKEN: "<real token>"
```

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config kubectl -n kuberay-projects get secret hf-secret
```

### Шаг 5. Включить сервис

```bash
argocd app sync kuberay-service-v100-rdma
```

Проверка ресурсов:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config kubectl -n kuberay-projects get rayservice,raycluster,pods,pvc,svc,ingress -o wide
```

Исправное состояние:

- `RayService llm-v100-rdma-deepseek-r1-qwen14b` в `Running`;
- `RayCluster ...` в `ready`;
- `head` и три `worker` pod в `Running`;
- `model-cache-pvc` в `Bound`;
- есть стабильный сервис `llm-v100-rdma-deepseek-r1-qwen14b-serve-svc`;
- ingress `llm-v100-rdma-api` существует.

## Что делает `RayService`

Текущий baseline в `serveConfigV2`:

- `model_source: deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`
- `distributed_executor_backend: ray`
- `tensor_parallel_size: 1`
- `pipeline_parallel_size: 3`
- `dtype: half`
- `gpu_memory_utilization: 0.90`
- `max_model_len: 8192`
- `max_num_seqs: 1`

Практический нюанс для reasoning-модели:

- при маленьком `max_tokens` модель может успеть отдать только `reasoning` и
  ещё не дойти до финального `content`;
- на live-проверке с `max_tokens=512` она уже возвращала нормальный
  `content: "4"` и отдельный `reasoning`.

Рабочая разрезка:

- одна Serve replica;
- placement group на три bundle по `CPU:8, GPU:1`;
- один worker pod на каждую `V100`.

## Почему здесь всё ещё есть runtime hotfix

На pinned nightly `rayproject/ray-llm:nightly.260418.64385a-py311-cu128`
`RDMA/NCCL` path уже рабочий, но `vLLM` Ray executor на multi-node `PP=3`
оставляет stale `global_rank` после worker re-rank.

Практический эффект:

- `rpc_rank` после re-rank уже новый;
- `global_rank` остаётся старым;
- `WorkerWrapperBase.initialize_from_config()` выбирает `kv_cache_config`
  по stale `global_rank`;
- `PP` stage получает не свой layer/KV mapping и дальше даёт либо
  `KeyError` по boundary-слоям, либо мусорный output.

Поэтому текущий baseline intentionally оставляет маленький runtime hotfix,
но уже не через `sitecustomize.py` и не через custom image. В worker pod
через `ConfigMap + initContainer + subPath mount` патчится только:

- `vllm/v1/executor/ray_utils.py`
- `RayWorkerWrapper.adjust_rank()`

После этого `adjust_rank()` синхронно обновляет и `rpc_rank`, и
`global_rank`, и `PP=3` pipeline начинает работать корректно на live
`DeepSeek-R1-Distill-Qwen-14B`.

## Live-проверки после раскатки

### 1. Проверить `Ray`

```bash
HEAD_POD=$(
  KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
  kubectl -n kuberay-projects get pods -l ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}'
)

KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects exec "$HEAD_POD" -c ray-head -- \
  bash -lc 'ray status && echo "=====SERVE=====" && serve status'
```

Ожидаемое состояние:

- `Recent failures: (no failures)`
- `3.0/3.0 GPU used`
- `applications.llms.status = RUNNING`
- `LLMServer:deepseek-r1-distill-qwen-14b = HEALTHY`
- `OpenAiIngress = HEALTHY`

### 2. Проверить внешний API

Сервисный host:

```text
https://openai-api.k8s-dvp.apiac.ru
```

Список моделей:

```bash
curl -k https://openai-api.k8s-dvp.apiac.ru/v1/models
```

Простой inference:

```bash
curl -k https://openai-api.k8s-dvp.apiac.ru/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-r1-distill-qwen-14b",
    "messages": [
      {"role": "user", "content": "Напиши одно короткое предложение про RDMA."}
    ],
    "max_tokens": 64
  }'
```

### 3. Проверить, что `Ray` реально использует `RDMA`

На worker в логах должны присутствовать:

```text
NCCL_SOCKET_IFNAME set by environment to eth0
NCCL_IB_HCA set to mlx5_*
NET/IB : Using [0]mlx5_*:1/RoCE [RO]
Using network IB
Init COMPLETE
Connected all trees
```

Практический смысл:

- `eth0` используется для socket bootstrap;
- `mlx5_*` используется для `NCCL` data path;
- `NCCL` не откатывается на plain TCP transport.

Команда:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects exec <worker-pod> -c ray-worker -- \
  bash -lc 'grep -R -h -E "Bootstrap:|NET/IB|Using network IB|Init COMPLETE|Connected all trees" /tmp/ray/session_latest/logs/worker-* | tail -n 60'
```

### 4. Проверить live `RDMA` surface в worker

Пример для `w1-c2`:

```bash
W1C2_POD=$(
  KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
  kubectl -n kuberay-projects get pods -l ray.io/node-type=worker,worker.apiac.ru/name=v100-w1-c2 -o jsonpath='{.items[0].metadata.name}'
)

KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects exec "$W1C2_POD" -c ray-worker -- \
  bash -lc 'ibv_devinfo -d mlx5_0 | egrep "state:|active_mtu:|link_layer:" && cat /sys/class/infiniband/mlx5_0/ports/1/gids/3'
```

Исправное состояние:

- `PORT_ACTIVE`
- `active_mtu: 4096`
- `link_layer: Ethernet`
- ненулевой `GID[3]`

## Подтверждённые метрики на live `Ray` worker

Ниже именно те цифры, которые были сняты не на старых `gpudirect-pod-*`, а на
текущей живой межузловой паре `Ray` worker:

- server:
  worker из группы `v100-w1-c2`
- client:
  worker из группы `v100-w3-01`
- device:
  `mlx5_0`
- direct IP:
  `172.31.140.3 <-> 172.31.140.2`
- `GID index`:
  `3`

### `RDMA` throughput

Команды:

```bash
ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -F --report_gbits -D 5
ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -F --report_gbits -D 5 172.31.140.3
```

Результат:

- `BW average[Gb/sec] 97.29`

### `RDMA` latency

Команды:

```bash
ib_send_lat -d mlx5_0 -x 3 -s 64 -n 5000 -F
ib_send_lat -d mlx5_0 -x 3 -s 64 -n 5000 -F 172.31.140.3
```

Результат:

- `t_avg 2.97 usec`
- `t_min 2.87 usec`
- `t_max 10.04 usec`
- `99% percentile 5.92 usec`
- `99.9% percentile 8.16 usec`

Практический вывод:

- текущий `RDMA` path на live worker работает на уровне,
  ожидаемом от этого стенда;
- `Ray` и `vLLM` сидят поверх уже живого `NET/IB` path, а не поверх деградировавшего
  TCP-only fallback.

## Что видно в логах `Ray` про сеть

Из самих `Ray`/`vLLM` логов можно вытащить:

- факт использования `NET/IB`;
- точные `mlx5_*`, через которые идёт `NCCL`;
- bootstrap timings;
- latency API-запросов на уровне `Serve proxy`.

Из логов `Ray` нельзя надёжно получить:

- чистый line-rate в `Gb/sec`;
- чистую `RDMA` latency в `usec`;
- byte counters/throughput per interface.

Поэтому для wire-level метрик используются отдельные `perftest`-команды,
запущенные внутри тех же worker pod.

### Что уже видно в `Serve proxy` логах

На текущем живом rollout `proxy` фиксировал:

- `GET /v1/models`: `10.7ms`, `11.7ms`, `12.2ms`, `16.8ms`
- `POST /v1/chat/completions`:
  - `8047.0ms`
  - `11025.0ms`
  - `20225.2ms`
  - `21071.0ms`
  - `24476.9ms`
  - `30378.3ms`
  - `72948.7ms`
  - `119112.0ms`
  - `161373.4ms`

Это уже end-to-end latency inference path, не отдельная задержка сети.

## Типовые проблемы

### `HF` token не задан

Симптом:

- `preload-*` на head не скачивает модель;
- `head` долго висит в `Init`.

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects logs <head-pod> -c preload-deepseek-r1-distill-qwen-14b
```

### Worker получил `RDMA` device, но `NCCL` не видит живой `GID`

Симптом:

- в логах `local GID ::, remote GID ::`
- `ibv_modify_qp failed ... No data available`

Что проверять:

- отработал ли `init-underlay-ip`;
- есть ли underlay IP на `enp*`;
- живой ли `GID[3]` у нужного `mlx5_*`.

### Маленький `memlock`

Симптом:

- `ibv_create_cq failed with error Cannot allocate memory`

Что проверять:

```bash
ulimit -l
cat /proc/self/limits | grep 'Max locked memory'
```

Если там снова `65536 bytes`, сначала чинится node/runtime слой, а не сам
`RayService`.

### В логах есть страшный `raylet` crash, но сервис жив

На текущем стенде это уже встречалось. В `raylet.err` лежали старые хвосты
неудачного раннего старта, а текущий rollout уже был healthy.

Проверять надо не только `raylet.err`, а сразу вместе:

```bash
ray status
serve status
kubectl get rayservice,raycluster,pods -n kuberay-projects
```

Если там всё `Running/HEALTHY`, старый хвост в `session_latest/logs` сам по
себе не означает живую проблему.

### Warning про `FA2` на `V100`

Симптом:

```text
Cannot use FA version 2 ... compute capability >= 8
```

Для `V100/Volta` это ожидаемо. Дальше `vLLM` уходит в другой backend
внимания и сам warning не считается blocker.

## Что не входит в документ

Документ не покрывает:

- host-side подготовку `RED OS 8` и NVIDIA/OFED с нуля;
- ручную отладку `DeviceClass` / `UnderlayNetwork` контроллеров;
- смену модели или переход на другой `ray-llm` image;
- бенчмарки качества/скорости самой модели;
- production hardening поверх текущего лабораторного профиля.
