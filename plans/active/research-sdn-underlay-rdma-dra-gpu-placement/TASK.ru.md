# Study SDN Underlay RDMA And GPU Placement

## Контекст

В соседнем репозитории `sdn` уже реализованы:

- `UnderlayNetwork` поверх Kubernetes DRA;
- проброс PF/VF в поды;
- режимы `NetDev`, `VFIO-PCI` и `DPDK`;
- собственный scheduler extender.

На живом кластере `k8s-dvp.apiac.ru` уже подтверждено:

- `sdn` модуль включён и работает;
- direct Mellanox 100GbE path между `k8s-dvp-w1-gpu.apiac.ru` и
  `k8s-dvp-w3-gpu.apiac.ru` существует;
- host-side RDMA по всем целевым direct NIC работает;
- на direct NIC выставлен `mtu 9000`, а RoCE path поднимается до
  `active_mtu 4096`.

Теперь нужно перейти от host-only validation к pod-level path через текущую
базу `sdn`: понять, как именно пробросить нужные Mellanox карты в pod,
какие `sdn` объекты/annotations для этого нужны, и как внутри pod доказать,
что RDMA действительно доступен и работает.

После первичного host-side smoke выяснилось ещё одно practical требование:

- plain `ib_write_bw --use_cuda=0` на `w1/w3` подтверждает, что `GPUDirect RDMA`
  path живой, но даёт только около `56-57 Gb/sec`;
- нужно довести benchmark path до near-line-rate на текущем железе и явно
  отделить:
  - что является real tuning fix;
  - что является hardware/virtualization ceiling;
  - какой минимальный pod security profile нужен для secure contour без
    `hostPath` и без `privileged`.

После pod-level `GPUDirect RDMA` smoke появился ещё один эксплуатационный
вопрос:

- нужно дать понятное сравнение `RDMA` и non-`RDMA` path на одних и тех же pod:
  - какими общепринятыми метриками это сравнивать;
  - какие exact команды запускать;
  - как интерпретировать результаты без `RDMA`-specific tribal knowledge.

Следующий practical gap после этого:

- сейчас smoke воспроизводим, но слишком зависит от ручных действий внутри pod;
- нужен отдельный reusable tools container, который:
  - уже содержит `iperf3`, `sockperf`, `perftest` и CUDA-aware `perftest`;
  - пишет результаты в понятный log/summary format;
  - умеет печатать короткий consolidated report и завершаться non-zero по
    порогам;
  - позволяет одинаково гонять `TCP`, plain `RDMA` и `GPUDirect RDMA`;
  - помогает самим командам быстро локализовать, сломался ли path на сети,
    на verbs уровне или уже на `GPU memory` path.

Ещё один практический шаг по этому workstream:

- нужно выгрузить с `k8s-dvp-w3-gpu.apiac.ru` все собранные и использованные
  артефакты для `RED OS 8` на локальный ноутбук в `~/Downloads`;
- артефакты должны лежать в отдельной папке и в архивах, чтобы их можно было
  переиспользовать на других таких же нодах;
- вместе с файлами нужно сохранить краткий manifest с тем, что именно было
  собрано и какие версии пакетов стояли на узле.

Для демонстрации текущего состояния стенда нужен ещё один отдельный шаг:

- в `sdn-rdma-test` должны быть подняты три актуальных `GPUDirect` pod:
  - межузловая benchmark-пара:
    - один на `w1`;
    - один на `w3`;
  - один дополнительный pod на spare `V100 + RDMA` паре на `w1`;
- старые plain-`RDMA` pod, которые занимают те же exact network pairs, нужно
  убрать, чтобы освободить claims;
- итоговое состояние должно позволять:
  - сразу показать внутри pod GPU через `nvidia-smi`;
  - сразу показать verbs devices через `/dev/infiniband`;
  - сразу показать direct underlay interface через `ip -br link`;
  - использовать межузловую `GPUDirect` пару как одну и ту же рабочую
    поверхность для:
    - plain `RDMA` benchmarks;
    - `GPUDirect RDMA` benchmarks;
    - `TCP` baseline на том же прямом канале.

## Постановка задачи

Исследовать локальный проект `/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn`
и ответить на практические вопросы:

- как сейчас пробрасываются отдельные сетевые устройства в pod;
- как на текущем `sdn` правильно описать и включить этот path в живом
  кластере;
- что именно должно оказаться внутри pod для RDMA-проверки;
- как провести pod-level smoke и functional validation для RDMA;
- как провести near-line-rate `GPUDirect RDMA` benchmark на текущем стенде и
  чем именно он должен запускаться;
- как сравнить pod-level `RDMA` и non-`RDMA` path по latency и throughput
  общепринятыми и воспроизводимыми метриками;
- какой reusable tools image и какой entrypoint/script нужен, чтобы эти тесты
  можно было стабильно гонять без ручной сборки окружения в каждом pod;
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
- прочитать live cluster CRD/status/manifests, связанные с:
  - `UnderlayNetwork`;
  - `SystemNetwork`;
  - DRA `ResourceClaim` / `ResourceClaimTemplate` / `DeviceClass` / `ResourceSlice`;
- зафиксировать текущую модель passthrough для PF/VF/netdev/VFIO;
- определить точный operational contract для pod-level passthrough Mellanox NIC
  через `sdn`;
