# KubeRay + vLLM на 3x V100 с GPUDirect

## Контекст

В `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/` уже
зафиксирован предыдущий workstream по `RDMA` и `GPUDirect RDMA` на стенде
`k8s-dvp.apiac.ru`.

По его итогам уже известно:

- на `RED OS 8` и `V100-PCIE-32GB` host-side `GPUDirect RDMA` доведён до
  рабочего состояния через `nvidia-peermem`;
- в кластере подтверждён pod-level `GPUDirect RDMA`;
- на стенде уже есть три живых `GPUDirect` pod на базе `V100`;
- на этой платформе `compute capability = 7.0`, то есть это `Volta`, а не
  `Ampere/Hopper`.

Следующий workstream уже не про низкоуровневый smoke, а про практический
distributed inference scenario:

- запуск `KubeRay + vLLM`;
- использование трёх `V100` в распределённом режиме;
- подбор модели и launch shape, которые реально совместимы с `V100`,
  `fp16`, `GPUDirect RDMA` и текущими ограничениями `vLLM`.
- При этом целевой путь остаётся только pod-level:
  - worker pods должны использовать `sdn` / `DRA`;
  - `hostNetwork` и node-level `RDMA` workaround не рассматриваются;
  - дополнительные Linux capabilities надо избегать и добавлять только если
    без них уже доказанно не работает целевой pod-level сценарий.

## Постановка задачи

Нужно выработать практический и source-backed план запуска `KubeRay + vLLM`
на трёх `V100 32GB` с использованием `GPUDirect RDMA` между узлами.

В рамках этой задачи нужно ответить на вопросы:

- какой distributed shape имеет смысл для трёх `V100`:
  - `tensor parallel`;
  - `pipeline parallel`;
  - их комбинация;
- какая модель в классе около `80 GB` в `fp16` реалистично подходит под этот
  стенд;
- как именно считать память:
  - чистый вес модели;
  - служебные накладные расходы;
  - запас под KV cache;
- как именно выглядит `V1`-путь на `V100` через публичный `rayproject/ray-llm`
  образ и нужен ли вообще отдельный `V0` fallback;
- какие версии и параметры `KubeRay`, `Ray`, `vLLM` и `NCCL` стоит
  зафиксировать;
- какие ограничения `V100 / compute capability 7.0` влияют на выбор модели,
  режима параллелизма и движка;
- какой образ, какие env vars и какой launch command должны использоваться в
  `KubeRay`;
- какие минимальные допуски внутри pod действительно нужны:
  - какие devices должны приехать через plugins;
  - можно ли обойтись без дополнительных capabilities;
  - что делать, если exact `GPU/NIC` pairing неудобно выражается в шаблонах
    `KubeRay`, не уходя в `hostNetwork`;
- какие реальные риски и ceilings надо ожидать ещё до первого запуска.

## Scope

- собрать актуальные факты по:
  - `vLLM` hardware support для `Volta / compute capability 7.0`;
  - статусу `V0` и `V1` engine;
  - distributed serving в `vLLM`;
  - `KubeRay` runtime expectations;
- определить один основной recommended profile и не более двух fallback
  профилей;
- подобрать модель-кандидат для 3x `V100 32GB` в `fp16`;
- сделать расчёт памяти и объяснить, почему модель помещается или не
  помещается;
- описать recommended launch topology для `KubeRay + vLLM`;
- описать её именно как pod-level `sdn` / `DRA` профиль, а не node-level
  обходной путь;
- зафиксировать это в новом bundle как operator-facing техническую записку;
- materialize минимальный внешний deliverable в соседнем `k8s-config` repo:
  - отдельный каталог под `k8s-dvp.apiac.ru/kuberay`;
  - raw manifests для `RayService` на `V100 + RDMA`;
  - вспомогательные `Secret`/`PVC`/`ServiceAccount`/`ResourceClaimTemplate`
    example-файлы;
  - `Argo CD Application`, указывающий на новый каталог;
