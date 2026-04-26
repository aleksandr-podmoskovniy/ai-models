# PLAN

## Current phase

Research-to-runtime bring-up slice. Работы уже включают не только выбор
профиля, но и materialization внешнего `k8s-config` каталога и живую
валидацию `RayService` в `k8s-dvp.apiac.ru`.

Текущий runtime verdict по `dvp`:

- `Qwen/Qwen3-14B` на `ray-llm 2.54.0 / vLLM 0.15.0 V1 / PP=3 / 3x V100`
  доходит до рабочего `RDMA/NCCL`, но падает на
  `KeyError: model.layers.{0,13,27}.self_attn.attn`;
- `DeepSeek-R1-Distill-Qwen-14B` на том же старом профиле падает на
  `KeyError: model.layers.{0,16}.self_attn.attn`;
- на pinned nightly `rayproject/ray-llm:nightly.260418.64385a-py311-cu128`
  (`vLLM >= 0.19.0`) сам `RDMA/NCCL` path уже подтверждён:
  `NET/IB`, `Init COMPLETE`, `Connected all trees`;
- финальный blocker оказался в `vLLM` Ray executor:
  после worker re-rank upstream `adjust_rank()` обновляет только `rpc_rank`,
  но оставляет stale `global_rank`;
- из-за этого `WorkerWrapperBase.initialize_from_config()` выбирает
  `kv_cache_config` по wrong `global_rank`, и `PP` stage получает не свой
  layer/KV mapping;
- корректный runtime workaround без custom image: маленький `ConfigMap`
  патчит только `vllm/v1/executor/ray_utils.py::RayWorkerWrapper.adjust_rank`,
  синхронизируя `rpc_rank` и `global_rank`;
- в live cluster это сняло мусорный output path: сервис на `PP=3` и
  direct `GPU + RDMA` стал `HEALTHY`, а `DeepSeek` снова возвращает
  корректный `content` при достаточном `max_tokens`;
- отдельно для reasoning-модели зафиксировано, что
  `engine_kwargs.reasoning_parser=deepseek_r1` обязателен, иначе reasoning
  токены уходят в обычный `content`.
- archived `SKALA` документ оставлен как low-level smoke reference и больше
  не должен читаться как альтернативный deployment path для whole app;
  для прикладной раскатки source of truth теперь явно закреплён за
  `KUBERAY-VLLM-RUNBOOK.ru.md`.

## Orchestration

- mode: `solo`
- reason:
  - задача исследовательская и docs-first;
  - главный риск сейчас в корректной compatibility/memory модели, а не в
    реализации кода;
  - отдельные subagents не разрешены пользователем явно.

## Slice 1. Reconstruct the hardware and runtime constraints

Цель:

- собрать ограничения, которые накладывают:
  - `V100`;
  - `compute capability 7.0`;
  - текущий `RDMA/GPUDirect` baseline;
  - layout на 3 GPU.

Затрагиваемые области:

- archived predecessor bundle
- official docs for `vLLM` and `KubeRay`

Проверки:

- facts must be separated from inference
- only primary or official sources

Артефакт:

- compact constraint baseline for the current stand.

## Slice 2. Determine the viable `vLLM` engine/runtime profile

Цель:

- понять, как `V100` влияет на выбор `vLLM` engine;
- определить, нужен ли явный `V0` pin/flag;
- понять, какие версии и launch knobs надо фиксировать.

Затрагиваемые области:

- active bundle docs
- official `vLLM` docs / issues / release references

Проверки:

- source-backed compatibility notes

Артефакт:

- exact engine/runtime verdict for `Volta`.

## Slice 3. Choose the model and parallelism strategy

Цель:

- подобрать основной модельный профиль;
- сравнить `TP=3`, `PP=3`, либо смешанный профиль;
- не выбрать конфигурацию, которая ломается на делимости голов, памяти или
  архитектурных ограничениях.

Затрагиваемые области:

