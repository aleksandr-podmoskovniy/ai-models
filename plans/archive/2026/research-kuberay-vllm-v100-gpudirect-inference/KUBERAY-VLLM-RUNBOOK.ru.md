# KubeRay + vLLM на 3x V100 с GPUDirect RDMA

## Цель запуска

Этот документ отвечает на один практический вопрос: как поднять рабочий
`RayService` с `KubeRay + vLLM` на стенде `k8s-dvp`, где три `V100 32GB`
используются в распределённом режиме через `GPUDirect RDMA` между подами.

Документ основан не на абстрактной схеме, а на реальном запуске
`deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`, который уже поднимался на этом
стенде и отвечал через внешний API. Здесь зафиксированы:

- проверенный стенд и текущий рабочий профиль;
- что должно быть готово до раскатки;
- где лежат GitOps-манифесты;
- какой именно `RayService` раскатывается;
- почему в текущем профиле пока остаётся узкий патч в `vLLM`;
- как раскатать сервис;
- как проверить, что `Ray`, `Serve`, API и `RDMA` действительно работают.

Этот документ не повторяет подготовку хостов: `RED OS`, `OFED`,
`nvidia-peermem` и базовый `RDMA` с нуля. Ниже эти вещи считаются уже
доведёнными до рабочего состояния и перечислены как обязательные предпосылки.

## Что считается успешным запуском

Запуск можно считать успешным, когда одновременно выполнены следующие условия:

- `RayService` находится в состоянии `Running`;
- `Ray` видит все три GPU и не держит ресурсы в ожидании группы размещения
  (`placement group`);
- `Serve` показывает приложение и deployment в состоянии `HEALTHY`;
- внешний API на `openai-api.k8s-dvp.apiac.ru` отвечает;
- в логах worker-подов видно, что `NCCL` поднялся именно через `NET/IB`, а не
  ушёл в откат на обычный `TCP`;
- внутри worker-подов есть живой `RDMA`-интерфейс, активный `mlx5_*` и ненулевой
  `GID[3]`.

## Маршрут запуска

Документ лучше проходить именно в таком порядке:

1. Сначала проверить, что базовый `RDMA` и `GPUDirect RDMA` уже живы на
   уровне узлов и тестовых подов.
2. Затем проверить, что в кластере уже есть нужные `UnderlayNetwork`,
   `DeviceClass` и нормальный `memlock` на GPU-нодах.
3. После этого раскатать `KubeRay` по шагам: `CRD -> оператор -> memlock ->
   HF secret -> RayService`.
4. Затем проверить, что живы `Ray`, `Serve`, внешний API и `NCCL over RDMA`.
5. Только после этого смотреть на сетевые цифры, логи и типовые проблемы.

## Стенд и рабочий профиль

### Что это за стенд

В текущем профиле используются две GPU-ноды:

| Узел | Тип | Что на нём используется |
| ---- | --- | ----------------------- |
| `k8s-dvp-w1-gpu.apiac.ru` | физический узел | две рабочие пары `GPU + RDMA` |
| `k8s-dvp-w3-gpu.apiac.ru` | `VM` на `PVE` | одна рабочая пара `GPU + RDMA` |

Поэтому в схеме участвуют три группы worker, хотя нод всего две: на `w1`
используются две разные пары `GPU + RDMA`, а на `w3` одна.

### Какие worker и прямые сети используются

| Группа worker | Узел | Заявка на GPU | Прямая сеть | Прямой интерфейс | RDMA-устройство |
| ------------- | ---- | ------------- | ----------- | ---------------- | --------------- |
| `v100-w1-80` | `k8s-dvp-w1-gpu.apiac.ru` | `gpu-v100-w1-81-gpudirect` | `rdma-w1-pair80` | `enp130s0np0` | `mlx5_1` |
| `v100-w1-c2` | `k8s-dvp-w1-gpu.apiac.ru` | `gpu-v100-w1-c2-gpudirect` | `rdma-w1-pairc0` | `enp193s0np0` | `mlx5_0` |
| `v100-w3-01` | `k8s-dvp-w3-gpu.apiac.ru` | `gpu-v100-w3-01-gpudirect` | `rdma-w3-pair00` | `enp2s0np0` | `mlx5_0` |

Прямые адреса на рабочей схеме:

