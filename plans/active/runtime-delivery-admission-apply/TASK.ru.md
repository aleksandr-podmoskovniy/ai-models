## 1. Заголовок

Детерминированная workload runtime delivery mutation без первого stale Pod

## 2. Контекст

Live smoke после обновления подтвердил рабочий publication/runtime baseline:
маленькая `Model` дошла до `Ready`, artifact прочитан из `DMCR`, workload
получил `MaterializeBridge` delivery и materialized files.

Оставшийся архитектурный хвост: текущий `workloaddelivery` controller
асинхронно патчит `Deployment` после создания. Kubernetes `Deployment`
controller может успеть создать первый ReplicaSet/Pod до patch, что даёт
transient stale `BackOff` event на первом revision. Финальный revision здоровый,
но сам паттерн недетерминированный: runtime contract применяется после того,
как workload controller уже мог начать rollout.

## 3. Постановка задачи

Нужно закрыть этот хвост в phase-1 baseline:

- добавить bounded admission-time scheduling gate для model-annotated workload
  pod templates;
- оставить существующий `workloaddelivery` controller как recovery/backfill
  для уже существующих объектов и webhook outage cases;
- не выводить внутренний `DMCR` или runtime backend в публичный контракт;
- заодно сжать локальный workload-delivery код там, где это убирает
  дублирование, но не делать blanket rewrite всего controller tree.

## 4. Scope

- новый bundle `plans/active/runtime-delivery-admission-apply/*`;
- controller admission/bootstrap code for model-annotated workloads;
- `images/controller/internal/controllers/workloaddelivery/*`;
- `images/controller/internal/adapters/k8s/modeldelivery/*`;
- controller templates/RBAC/service/certs only if needed for webhook wiring;
- focused tests for admission-time mutation and controller fallback.

## 5. Non-goals

- не проектировать node-local cache daemon или `DMZ` registry tier;
- не менять public `Model` / `ClusterModel` spec/status;
- не вводить public knobs в workload annotations сверх текущих
  `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel`;
- не удалять controller fallback path в этом slice;
- не делать blanket сокращение всего репозитория до 25k LOC в одном diff;
- не менять RBAC для human-facing roles без отдельной RBAC task.

## 6. Затрагиваемые области

- admission scheduling gate entrypoint for `Deployment`, `StatefulSet` and
  `DaemonSet`;
- existing controller fallback for `Deployment`, `StatefulSet`, `DaemonSet`
  and `CronJob`;
- controller manager/bootstrap;
- service-account RBAC for internal model/status reads if admission path needs
  them explicitly;
- workload-delivery tests and evidence docs.

## 7. Критерии приёмки

- For a model-annotated `Deployment`, `StatefulSet` or `DaemonSet`, the first
  pod template is scheduling-gated before workload persistence / first rollout.
- For a model-annotated workload referencing a ready `Model`, controller
  fallback applies runtime delivery and removes the scheduling gate in the same
  persisted workload patch.
- Existing reconcile controller remains able to repair legacy or unmutated
  workloads.
- Admission path and controller fallback share the same resolver/mutator logic
  instead of duplicating object shaping.
- No public API field changes.
- User-facing RBAC remains unchanged; service-account RBAC is internal-only and
  not aggregated into human roles.
- The webhook is an internal controller service endpoint only: no human-facing
  access level receives Secret, `pods/log`, `pods/exec`, `pods/attach`,
  `pods/portforward`, `status`, `finalizers` or internal runtime-object access
  through this slice.
- Focused tests cover:
  - admission scheduling-gate mutation for supported model-annotated workload
    kinds;
  - no side-effecting registry Secret projection in admission;
  - fallback still patches an unmutated existing workload;
  - not-ready/not-found model behavior keeps the scheduling gate instead of
    launching an unmutated pod.
- Relevant checks pass:
  - `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery`;
  - `make helm-template`;
  - `make kubeconform`;
  - `git diff --check`.

## 8. Риски

- Mutating webhooks add operational coupling to controller availability and TLS
  wiring; failure policy must be chosen consciously.
- If admission rejects not-ready models, precreating workloads before model
  publication becomes impossible; if it allows them, the old eventual tail
  remains for that case.
- Duplicating resolver logic between webhook and controller would make the
  architecture worse while hiding behind a new entrypoint.
- Over-scoping this into generic workload admission would turn the module into
  a kubebuilder-style app instead of a DKP module.
- `CronJob` is intentionally excluded from admission-time scheduling gate in
  this slice: a `Job` created before controller patch could inherit a gate that
  this controller does not own. CronJob remains controller-fallback-only until
  there is a dedicated Job-level recovery design.