- определить fastest practical benchmark profile для `GPUDirect RDMA` на
  `k8s-dvp-w1-gpu.apiac.ru` и `k8s-dvp-w3-gpu.apiac.ru`;
- собрать минимальный набор manifest shapes для:
  - network object;
  - pod annotation/spec;
  - validation pod;
- собрать reproducible benchmark/manual bundle для:
  - non-`RDMA` throughput/latency;
  - `RDMA` throughput/latency;
  - отдельного указания, где заканчивается plain `RDMA` и начинается
    `GPUDirect RDMA`;
- собрать bundle-local validation toolkit:
  - container build context;
  - entrypoint/runner scripts;
  - minimal pod manifest templates;
  - log/summary contract;
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
- не писать сразу финальный end-to-end runbook по всем слоям, пока не доказан
  pod-level `sdn` path;
- не смешивать исследование с полной интеграцией этого механизма в `ai-models`.
- не выдавать симметричный `~90+ Gb/sec` как achieved fact, если на одной из
  сторон останется подтверждённый ceiling.

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
- live cluster `k8s-dvp.apiac.ru`
  - `UnderlayNetwork`, `SystemNetwork`, `ClusterIPAddressPool`
  - `resourceclaims.resource.k8s.io`
  - `resourceslices.resource.k8s.io`
  - test manifests in a dedicated namespace
  - live GPU/RDMA nodes `k8s-dvp-w1-gpu.apiac.ru` and `k8s-dvp-w3-gpu.apiac.ru`
- локальный ноутбук исполнителя
  - `~/Downloads/*`

## Критерии приёмки

- описан текущий end-to-end path: `Pod annotation -> ResourceClaimTemplate ->
  DRA allocation -> claim prepare -> CDI/CNI handoff -> pod-visible device`;
- явно зафиксирован minimal operational contract для текущего `sdn`:
  - какие `sdn` объекты создать;
  - какие поля/labels/annotations обязательны;
  - какой `bindingMode` нужен для Mellanox RDMA baseline;
  - что ожидается внутри pod;
- есть минимальный pod-level validation flow:
  - как проверить presence netdev и RDMA character devices;
  - как проверить `ibv_devinfo`/`rdma link` внутри pod;
  - как проверить connectivity/bandwidth между двумя pod;
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
- если live cluster validation будет сделана, её результат зафиксирован в bundle
  без расхождения с фактическим состоянием кластера;
- есть explicit benchmark verdict:
  - fastest practical `GPUDirect RDMA` profile on the current hardware;
  - exact command line and exact measured throughput;
  - exact asymmetries or ceilings that remained;
- есть отдельный operator-facing comparison verdict для pod-level `RDMA` vs
  non-`RDMA`:
  - какие метрики использовать как canonical;
  - exact commands;
  - exact measured latency/throughput;
  - короткая интерпретация того, что означают эти цифры;
- есть reusable validation toolkit, который можно собрать как контейнер и
  использовать для self-service smoke:
  - image recipe;
  - entrypoint/runner modes;
  - expected env vars / arguments;
  - log/summary format;
  - short report mode;
  - optional threshold-based non-zero exit for regression smoke;
- явно зафиксирован minimal pod security contract для secure contour:
  - without `hostPath`;
  - without `privileged`;
  - with only the capabilities that were actually needed in live validation;
- отдельно проверен pod-level `GPUDirect RDMA`, то есть один и тот же pod
  одновременно получает:
  - GPU device;
  - Mellanox RDMA device через `UnderlayNetwork`;
  - и реально проходит межподовый benchmark с `--use_cuda*`;
- bundle оставляет один canonical operator-facing документ с пошаговой
  инструкцией и отдельно один deep engineering log без competing source of
  truth;
- на локальном ноутбуке появляется отдельная папка в `~/Downloads` с
  архивами, выгруженными с `k8s-dvp-w3-gpu.apiac.ru`;
- в этой папке есть manifest, из которого понятно:
  - какие именно артефакты были выгружены;
  - какие версии ключевых пакетов стояли на узле;
  - от какого узла и в какую дату сделана выгрузка;
- в кластере остаются три демонстрационных pod:
  - два `GPUDirect` pod:
    - один на `k8s-dvp-w1-gpu.apiac.ru`;
    - один на `k8s-dvp-w3-gpu.apiac.ru`;
  - один дополнительный `GPUDirect` pod на запасной exact pair
    `rdma-w1-pair80` и spare `V100`;
- старые plain-`RDMA` pod, занимавшие те же exact network claims, удалены;
- внутри новых pod подтверждаются одновременно:
  - GPU через `nvidia-smi`;
  - verbs devices через `/dev/infiniband`;
  - direct underlay interface через `ip -br link`;
- если код менялся, изменения остаются bounded и проверяемыми.

## Риски

- можно принять DPDK-over-mlx5 plumbing за полноценную RDMA поддержку, хотя это
  не одно и то же;
- можно перепутать node-level RDMA connectivity и pod-level passthrough contract,
  хотя это разные проверочные поверхности;
- можно выбрать неправильный `bindingMode` и получить working netdev без usable
  verbs path внутри pod;
- можно переоценить scheduler extender, хотя современный рынок мог сместиться в
  сторону scheduler framework и topology-aware drivers;
- можно недооценить необходимость topology/NUMA awareness для `GPU + NIC`
  matching.
