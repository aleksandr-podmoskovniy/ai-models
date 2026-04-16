# PLAN

## Current phase

Исследовательский pre-implementation slice. Это не phase-1/2 runtime change в
`ai-models`, а bounded R&D по соседнему `sdn` repo для будущих DRA-based
placement решений.

## Orchestration

- mode: `solo`
- reason:
  - текущий срез аналитический и docs-first;
  - main risk сейчас не implementation throughput, а корректная реконструкция
    существующего `sdn` baseline и external market state;
  - отдельные subagents в этом runtime недоступны без явного user request.

## Slice 1. Reconstruct the current `sdn` underlay path

Цель:

- понять, как `UnderlayNetwork` публикует девайсы в DRA и как они доходят до
  pod.

Затрагиваемые области:

- внешний `sdn` repo:
  - `docs/README*.md`
  - `docs/ADMIN_GUIDE*.md`
  - `images/controller/.../underlay-controller/*`
  - `images/controller/.../pod-claim-webhook/*`
  - `images/agent/.../dra-plugin/*`
  - `images/agent/.../cni-server/*`
  - `images/agent/.../interface-syncer/*`

Проверки:

- targeted code reading and traceability notes

Артефакт:

- точная схема current passthrough path и binding modes.

## Slice 2. Assess RDMA feasibility on top of the current baseline

Цель:

- отделить:
  - что уже работает для Mellanox/`mlx5_core`;
  - что лишь помогает DPDK;
  - что обязательно нужно добавить для explicit `RDMA` mode.

Затрагиваемые области:

- внешний `sdn` repo:
  - `api/network.deckhouse.io/v1alpha1/*`
  - `images/agent/.../interface-syncer/*`
  - `images/agent/.../dra-plugin/driver/*`

Проверки:

- manual consistency check between API, status fields, driver binding, CDI
  mounts and CNI handoff

Артефакт:

- feasibility verdict:
  - no-change workaround;
  - bounded prototype path;
  - larger redesign path.

## Slice 3. Research market and upstream references

Цель:

- собрать актуальные внешние reference points по DRA, RDMA exposure и
  `GPU + NIC` placement.

Затрагиваемые области:

- внешние источники:
  - Kubernetes upstream docs / KEPs / blog posts
  - vendor docs for DRA or RDMA integrations
  - scheduler-related reference implementations

Проверки:

- sources must be primary or official where possible
- conclusions must distinguish facts from inference

Артефакт:

- curated source-backed recommendation set.

## Slice 4. Decide on a bounded prototype

Цель:

- если separate `RDMA` mode можно добавить без архитектурного разрастания,
  сделать небольшой prototype;
- иначе явно остановиться на research output и next-step proposal.

Затрагиваемые области:

- при необходимости внешний `sdn` repo:
  - `api/network.deckhouse.io/v1alpha1/*`
  - `images/agent/.../interface-syncer/*`
  - `images/agent/.../dra-plugin/driver/*`
  - `docs/*`

Проверки:

- только узкие targeted checks, соответствующие touched files
- `git diff --check`

Артефакт:

- либо bounded code change, либо documented no-go decision с причинами.

## Slice 5. Record findings

Цель:

- зафиксировать инженерный вывод в bundle и выдать пользователю короткую
  actionable summary.

Затрагиваемые области:

- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/*`

Проверки:

- manual review of the final notes for factual consistency

Артефакт:

- итоговые notes/recommendations в bundle и финальный handoff.

## Slice 6. Derive pod-level `sdn` contract for Mellanox RDMA

Цель:

- реконструировать точный operational path для текущего `sdn`, при котором
  Mellanox NIC доезжает до pod вместе с тем, что нужно для RDMA проверки.

Затрагиваемые области:

- внешний `sdn` repo:
  - `docs/README*.md`
  - `api/network.deckhouse.io/v1alpha1/*`
  - `images/controller/.../pod-claim-webhook/*`
  - `images/controller/.../underlay-controller/*`
  - `images/agent/.../dra-plugin/driver/*`
  - `images/agent/.../cni-server/*`
- live cluster:
  - installed CRD schemas
  - `d8-sdn` objects

Проверки:

- targeted code reading and cluster object inspection
- consistency between docs, CRD schema, and live controller behavior

Артефакт:

- minimal contract:
  - какие `sdn` objects нужны;
  - какой `bindingMode` нужен;
  - какие pod annotations/spec fields обязательны;
  - что должно появиться в pod.

## Slice 7. Validate pod-level passthrough on the live cluster

Цель:

- проверить, что текущая база `sdn` реально может довезти Mellanox device в pod
  и что внутри pod можно увидеть usable RDMA surface.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - dedicated test namespace
  - `UnderlayNetwork` / related `sdn` objects
  - DRA claims/slices
  - validation pods

Проверки:

- `kubectl get/describe` по `UnderlayNetwork`, `ResourceClaim`,
  `ResourceClaimTemplate`, `DeviceClass`, `ResourceSlice`
- inside-pod checks:
  - `ip link`
  - `/dev/infiniband`
  - `ibv_devinfo` / `rdma link`
- if the contract allows it, one narrow connectivity/bandwidth smoke between two
  test pods

Артефакт:

- live validation note with:
  - what worked;
  - what did not work;
  - exact blockers if pod-level RDMA is still incomplete on current `sdn`.

## Slice 8. Prepare the instruction skeleton

Цель:

- собрать доказанный skeleton будущей полной инструкции:
  - host prerequisites;
  - `sdn` setup;
  - pod manifests;
  - pod-level RDMA verification.

Затрагиваемые области:

- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/*`

Проверки:

- manual review for step ordering and factual consistency with live validation

Артефакт:

- draft operational instruction structure that can be expanded into the final
  end-to-end runbook.

## Slice 9. Tune the live GPUDirect benchmark path

Цель:

- понять, почему initial `ib_write_bw --use_cuda=0` на `w1/w3` даёт лишь
  `~56-57 Gb/sec`;
- отделить fabric baseline от GPU-memory bottleneck;
- найти fastest practical benchmark profile for the current hardware.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - `k8s-dvp-w1-gpu.apiac.ru`
  - `k8s-dvp-w3-gpu.apiac.ru`
- bundle docs:
  - `TASK.ru.md`
  - `PLAN.ru.md`
  - `NOTES.ru.md`
  - `SKALA-SDN-RDMA-SMOKE.ru.md`

Проверки:

- topology / PCIe / NUMA inspection
- control benchmark without CUDA
- tuned GPU benchmark with explicit GPU-to-NIC pairing
- `git diff --check`

Артефакт:

- exact benchmark verdict:
  - what reaches near-line-rate;
  - what remains capped;
  - what is likely tunable vs what looks like a platform ceiling.

## Slice 10. Export reusable node artifacts to the laptop

Цель:

- собрать с `k8s-dvp-w3-gpu.apiac.ru` реальные артефакты, использованные для
  `RED OS 8` host setup и benchmark tooling;
- выгрузить их на локальный ноутбук в `~/Downloads` в виде отдельных архивов;
- приложить краткий manifest с содержимым выгрузки и версиями ключевых
  пакетов.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - `k8s-dvp-w3-gpu.apiac.ru`
  - temporary node-debugger pod in `kube-system`
- local workstation
  - `~/Downloads/*`
- bundle docs
  - `TASK.ru.md`
  - `PLAN.ru.md`

Проверки:

- inventory on the host before export
- archive creation must succeed without missing-path errors
- local folder must contain the expected archives and manifest
- `git diff --check`

Артефакт:

- local export directory with:
  - archived node artifacts;
  - package/version manifest;
  - short file inventory.

## Slice 11. Compare pod-level RDMA vs non-RDMA with canonical metrics

Цель:

- собрать понятное и воспроизводимое comparison matrix для одних и тех же
  validation pod;
- отделить общепринятые universal metrics от RDMA-native metrics;
- зафиксировать operator-facing manual, который можно повторить в secure
  contour без `hostPath` и без `privileged`.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - `gpudirect-pod-w1-c2`
  - `gpudirect-pod-w3-01`
- bundle docs:
  - `TASK.ru.md`
  - `PLAN.ru.md`
  - `NOTES.ru.md`
  - `SKALA-SDN-RDMA-SMOKE.ru.md`

Проверки:

- TCP throughput on the direct pod-to-pod underlay path
- TCP latency on the same path
- RDMA throughput on the same path
- RDMA latency on the same path
- `git diff --check`

Артефакт:

- comparison verdict with:
  - exact metrics;
  - exact commands;
  - exact measured numbers;
  - short explanation of how to read the result and what to rerun manually.

## Slice 12. Lock the secure pod contract

Цель:

- проверить, что pod-level `sdn` path не требует `hostPath` и `privileged`;
- зафиксировать реальный minimal security profile для secure contour.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - `sdn-rdma-test` namespace
  - validation pod specs
- bundle docs:
  - `NOTES.ru.md`
  - `SKALA-SDN-RDMA-SMOKE.ru.md`

Проверки:

- inspect live pod specs and capabilities
- confirm absence of `hostPath` and `privileged`
- `git diff --check`

Артефакт:

- short operational contract for secure contour:
  - what the pod actually needs;
  - what was only needed for debug conveniences.

## Slice 13. Normalize the bundle docs and pass final review

Цель:

- привести `SKALA-SDN-RDMA-SMOKE.ru.md` к виду одного понятного operator-facing
  runbook;
- оставить расследование, артефакты сборки и длинную историю только в
  `NOTES.ru.md`;
- пройти финальный review по bundle scope, factual consistency и patchwork
  symptoms.

Затрагиваемые области:

- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/TASK.ru.md`
- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/PLAN.ru.md`
- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`
- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/NOTES.ru.md`

Проверки:

- `git diff --check`
- manual review against `docs/development/REVIEW_CHECKLIST.ru.md`

Артефакт:

- cleaned canonical runbook;
- `NOTES` kept as engineering log;
- explicit residual risks and validation record.

## Slice 14. Validate pod-level GPUDirect RDMA

Цель:

- проверить уже не просто pod-level RDMA, а одновременный `GPU + RDMA` path
  внутри одного pod;
- подтвердить, что межподовый verbs benchmark проходит с `--use_cuda*`;
- отделить real pod-level GPUDirect proof от host-only prerequisite.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - `k8s-dvp-w1-gpu.apiac.ru`
  - `k8s-dvp-w3-gpu.apiac.ru`
  - `sdn-rdma-test` namespace
- bundle docs:
  - `TASK.ru.md`
  - `PLAN.ru.md`
  - `NOTES.ru.md`
  - `SKALA-SDN-RDMA-SMOKE.ru.md`

Проверки:

- create or reuse deterministic GPU `DeviceClass` / `ResourceClaimTemplate`
- create two validation pods with:
  - GPU claim
  - network claim
- inside-pod checks:
  - `/dev/nvidia*`
  - `nvidia-smi`
  - `/dev/infiniband/*`
  - CUDA-enabled `ib_write_bw` / `ib_read_bw --help`
- межподовый benchmark with `--use_cuda*`
- `git diff --check`

Артефакт:

- exact pod-level GPUDirect verdict:
  - succeeded or blocked;
  - exact commands;
  - exact throughput or exact blocker.

## Slice 15. Build a reusable validation toolkit container

Цель:

- перестать зависеть от ручной установки/сборки инструментов внутри debug pod;
- собрать bundle-local image recipe и runner scripts для self-service smoke;
- дать один понятный operational interface для:
  - TCP throughput/latency;
  - plain RDMA throughput/latency;
  - GPUDirect RDMA throughput.

Затрагиваемые области:

- `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/*`
  - new toolkit build context
  - operator-facing docs

Проверки:

- static review of Dockerfile/script contract
- shell syntax check where applicable
- `git diff --check`

Артефакт:

- buildable toolkit directory with:
  - Dockerfile/Containerfile;
  - runner/entrypoint script;
  - manifest templates;
  - README/manual for operator use;
  - short report mode and threshold-based check mode.

## Slice 16. Replace the old RDMA pods with live GPUDirect demo pods

Цель:

- освободить exact network claims, которые сейчас заняты plain-`RDMA` pod;
- поднять в кластере три демонстрационных pod:
  - два combined `GPU + RDMA` pod как межузловую benchmark-пару;
  - один дополнительный combined `GPU + RDMA` pod на spare exact pair;
- оставить в кластере состояние, в котором `GPUDirect` можно показать сразу
  изнутри pod без дополнительной пересборки окружения.

Затрагиваемые области:

- live cluster `k8s-dvp.apiac.ru`
  - `sdn-rdma-test` namespace
  - existing `rdma-pod-*`
  - `ResourceClaimTemplate` для GPU и exact RDMA pair

Проверки:

- delete old plain-`RDMA` pod and confirm claims are released
- create three combined `GPU + RDMA` pod and wait for `Running`
- inside-pod checks:
  - `nvidia-smi -L`
  - `ls -l /dev/infiniband`
  - `ip -br link`
- `git diff --check`

Артефакт:

- three live demo pod:
  - `GPUDirect` benchmark pod on `w1`
  - `GPUDirect` benchmark pod on `w3`
  - extra `GPUDirect` pod on `w1`/`rdma-w1-pair80`
- the docs explicitly distinguish:
  - which two pod form the real inter-node benchmark pair;
  - which third pod is only an additional live `GPU + RDMA` surface on the
    current stand;
- cluster no longer contains the old `rdma-pod-w1-c0` / `rdma-pod-w3-00`
  holders for the same exact RDMA pairs.

## Rollback point

Если prototype path окажется слишком широким:

1. не менять внешний `sdn` code;
2. оставить только исследовательский bundle и findings;
3. вынести actual implementation в отдельный follow-up bundle.

## Final validation

- `git diff --check`
- если будут code changes во внешнем `sdn`, прогнать самую узкую проверку по
  затронутым пакетам и зафиксировать результат отдельно.