| Worker | Прямой IP | `GID index` |
| ------ | --------- | ----------- |
| `v100-w1-80` | `172.31.140.1/29` | `3` |
| `v100-w3-01` | `172.31.140.2/29` | `3` |
| `v100-w1-c2` | `172.31.140.3/29` | `3` |

Практически эта сеть используется так:

- `eth0` нужен для начальной координации `Ray` и `Gloo`;
- основной обмен `NCCL` идёт через `NET/IB` на `mlx5_*`;
- для `RoCE v2` используется `GID index = 3`.

### Какой профиль сейчас считается рабочим

Ниже приведён точный профиль, на котором сервис уже поднимался и отвечал:

- кластер: `k8s-dvp.apiac.ru`
- пространство имён сервиса: `kuberay-projects`
- пространство имён оператора: `kuberay-operator`
- `KubeRay operator`: `v1.6.0`
- образ: `rayproject/ray-llm:nightly.260418.64385a-py311-cu128`
- `RayService.spec.rayVersion`: `"2.55.0"`
- `ray` внутри контейнера: `3.0.0.dev0`
- `vllm`: `0.19.0`
- `torch`: `2.10.0+cu128`
- `transformers`: `4.57.6`
- модель: `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`
- `dtype`: `half`
- `tensor_parallel_size`: `1`
- `pipeline_parallel_size`: `3`
- `max_model_len`: `8192`
- `max_num_seqs`: `1`
- `reasoning_parser`: `deepseek_r1`
- класс хранилища: `ceph-fs-nvme-sc`
- внешний адрес API: `openai-api.k8s-dvp.apiac.ru`

Из этого профиля важны четыре вещи:

- рабочая конфигурация подтверждена именно на `vllm 0.19.0`, а не на
  условном `0.19.x`;
- поле `rayVersion` в `RayService` и фактическая версия `ray` внутри образа
  здесь не совпадают, и для этого профиля это не считается проблемой;
- распределение сделано как `PP=3`, `TP=1`, то есть один worker на каждую
  `V100`;
- для reasoning-модели `reasoning_parser=deepseek_r1` обязателен.

На head-поде рабочий набор библиотек выглядел так:

```text
vllm 0.19.0
ray 3.0.0.dev0
torch 2.10.0+cu128
transformers 4.57.6
```

## Что должно быть готово до раскатки

До запуска `KubeRay` должны быть закрыты три слоя: узлы, сеть и кластерные
объекты.

### 1. Базовый `RDMA` и `GPUDirect RDMA` уже живы

Перед раскаткой должно быть уже подтверждено, что:

- на GPU-нодах работает обычный `RDMA`;
- `nvidia-peermem` поднимается штатно;
- `perftest` доступен и умеет работать с CUDA;
- тестовый под получает прямой сетевой интерфейс и `/dev/infiniband/uverbs*`;
- `ib_write_bw` и `ib_send_lat` уже проходили внутри тестового пода.

Если это ещё не так, сначала нужно чинить низкоуровневый слой: узел,
виртуализацию, прямую сеть, `GID`, `RDMA`-устройство в поде и только потом идти в
`RayService`.

Для `w3` это особенно важно, потому что нода живёт как `VM` на `PVE`. На ней
до раскатки должны быть уже исправлены `vIOMMU`, `intel_iommu=on iommu=pt` и
видимый `iommu_group` у проброшенного Mellanox PF.

### 2. Нужные сетевые объекты уже существуют в кластере

До раскатки сервиса в кластере уже должны быть:

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

### 3. На GPU-нодах уже поднят нормальный `memlock`

Для `NCCL over RDMA` на `V100` нужен нормальный `memlock`. На `dvp` это
делается отдельным скриптом:

```text
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/apply-containerd-memlock.sh
```

Проверка внутри worker-пода:

```bash
ulimit -l
cat /proc/self/limits | grep 'Max locked memory'
```

Исправное состояние:

- `unlimited`
- либо другой высокий лимит, но не `65536 bytes`

Если здесь снова `65536 bytes`, сначала нужно чинить узел и `containerd`,
а не сам `RayService`.

## Где лежат манифесты

GitOps-каталог находится вне репозитория `ai-models`:

```text
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay
```

Основные файлы:

- `argo-app/01-helm-kuberay-operator-crds.yaml`
- `argo-app/02-helm-kuberay-operator.yaml`
- `argo-app/10-kuberay-service-v100-rdma.yaml`
- `apply-containerd-memlock.sh`
- `charts/ray-service-v100-rdma/20-v100-rdma-deepseek-r1-qwen14b-rayservice.yaml`
- `charts/ray-service-v100-rdma/12-api-ingress.yaml`
- `charts/ray-service-v100-rdma/03-hf-secret.yaml.example`
- `charts/ray-service-v100-rdma/README.ru.md`

