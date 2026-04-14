# Study SDN Underlay RDMA And GPU Placement

## Контекст

В соседнем репозитории `sdn` уже реализованы:

- `UnderlayNetwork` поверх Kubernetes DRA;
- проброс PF/VF в поды;
- режимы `NetDev`, `VFIO-PCI` и `DPDK`;
- собственный scheduler extender.

Нужно понять, можно ли использовать этот baseline не только для DPDK, но и для
RDMA workloads, а также как в будущем состыковать сетевые устройства с GPU
allocation через DRA и placement logic.

## Постановка задачи

Исследовать локальный проект `/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn`
и ответить на практические вопросы:

- как сейчас пробрасываются отдельные сетевые устройства в pod;
- что именно уже умеет текущий `DPDK`/`VFIO` path для Mellanox и SR-IOV;
- можно ли получить usable RDMA mode на текущем baseline или через bounded
  расширение;
- как проектировать future matching `GPU + RDMA NIC` через DRA;
- нужен ли отдельный scheduler extender, или на рынке уже есть более
  подходящие reference patterns.

## Scope

- прочитать ключевые `sdn` docs и runtime code paths, связанные с:
  - `UnderlayNetwork`;
  - DRA driver/controller;
  - claim mutation;
  - CNI/agent handoff;
  - scheduler extender;
- зафиксировать текущую модель passthrough для PF/VF/netdev/VFIO;
- оценить feasibility отдельного `RDMA` режима:
  - без code changes;
  - с bounded prototype change;
- собрать внешние reference sources по:
  - Kubernetes DRA;
  - RDMA device exposure;
  - GPU/NIC co-placement;
  - scheduler patterns на рынке;
- если появится узкий и defendable prototype path, реализовать только его.

## Non-goals

- не redesign'ить весь `sdn` runtime, CRD surface или scheduler architecture;
- не делать production-ready GPU/RDMA co-scheduler в одном срезе;
- не вводить новый платформенный контракт без evidence, что текущий baseline
  недостаточен;
- не смешивать исследование с полной интеграцией этого механизма в `ai-models`.

## Затрагиваемые области

- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/*`
- внешний repo `/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn`
  - `docs/*`
  - `api/network.deckhouse.io/v1alpha1/*`
  - `images/controller/src/internal/manager/controllers/underlay-controller/*`
  - `images/controller/src/internal/manager/controllers/pod-claim-webhook/*`
  - `images/agent/src/internal/dra-plugin/*`
  - `images/agent/src/internal/interface-syncer/*`
  - `images/agent/src/internal/cni-server/*`
  - `images/scheduler/src/internal/*`

## Критерии приёмки

- описан текущий end-to-end path: `Pod annotation -> ResourceClaimTemplate ->
  DRA allocation -> claim prepare -> CDI/CNI handoff -> pod-visible device`;
- явно зафиксировано, что уже умеет текущий Mellanox path и чего ему не хватает
  для реального RDMA workload;
- есть вывод, возможен ли `RDMA` режим:
  - как конфигурационный workaround;
  - как bounded code extension;
  - или только как отдельный larger workstream;
- есть recommendation для future `GPU + RDMA NIC` placement:
  - использовать только DRA / topology hints;
  - использовать scheduler extender / framework plugin;
  - или опереться на существующие market references;
- собраны и процитированы внешние источники, достаточные для инженерного
  решения;
- если код менялся, изменения остаются bounded и проверяемыми.

## Риски

- можно принять DPDK-over-mlx5 plumbing за полноценную RDMA поддержку, хотя это
  не одно и то же;
- можно переоценить scheduler extender, хотя современный рынок мог сместиться в
  сторону scheduler framework и topology-aware drivers;
- можно недооценить необходимость topology/NUMA awareness для `GPU + NIC`
  matching.