- active bundle docs
- official model configs/cards
- official `vLLM` docs

Проверки:

- explicit memory math
- explicit model architecture constraints

Артефакт:

- primary recommendation and fallback options.

## Slice 4. Define the `KubeRay + vLLM` deployment shape

Цель:

- превратить выбранный профиль в практический launch plan:
  - pod layout;
  - head/worker shape;
  - GPU assignment;
  - env vars;
  - launch command.

Затрагиваемые области:

- active bundle docs
- official `KubeRay` docs
- official `vLLM` docs

Проверки:

- launch shape must be consistent with the chosen parallelism mode
- no contradiction with the known GPUDirect baseline

Артефакт:

- concrete deployment recipe draft.

## Slice 5. Record the recommendation

Цель:

- оформить итог в bundle как operator-facing technical note;
- оставить короткую actionable summary для следующего шага.

Затрагиваемые области:

- `plans/active/research-kuberay-vllm-v100-gpudirect-inference/*`

Проверки:

- `git diff --check`
- manual consistency review of the bundle

Артефакт:

- `RECOMMENDATION.ru.md` with:
  - recommended model;
  - recommended engine;
  - recommended parallelism;
  - deployment shape;
  - residual risks.

## Slice 6. Materialize the `k8s-config` delivery

Цель:

- собрать рядом с уже существующим `k8s.apiac.ru/kube-ray/charts/ray-service`
  отдельный `dvp`-каталог с raw manifests для текущего `V100 + RDMA` стенда;
- оставить этот каталог достаточно близким к соседнему кластеру по layout, но
  уже с pod-level `sdn` / `DRA` контрактом и реальными `dvp`-специфичными
  claim-именами.

Затрагиваемые области:

- `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/*`
- текущий active bundle docs

Проверки:

- YAML files must parse cleanly
- references to `ResourceClaimTemplate`, `UnderlayNetwork`, node hostnames and
  interfaces must match the known `dvp` stand
- no `hostNetwork`
- main worker container must not request unnecessary capabilities

Артефакт:

- external folder with:
  - `argo-app` entry;
  - `RayService` baseline manifest;
  - support secrets/PVC/ServiceAccount/examples;
  - namespaced `ResourceClaimTemplate` copies for `kuberay-projects`;
  - image build helper/example for the chosen `vLLM` baseline.

## Slice 7. Stabilize the live `RayService` bring-up

Цель:

- подтвердить, что worker pod реально поднимаются на трёх `V100` без
  `NET_ADMIN` и других лишних capability;
- подтвердить, что `sdn/DRA` автоматически довозит underlay claim и интерфейс
  в worker pod;
- довести `serveConfigV2` до схемы, совместимой именно с
  `rayproject/ray-llm:2.54.0-py311-cu128`;
- убедиться, что `Ray` видит `GPU=3` и placement group не остаётся
  `INFEASIBLE` из-за недооценённого `CPU` budget;
- отдельно зафиксировать, если live объект переписывается внешним applier и
  тем самым откатывает локально исправленный chart.

Затрагиваемые области:

- `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/*`
- `plans/active/research-kuberay-vllm-v100-gpudirect-inference/*`
- live cluster `k8s-dvp.apiac.ru`

Проверки:

- `kubectl get/describe rayservice,raycluster,pods -n kuberay-projects`
- `ray status` / `serve status` inside the live head pod
- no `NET_ADMIN` in live worker pod spec
- no `placement_group_config` nested under `deployment_config` in live object

Артефакт:

- honest runtime verdict:
  - either a working live rollout,
  - or an exact blocker outside the chart itself.

Текущий live verdict после первичной bring-up отладки:

- первый runtime blocker с `PENDING_NODE_ASSIGNMENT` уже снят:
  worker подняты до `10 CPU`, и `initialize_remote_node` больше не висит;