Практически здесь важно следующее:

- каталог `charts/kuberay-operator` в `dvp` сделан как локальная копия
  официального чарта `KubeRay v1.6.0`;
- каталог `charts/ray-service-v100-rdma` не является Helm chart:
  `Chart.yaml`, `values.yaml` и `templates/` там убраны;
- `Argo CD` читает каталог сервиса как обычный набор манифестов.

## Что именно раскатывается

Каталог `charts/ray-service-v100-rdma` создаёт:

- пространство имён `kuberay-projects`
- `ServiceAccount kuberay-llm`
- `PersistentVolumeClaim model-cache-pvc`
- внешний `Redis` для устойчивости `Ray`
- `ResourceClaimTemplate` под пары `GPU/RDMA`
- `RayService llm-v100-rdma-deepseek-r1-qwen14b`
- `Ingress llm-v100-rdma-api`

Внутри `RayService` сейчас закреплены такие параметры:

- `model_source: deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`
- `distributed_executor_backend: ray`
- `tensor_parallel_size: 1`
- `pipeline_parallel_size: 3`
- `dtype: half`
- `gpu_memory_utilization: 0.90`
- `max_model_len: 8192`
- `max_num_seqs: 1`
- `reasoning_parser: deepseek_r1`

Размещение такое:

- одна реплика `Serve`
- одна группа размещения (`placement group`) на три набора по `CPU:8, GPU:1`
- один worker-под на каждую `V100`

Из практических нюансов здесь важны ещё две вещи:

- если задать слишком маленький `max_tokens`, reasoning-модель может успеть
  вернуть только поле `reasoning` без обычного текста ответа;
- в рабочем прогоне с `max_tokens=512` модель уже возвращала и поле
  `reasoning`, и обычный `content: "4"`.

## Что важно не сломать в текущем профиле

### 1. `Gloo` и `Ray` не стартуют через прямой интерфейс `enp*`

В текущем рабочем профиле:

- `GLOO_SOCKET_IFNAME` и `NCCL_SOCKET_IFNAME` должны смотреть на `eth0`;
- `NCCL_IB_HCA` и связанная `RDMA`-настройка должны оставаться на `mlx5_*`.

Практический смысл такой:

- `eth0` нужен для начальной координации;
- основной путь обмена `NCCL` идёт через `NET/IB` на `mlx5_*`;
- если прибить `GLOO_SOCKET_IFNAME` к прямому интерфейсу `enp*`, начальная
  координация может снова сломаться, потому что эти интерфейсы в текущем
  профиле `sdn/DRA` приезжают в под без отдельного `L3`-адреса.

### 2. Патч в `vLLM` пока убирать нельзя

На образе `rayproject/ray-llm:nightly.260418.64385a-py311-cu128` с
`vllm 0.19.0` сам `RDMA/NCCL` уже работает. Проблема была не в сети, а в том,
как `vLLM` ведёт себя в многонодовом режиме с `pipeline_parallel_size=3`.

Без патча происходило следующее:

- `rpc_rank` после переназначения уже новый;
- `global_rank` оставался старым;
- `WorkerWrapperBase.initialize_from_config()` выбирал `kv_cache_config` по
  старому `global_rank`;
- этап конвейера получал не свои слои и не своё отображение `KV`;
- дальше модель падала с `KeyError` или начинала выдавать мусор.

Сейчас это обходится узким патчем без собственного образа и без
`sitecustomize.py`. Через связку `ConfigMap + initContainer + subPath` в
worker-поде
подменяется только одно место:

- `vllm/v1/executor/ray_utils.py`
- `RayWorkerWrapper.adjust_rank()`

После этого `adjust_rank()` обновляет и `rpc_rank`, и `global_rank`, и
конвейер `PP=3` начинает работать нормально.

Итоговая рабочая связка сейчас такая:

- `vllm 0.19.0`
- `ray 3.0.0.dev0`
- `torch 2.10.0+cu128`
- `transformers 4.57.6`
- образ `nightly.260418.64385a-py311-cu128`
- узкий патч только на `RayWorkerWrapper.adjust_rank()`

Если это поведение будет исправлено в самом `vLLM`, патч можно будет убрать.

