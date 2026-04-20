# Runtime baseline reset to prod-ready target

## 1. Заголовок

Сбросить runtime baseline `ai-models` к прод-готовой целевой картине без
legacy runtime shell и лишних intermediate seams

## 2. Контекст

В live codebase уже landed:

- public DKP API `Model` / `ClusterModel`;
- controller-owned publication в native OCI `ModelPack`;
- `DMCR` direct-upload contract для тяжёлых layer blobs;
- managed node-cache substrate и stable per-node runtime plane.

Но repo-local surfaces всё ещё разъехались:

- node-cache runtime держит отдельный per-node intent `ConfigMap` contract,
  который усложняет architecture surface и не выглядит defendable minimal
  boundary для текущего stage;
- active bundles и часть docs всё ещё описывают stale intermediate paths;
- структура и workflow wording местами отстают от live runtime baseline.

Пользователь явно требует:

- убрать лишние legacy/intermediate seams;
- довести live runtime baseline до cleaner production-ready формы;
- делать это с минимальным объёмом кода, а не через наращивание ещё одной
  прослойки.

## 3. Постановка задачи

Нужно провести bounded reset текущего runtime baseline:

- убрать `per-node intent ConfigMap` как отдельный controller-owned contract;
- заменить его более прямым node-runtime contract без лишнего persisted mirror
  слоя;
- выровнять runtime tree, docs, active bundles и CI wording под уже landed
  native publication/runtime baseline;
- не возвращать historical backend / `KitOps` / `PostgreSQL` narrative в live
  repo surfaces.

## 4. Scope

- `images/controller/internal/nodecacheintent/*`
- `images/controller/internal/controllers/nodecacheintent/*`
- `images/controller/internal/adapters/k8s/nodecacheintent/*`
- `images/controller/internal/nodecache/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/bootstrap/*`
- `templates/controller/*`
- `templates/node-cache-runtime/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `README.md`
- `README.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `.gitlab-ci.yml`
- `plans/active/node-local-cache-runtime-delivery/*`
- `plans/active/phase2-model-distribution-architecture/*`
- `plans/archive/2026/phase2-runtime-followups/*`
- `plans/archive/2026/publication-source-acquisition-modes/*`
- `plans/archive/2026/*`
- новый task bundle для этой задачи

## 5. Non-goals

- не реализовывать сейчас весь `DMZ` distribution tier;
- не проектировать новый public `Model.spec` contract;
- не тащить в этот bundle governance edits для `AGENTS.md`, `.codex/*` и
  workflow-governance docs;
- не делать blanket refactor всего controller tree без прямой пользы для
  current prod baseline;
- не обещать workload-facing shared mount service, если этот contract ещё не
  landed в code.

## 6. Затрагиваемые области

- runtime node-cache contract;
- controller bootstrap and watches;
- runtime Pod/PVC env and RBAC shaping;
- product/runtime docs and README surfaces;
- active phase-2 bundles, которые уже не совпадают с landed baseline;
- CI human-facing wording.

## 7. Критерии приёмки

- runtime baseline больше не держит отдельный per-node intent `ConfigMap`
  contract и dedicated `nodecacheintent` controller surface;
- live code больше не держит misleading helper package/boundary name
  `internal/nodecacheintent`;
- node-cache runtime получает desired artifact set напрямую из live cluster
  truth с bounded minimal contract;
- `images/controller` structure/docs перечисляют только живые packages and
  owner boundaries;
- docs и README surfaces больше не несут stale `PostgreSQL`, backend-first или
  `KitOps`-baseline wording там, где live runtime уже другой;
- active phase-2 bundles больше не маскируют current runtime baseline под
  historical `KitOps` or `DaemonSet` steps;
- oversized stale active bundle больше не остаётся в `plans/active` как
  parallel historical source of truth;
- `.gitlab-ci.yml` больше не инструктирует пользователя про `PostgreSQL`
  baseline;
- узкие runtime tests проходят;
- `make helm-template`, `make kubeconform` и `make verify` проходят.

## 8. Риски

- можно заменить `ConfigMap`-intent слой на другой equally-artificial mirror
  contract и не сократить real complexity;
- можно случайно смешать runtime desired-set extraction с workload mutation
  policy и получить новую giant boundary;
- можно вычистить docs быстрее, чем код, и получить reverse drift;
- active bundles легко превратить в partial rewrite instead of compact current
  working surfaces.
