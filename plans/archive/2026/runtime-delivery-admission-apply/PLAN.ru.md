## 1. Current phase

Phase 1 publication/runtime baseline hardening.

Цель этого bundle: сделать текущий `MaterializeBridge` delivery path
детерминированным на create/update workload path без перехода к phase-2
distribution topology.

## 2. Orchestration

`full`

Причина:

- задача меняет runtime entrypoint и controller boundaries;
- admission/TLS/templates/RBAC имеют operational risk;
- решение должно остаться DKP module pattern, а не kubebuilder repo drift;
- пользователь разрешил subagents.

Read-only reviews до реализации:

- `repo_architect`: проверить, где должна жить admission boundary и как не
  размазать workload delivery по пакетам;
- `integration_architect`: проверить DKP/runtime wiring, TLS/service/RBAC и
  failure-policy tradeoff;
- `api_designer`: проверить Kubernetes API semantics, model readiness behavior
  and no public contract drift.

Финально после реализации:

- `review-gate`;
- `reviewer`, потому что task substantial и использует delegation.

## 3. Slices

### Slice 1. Inspect and design deterministic entrypoint

Цель:

- подтвердить текущую гонку по коду;
- выбрать минимальный entrypoint для admission-time mutation;
- зафиксировать решения subagents.

Файлы:

- `plans/active/runtime-delivery-admission-apply/NOTES.ru.md`;
- read-only inspection of `images/controller/internal/controllers/workloaddelivery`;
- read-only inspection of `images/controller/internal/adapters/k8s/modeldelivery`;
- read-only inspection of `images/controller/internal/bootstrap`,
  `images/controller/cmd/ai-models-controller`, `templates/controller`.

Проверки:

- no code changes before read-only findings are recorded.

Артефакт:

- compact decision note: mutate vs reject behavior, failure policy, code
  boundary.

### Slice 2. Share workload delivery gating between controller and admission

Цель:

- добавить общий managed scheduling-gate helper в runtime delivery boundary;
- admission ставит только gate, без registry Secret projection и без
  side-effecting runtime mutation;
- controller fallback снимает gate только вместе с готовым runtime delivery
  patch.

Файлы:

- `images/controller/internal/controllers/workloaddelivery/*`;
- `images/controller/internal/adapters/k8s/modeldelivery/*`;

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery`.

Артефакт:

- controller fallback продолжает работать, тесты зелёные.

### Slice 3. Admission-time workload scheduling gate

Цель:

- добавить bounded admission handler for supported rollout workload kinds:
  `Deployment`, `StatefulSet`, `DaemonSet`;
- handler only adds a managed `PodSchedulingGate` for opt-in workloads;
- not-ready/not-found behavior is owned by controller fallback and keeps the
  gate until the referenced model becomes consumable.

Файлы:

- new or existing controller internal package under `images/controller/internal`;
- `images/controller/internal/bootstrap/*`;
- `images/controller/cmd/ai-models-controller/*`;
- focused tests.

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery ./internal/bootstrap ./cmd/ai-models-controller`.

Артефакт:

- model-annotated `Deployment`, `StatefulSet` and `DaemonSet` get a scheduling
  gate synchronously before persistence, while side-effecting delivery stays
  controller-owned; `CronJob` stays controller-fallback-only in this slice.

### Slice 4. DKP template wiring

Цель:

- добавить только необходимые Service/RBAC/cert/webhook objects;
- сохранить human-facing RBAC unchanged;
- не добавить generic admission surface beyond model-annotated rollout
  workloads.

Файлы:

- `templates/controller/*`;
- possible new `templates/controller/webhook*.yaml`;
- `tools/helm-tests/*` only if rendered guardrail is needed.

Проверки:

- `make helm-template`;
- `make kubeconform`.

Артефакт:

- rendered module contains bounded admission wiring for annotated
  `Deployment`, `StatefulSet` and `DaemonSet` only, with internal controller
  RBAC unchanged.

### Slice 5. Smoke and cleanup

Цель:

- проверить, что first stale pod больше не появляется для ready model;
- убрать лишний локальный code split if already made obsolete by shared
  admission/controller path.

Проверки:

- focused local tests;
- if cluster build is available, live smoke with model-annotated deployment;
- `git diff --check`.

Артефакт:

- NOTES contain exact result and remaining risks.

## 4. Rollback point

After Slice 2: controller-side scheduling-gate cleanup can be kept
independently. If admission wiring proves too large/risky for this pass, stop
with controller-owned gating and a documented webhook implementation plan.

## 5. Final validation

- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery ./internal/bootstrap ./cmd/ai-models-controller`;
- `make helm-template`;
- `make kubeconform`;
- `git diff --check`;
- `review-gate`;
- final read-only `reviewer`.