## Порядок запуска

### 1. Включить CRD `KubeRay`

```bash
argocd app sync kuberay-operator-crds
```

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl get crd | egrep 'rayservices|rayclusters|rayjobs'
```

Ожидается:

- `rayservices.ray.io`
- `rayclusters.ray.io`
- `rayjobs.ray.io`

### 2. Включить оператор

```bash
argocd app sync kuberay-operator
```

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-operator get deploy,pods
```

Нормальное состояние:

- deployment оператора `Available=True`
- pod оператора `Running`

### 3. Поднять `memlock` на GPU-нодах

```bash
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/apply-containerd-memlock.sh --ssh
```

После этого новые worker-поды должны показывать высокий `memlock`, а не
`65536 bytes`.

### 4. Подготовить токен `HF`

Нужно отредактировать файл:

```text
/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/charts/ray-service-v100-rdma/03-hf-secret.yaml.example
```

В кластере после этого должен появиться секрет `hf-secret`:

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
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects get secret hf-secret
```

### 5. Включить сервис

```bash
argocd app sync kuberay-service-v100-rdma
```

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects get rayservice,raycluster,pods,pvc,svc,ingress -o wide
```

После раскатки ожидается:

- `RayService llm-v100-rdma-deepseek-r1-qwen14b` в `Running`
- `RayCluster ...` в `ready`
- head и три worker-пода в `Running`
- `model-cache-pvc` в `Bound`
- есть сервис `llm-v100-rdma-deepseek-r1-qwen14b-serve-svc`
- есть ingress `llm-v100-rdma-api`

## Как проверить, что сервис реально жив

### 1. Проверить `Ray` и `Serve`

```bash
HEAD_POD=$(
  KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
  kubectl -n kuberay-projects get pods -l ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}'
)

KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects exec "$HEAD_POD" -c ray-head -- \
  bash -lc 'ray status && echo "=====SERVE=====" && serve status'
```

Ожидаемая картина:

- `Recent failures: (no failures)`
- `3.0/3.0 GPU used`
- `applications.llms.status = RUNNING`
- `LLMServer:deepseek-r1-distill-qwen-14b = HEALTHY`
- `OpenAiIngress = HEALTHY`
- `ray status` не показывает незакрытый спрос по текущей группе размещения
  (`placement group`)

### 2. Проверить внешний API

Список моделей:

```bash
curl -k https://openai-api.k8s-dvp.apiac.ru/v1/models
```

Простой запрос:

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

Если нужно проверить не только сам ответ, но и reasoning-часть, лучше сразу
давать больший `max_tokens`, например `512`.

### 3. Проверить, что `NCCL` действительно пошёл через `RDMA`

В логах worker-пода нужны строки вида:

```text
NCCL_SOCKET_IFNAME set by environment to eth0
NCCL_IB_HCA set to mlx5_*
NET/IB : Using [0]mlx5_*:1/RoCE [RO]
Using network IB
Init COMPLETE
Connected all trees
```

Это означает:

- `eth0` используется только для начальной координации;
- основной путь обмена `NCCL` идёт через `mlx5_*`;
- отката на обычный `TCP` нет.

Команда:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects exec <worker-pod> -c ray-worker -- \
  bash -lc 'grep -R -h -E "Bootstrap:|NET/IB|Using network IB|Init COMPLETE|Connected all trees" /tmp/ray/session_latest/logs/worker-* | tail -n 60'