- довести live `RayService` bring-up в `k8s-dvp.apiac.ru` до состояния, где:
  - worker pod поднимаются без `NET_ADMIN` и других лишних capability;
  - `sdn/DRA` автоматически довозит underlay device в pod;
  - `NCCL/Gloo` pinned на прямой underlay path;
  - `Serve` больше не ломается на несовместимом `placement_group_config`;
- при необходимости опереться на archived predecessor только как на фон, но не
  как на новый source of truth.

## Non-goals

- не выполнять прямо сейчас deployment `KubeRay` в кластере;
- не писать production-ready Helm/chart/manifests для `KubeRay`;
- не менять runtime code, CRD или API в репозитории `ai-models`;
- не пытаться в этом срезе автоматически установить в `dvp` сам `KubeRay`
  operator/CRD, если их ещё нет в кластере;
- не делать benchmark actual inference latency/throughput в этом срезе;
- не проектировать весь inference stack платформы целиком;
- не смешивать эту задачу с phase-1/phase-2 runtime задачами модуля.

## Затрагиваемые области

- `plans/active/research-kuberay-vllm-v100-gpudirect-inference/*`
- `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s-dvp.apiac.ru/kuberay/*`
- при необходимости archived references:
  - `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/*`
- при необходимости существующие runtime/API references в repo:
  - `api/core/v1alpha1/*`
  - `crds/*`

## Критерии приёмки

- создан отдельный active bundle для нового workstream;
- есть source-backed compatibility verdict по `vLLM` на `V100 / cc 7.0`:
  - что поддерживается;
  - что не поддерживается;
  - почему нужен или не нужен `V0`;
- есть один основной recommended deployment profile для `3x V100`:
  - exact parallelism mode;
  - exact model class;
  - exact precision;
  - exact memory reasoning;
- есть минимум один fallback profile, если основной профиль не проходит по
  памяти или по ограничениям архитектуры;
- отдельно зафиксировано, какой размер модели в `fp16` реалистичен для
  `3x32GB` и что означает "модель около 80 GB" на практике;
- есть recommended `KubeRay + vLLM` launch shape:
  - head/worker layout;
  - распределение GPU по pod;
  - использование `sdn` / `DRA` на уровне pod;
  - отсутствие `hostNetwork`;
  - ключевые env vars;
  - базовая строка запуска `vllm serve`;
- в `k8s-config` создан отдельный каталог для `k8s-dvp.apiac.ru/kuberay`
  с raw manifests, который:
  - использует реальные `DeviceClass` / `ResourceClaimTemplate` имена для
    `w1-c2`, `w3-01` и `w1-80`;
  - не использует `hostNetwork`;
  - выражает pod-level `UnderlayNetwork` через annotation и
    `spec.resourceClaims`;
  - оставляет основной worker container без лишних capabilities;
- live `RayService` в `dvp` больше не содержит:
  - `init-underlay-ip`;
  - `NET_ADMIN`;
  - `placement_group_config` внутри `deployment_config`;
- зафиксирован честный runtime verdict по текущему live blocker, если rollout
  всё ещё срывается не из-за pod security / `sdn`, а из-за внешнего applier
  или самого `ray-llm` runtime;
- отдельно зафиксированы минимальные права pod:
  - какие capabilities не нужны;
  - какие можно не выдавать при штатном IPAM через `sdn`;
  - какие допустимы только как исключение и почему;
- есть отдельный раздел про риски и ограничения:
  - `Volta`;
  - `V0`;
  - `NCCL`;
  - делимость модели/attention heads для `TP`;
  - запас под KV cache;
- итог зафиксирован в bundle так, чтобы следующий шаг можно было уже
  превращать в deployment/validation без догадок.

## Риски

- можно выбрать модель, которая красиво выглядит по числу параметров, но не
  укладывается по реальной памяти `vLLM`;
- можно переоценить зрелость или ожидаемую производительность `V1` на
  `Volta`, хотя upstream уже формально допускает `cc 7.0`;
- можно выбрать `tensor_parallel_size=3`, а потом упереться в архитектурные
  ограничения конкретной модели;
- можно смешать "влезает по весам" и "влезает как serving runtime с KV cache";
- можно переоценить пользу `GPUDirect` для профиля, где bottleneck будет не в
  меж-GPU обмене, а в самом decode path.