- `Ray` видит все три worker и placement group создаётся;
- текущий стопор уже глубже, внутри `vLLM V1` engine core:
  `torch.distributed.new_group(... backend=\"gloo\")` падает с
  `ProcessGroupGloo ... connect: Network is unreachable`;
- причина не в самом `NCCL`, а в socket bootstrap:
  `GLOO_SOCKET_IFNAME` и `NCCL_SOCKET_IFNAME` были прибиты к underlay
  интерфейсам `enp*`, которые в текущем `sdn/DRA` профиле приезжают в pod
  без отдельного L3-адреса;
- практический фикс для current chart: оставить `NCCL_IB_HCA` /
  `NCCL_IB_GID_INDEX` на `mlx5_*`, но перевести `GLOO_SOCKET_IFNAME` и
  `NCCL_SOCKET_IFNAME` на `eth0`, чтобы bootstrap шёл по обычному pod IP.

Текущий live verdict после этого фикса:

- `Gloo` bootstrap больше не является blocker;
- `NCCL` реально поднимается в `NET/IB` режиме:
  `Bootstrap: Using eth0:<pod-ip>` и `NET/IB : Using mlx5_*:1/RoCE [RO]`;
- новый blocker находится в `ibverbs` ресурсах:
  `misc/ibvwrap.cc:203 NCCL WARN Call to ibv_create_cq failed with error Cannot allocate memory`;
- на `w1` worker внутри pod `RLIMIT_MEMLOCK` равен всего `65536 bytes`, на
  `w3` — `8388608 bytes`;
- baseline-policy в namespace `kuberay-projects` не разрешает `IPC_LOCK`, так
  что прямое добавление capability сейчас не проходит `admission`;
- для текущего bring-up выбран pragmatic workaround:
  namespace `kuberay-projects` переводится в
  `security.deckhouse.io/pod-policy: privileged`, а в `ray-worker`
  добавляется только `IPC_LOCK`;
- как следующий более чистый шаг добавлен отдельный GitOps-компонент
  `kuberay-node-memlock`, который вешает systemd drop-in с
  `LimitMEMLOCK=infinity` на `containerd` только для
  `k8s-dvp-w1-gpu.apiac.ru` и `k8s-dvp-w3-gpu.apiac.ru`;
- долгосрочно целевой путь остаётся прежним:
  поднять default `memlock` в runtime/containerd и затем убрать `IPC_LOCK`
  обратно.

## Slice 8. Write the whole-app runbook

Цель:

- вынести из `RECOMMENDATION.ru.md`, внешнего `k8s-config` каталога и live
  cluster state отдельный operator-facing runbook;
- сделать документ по структуре близким к archived `SKALA` smoke doc, но уже
  для full application path:
  - `Argo CD`;
  - `KubeRay operator`;
  - `RayService`;
  - `HF secret`;
  - storage;
  - ingress;
  - live validation;
  - `RDMA`/`Ray` network evidence;
- не размазывать текущий rollout verdict по нескольким длинным файлам.

Затрагиваемые области:

- `plans/active/research-kuberay-vllm-v100-gpudirect-inference/*`
- archived reference:
  `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`
- `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/*`

Проверки:

- manual review for:
  - exact file paths;
  - exact `Argo` app names;
  - exact `Ingress` host;
  - exact `RayService` / `RayCluster` / pod names;
  - exact live `RDMA` metrics used in the document;
- `git diff --check`

Артефакт:

- standalone runbook that another operator can follow without reopening the
  whole chat history or the long `RECOMMENDATION.ru.md`.

## Rollback point

Если по итогам выяснится, что на `3x V100` жизнеспособен только слишком
ограниченный профиль:

1. не создавать ложное impression, что full distributed profile уже выбран;
2. оставить bundle как честный compatibility verdict;
3. выделить next-step отдельно: либо smaller model, либо иной runtime, либо
   другой hardware target.

## Final validation

- `git diff --check`
- manual bundle review against scope and factual consistency
- YAML parse check for the new external manifests
