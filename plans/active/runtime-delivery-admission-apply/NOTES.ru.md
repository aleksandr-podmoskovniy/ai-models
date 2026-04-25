## 1. Initial observation

- Live smoke left one stale `BackOff` event from the first consumer deployment
  revision.
- The final revision was healthy after `workloaddelivery` patched the pod
  template.
- This is consistent with an asynchronous controller race: `Deployment`
  controller can create ReplicaSet/Pod before ai-models controller applies
  runtime delivery mutation.

## 2. Working hypothesis

- Deterministic first rollout requires admission-time mutation or an equivalent
  synchronous entrypoint.
- Existing reconcile path should remain as recovery/backfill because admission
  cannot repair already persisted legacy objects and may be temporarily
  unavailable.

## 3. Read-only subagent findings

### 3.1 API designer

- `ai.deckhouse.io/clustermodel` is already part of the workload reference
  surface and must be handled consistently with `ai.deckhouse.io/model`.
- Existing controller scope includes `Deployment`, `StatefulSet`, `DaemonSet`
  and `CronJob`; admission/fallback semantics must not silently differ by kind.
- Full admission mutation should reject unresolved references if it claims
  deterministic delivery, but current pre-create UX may still need pending
  workloads.
- Existing resolved artifact annotations include backend URI leakage; do not
  make that a larger public API surface.

### 3.2 Repo architect

- Full create-time `modeldelivery.ApplyToPodTemplate` is incompatible with
  admission because it creates projected registry/image-pull Secrets named by
  owner UID, and UID is not available before persistence.
- Admission ownership should stay inside
  `images/controller/internal/controllers/workloaddelivery`.
- `bootstrap` should only wire process/runtime options; `modeldelivery` should
  own pure pod-template helpers.
- Safe narrowed implementation is admission-time rollout gating, with
  controller-owned secret projection and cleanup remaining in the fallback
  reconciler.

### 3.3 Integration architect

- The controller shell currently has no webhook listener, Service port, cert
  mount or admission configuration.
- If a webhook is added, it should live in the existing controller deployment
  with a dedicated internal TLS Secret and no public ingress.
- Human-facing RBAC must remain unchanged; controller service-account RBAC is
  already internal-only and broad enough for this slice.
- Failure policy tradeoff:
  - `Ignore` preserves cluster operability and falls back to current behavior
    on webhook outage;
  - `Fail` is deterministic but blocks annotated workload rollout during
    controller/TLS outages.

## 4. Implementation decision

- Do not implement full admission-time runtime delivery mutation in this
  slice.
- Add a managed `PodSchedulingGate` named `ai.deckhouse.io/model-delivery`.
- Admission will only add that gate to opt-in `Deployment`, `StatefulSet` and
  `DaemonSet` pod templates and will not create/read/write registry projection
  Secrets.
- Controller fallback will:
  - keep/add the gate while referenced `Model` / `ClusterModel` is missing or
    not consumable;
  - create projected registry/image-pull Secrets only after workload UID exists;
  - apply runtime delivery and remove the gate in the same persisted workload
    patch when the model is ready.
- This preserves pending-consumer UX and prevents the first unmutated Pod from
  starting when admission is available.
- `CronJob` is excluded from admission gating in this slice because a `Job`
  created before the controller patches the `CronJob` template could inherit
  the gate and remain stuck. CronJob delivery remains controller-fallback-only
  until a dedicated Job-level recovery design exists.

## 5. Implemented slice

- Added `modeldelivery` managed scheduling-gate helpers:
  - gate name: `ai.deckhouse.io/model-delivery`;
  - `EnsureSchedulingGate`;
  - `RemoveSchedulingGate`;
  - `HasSchedulingGate`.
- `modeldelivery.applyRendered` now removes the managed scheduling gate when
  runtime delivery is rendered.
- `workloaddelivery` now treats the scheduling gate as managed template state.
- When a referenced model is missing or not ready, controller fallback removes
  stale runtime delivery state, deletes projected registry/image-pull Secrets
  and keeps/adds the scheduling gate instead of letting an unmutated Pod start.
- Added bounded mutating admission handler under `workloaddelivery`:
  - supports `Deployment`, `StatefulSet`, `DaemonSet`;
  - adds only the scheduling gate;
  - does not resolve models;
  - does not create or read projected registry Secrets;
  - denies ambiguous references with both `ai.deckhouse.io/model` and
    `ai.deckhouse.io/clustermodel`.
- Controller bootstrap now runs a controller-runtime webhook server on port
  `9443` with certs under `/tmp/k8s-webhook-server/serving-certs`.
- Templates now render:
  - internal webhook TLS Secret;
  - controller webhook port and cert mount;
  - controller Service webhook port;
  - `MutatingWebhookConfiguration` for annotated `Deployment`, `StatefulSet`
    and `DaemonSet` objects with `failurePolicy: Ignore`.
- Access coverage evidence:
  - human-facing RBAC fragments are not changed;
  - the webhook is an internal controller Service endpoint only;
  - no human role gains Secret, `pods/log`, `pods/exec`, `pods/attach`,
    `pods/portforward`, `status`, `finalizers` or internal runtime-object
    access;
  - controller service-account RBAC stays internal-only and is not aggregated
    into user roles.

## 6. Validation

- `cd images/controller && go test -count=1 ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/bootstrap ./cmd/ai-models-controller`;
- `make helm-template`;
- `make kubeconform`;
- `make verify`;
- `git diff --check`.

All listed checks passed.

## 7. Residual risks

- `failurePolicy: Ignore` intentionally preserves cluster operability on
  controller/webhook outage and falls back to current async behavior.
- `CronJob` still has the old async first-run race; closing it safely requires
  a separate Job-level recovery/admission design, not only a CronJob template
  gate.
- Full admission-time runtime mutation is still blocked by UID-derived
  projected Secret ownership; this was deliberately not changed in this slice.
- Workload annotations still include resolved artifact URI from the current
  runtime delivery contract; this remains a separate public-surface cleanup
  candidate.

## 8. Review-gate fixes

- Final reviewer found that CronJob admission gating was unsafe: a first `Job`
  could inherit the scheduling gate before the controller patched the parent
  `CronJob`, and the controller does not own already-created `Job` objects in
  this slice.
- Fixed by removing `CronJob` from the admission webhook and handler; existing
  CronJob delivery remains controller-fallback-only.
- Fixed generic admission-surface drift by adding Kubernetes 1.30
  `matchConditions` so the webhook is invoked only for objects carrying
  `ai.deckhouse.io/model` or `ai.deckhouse.io/clustermodel`.
- Added focused coverage for:
  - `ClusterModel` runtime delivery;
  - missing `Model` keeping the scheduling gate;
  - pending `Model` removing stale delivery state, projected Secrets and
    imagePullSecret references while keeping the gate;
  - CronJob fallback applying delivery without an admission scheduling gate.
- Reviewer recheck after these fixes reported no remaining blockers.
