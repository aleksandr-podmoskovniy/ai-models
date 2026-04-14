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

## Rollback point

Если prototype path окажется слишком широким:

1. не менять внешний `sdn` code;
2. оставить только исследовательский bundle и findings;
3. вынести actual implementation в отдельный follow-up bundle.

## Final validation

- `git diff --check`
- если будут code changes во внешнем `sdn`, прогнать самую узкую проверку по
  затронутым пакетам и зафиксировать результат отдельно.