```

### 4. Проверить `RDMA` внутри worker

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

Ожидается:

- `PORT_ACTIVE`
- `active_mtu: 4096`
- `link_layer: Ethernet`
- ненулевой `GID[3]`

## Какие сетевые цифры уже получены на текущей рабочей паре подов

Замеры ниже снимались уже на текущей рабочей межузловой паре подов, а не на
старых тестовых `gpudirect-pod-*`.

- принимающая сторона теста: worker-под из группы `v100-w1-c2`
- отправляющая сторона теста: worker-под из группы `v100-w3-01`
- устройство: `mlx5_0`
- прямые IP: `172.31.140.3 <-> 172.31.140.2`
- `GID index`: `3`

### Пропускная способность `RDMA`

```bash
ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -F --report_gbits -D 5
ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -F --report_gbits -D 5 172.31.140.3
```

Результат:

- `BW average[Gb/sec] 97.29`

### Задержка `RDMA`

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

Это означает, что межузловой `RDMA` между worker-подами действительно живой, а
`Ray` и `vLLM` работают поверх реального `NET/IB`, а не поверх аварийного
отката на обычный `TCP`.

## Что логи подтверждают, а что нет

По логам `Ray`, `vLLM` и `NCCL` можно уверенно подтвердить:

- что `NCCL` использует `NET/IB`;
- какие `mlx5_*` реально задействованы;
- что `Serve` и `Ray` дошли до рабочего состояния;
- полную задержку запроса на уровне `Serve proxy`.

По этим логам нельзя надёжно получить:

- точную пропускную способность линии;
- чистую сетевую задержку `RDMA`;
- счётчики байтов по интерфейсам.

Для этого и нужны отдельные замеры `perftest`.

В текущем запуске `Serve proxy` показывал, например:

- `GET /v1/models`: `10.7ms`, `11.7ms`, `12.2ms`, `16.8ms`
- `POST /v1/chat/completions`: `8047.0ms`, `11025.0ms`, `20225.2ms`,
  `21071.0ms`, `24476.9ms`, `30378.3ms`, `72948.7ms`, `119112.0ms`,
  `161373.4ms`

Это уже полная задержка запроса к модели, а не чистая сетевая задержка.

## Частые проблемы

### Не задан токен `HF`

Симптомы:

- `preload-*` на head-поде не скачивает модель;
- head-под долго висит в `Init`.

Проверка:

```bash
KUBECONFIG=/Users/myskat_90/.kube/k8s-config \
kubectl -n kuberay-projects logs <head-pod> -c preload-deepseek-r1-distill-qwen-14b
```

### Worker-под получил `RDMA`-устройство, но `NCCL` не видит `GID`

Симптомы:

- в логах `local GID ::, remote GID ::`;
- `ibv_modify_qp failed ... No data available`.

Что смотреть:

- появился ли прямой IP на `enp*`;
- живой ли `GID[3]` у нужного `mlx5_*`;
- соответствует ли интерфейс той паре `GPU + RDMA`, на которую был рассчитан
  worker.

### Слишком маленький `memlock`

Симптом:

- `ibv_create_cq failed with error Cannot allocate memory`

Проверка:

```bash
ulimit -l
cat /proc/self/limits | grep 'Max locked memory'
```

Если там снова `65536 bytes`, сначала нужно чинить ноду и `containerd`, а
не сам `RayService`.

### `vLLM` снова падает с `KeyError` по `model.layers...self_attn.attn`

Если вернулась ошибка вида:

```text
KeyError: model.layers.{...}.self_attn.attn
```

в первую очередь надо проверить, не потерялся ли текущий узкий патч на
`RayWorkerWrapper.adjust_rank()`. Для этого профиля это не сетевой симптом, а
симптом рассинхронизации рангов внутри `vLLM V1`.

### Модель отвечает только полем `reasoning` или не успевает дойти до обычного ответа

Сначала надо проверить две вещи:

- в `serveConfigV2` действительно задан `reasoning_parser: deepseek_r1`;
- в запросе достаточно большой `max_tokens`.

Для быстрой живой проверки лучше не ставить слишком маленький `max_tokens`.
На этом стенде рабочий прогон с нормальным текстом ответа уже проходил на
`max_tokens=512`.

### В логах есть старый `raylet` crash, но сервис уже жив

Такое на этом стенде уже было. В `raylet.err` лежали хвосты от раннего
неудачного запуска, хотя текущий запуск уже работал нормально.

Поэтому проверять нужно не только `raylet.err`, а сразу всё вместе:

```bash
ray status
serve status
kubectl get rayservice,raycluster,pods -n kuberay-projects
```

Если там всё `Running/HEALTHY`, старые строки в `session_latest/logs` сами по
себе ещё не означают, что проблема актуальна.

### Предупреждение про `FA2` на `V100`

Обычно это выглядит так:

```text
Cannot use FA version 2 ... compute capability >= 8
```

Для `V100/Volta` это ожидаемо. Дальше `vLLM` переключается на другой механизм
внимания, и само предупреждение не считается блокирующим.

## Что здесь не разбирается

В документ не входят:

- подготовка `RED OS 8`, NVIDIA и `OFED` на хосте с нуля;
- ручная отладка `UnderlayNetwork` и `DeviceClass` контроллеров;
- перенос запуска на другую модель или другой образ `ray-llm`;
- бенчмарки качества и скорости самой модели;
- доведение текущей схемы до production-ready состояния.
