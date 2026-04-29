# CSI workload UX contract plan

## Active bundle disposition

- `live-e2e-ha-validation`: keep. Это следующий executable validation stream
  после выката runtime changes.
- `modelpack-efficient-compression-design`: keep. Отдельный research/design
  stream по storage efficiency, не пересекается с CSI UX implementation.
- `observability-signal-hardening`: keep. Отдельный observability stream,
  нужен для логов/метрик и не должен смешиваться с workload delivery contract.

## Orchestration

Mode: `light`.

Read-only review before implementation:

- `integration_architect`: CSI/Vault/Stronghold/Deckhouse UX, storage/HA
  boundaries.
- `api_designer`: annotations, K8s API conventions, spec/status split and
  failure UX.

Reviewer findings captured:

- Do not expose a public self-rendered CSI contract now: raw
  `resolved-*` annotations are user-mutable and must not be trusted by
  node-cache runtime.
- Supported UX is annotation-only on supported Kubernetes workload Pod
  templates. Controller owns CSI volume injection and internal attributes.
- If a future external renderer is needed, it must use a trusted delivery
  ticket/API, not copied Pod annotations.
- CronJob admission must match controller support, otherwise the first Job can
  start before the controller stamps delivery state.

## Reference scan

- Kubernetes CSI inline/ephemeral volume semantics.
- Secrets Store CSI Driver and Vault CSI Provider UX: workload declares intent,
  driver/provider owns mount-time materialization, sync-to-Secret is explicit.
- Local Deckhouse/virtualization/Stronghold patterns: avoid leaking third-party
  object internals into module-specific controllers.

## Slice 1. Contract decision

Files:

- `plans/active/csi-workload-ux-contract/PLAN.ru.md`
- docs under `docs/*`
- e2e runbook under `plans/active/live-e2e-ha-validation/*`

Decision:

- One annotation is the preferred built-in workload UX.
- Internal CSI attributes are controller-stamped.
- External controllers get no raw CSI escape hatch in this slice; they must use
  supported PodTemplate mutation or a future trusted ticket/API.
- Node-cache runtime trusts only controller-signed resolved delivery
  annotations.

Validation:

- Review notes captured in this plan.

## Slice 2. Generic mutation implementation

Files:

- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- tests in the same packages.

Implementation:

- Auto-create missing node-cache CSI volumes for managed SharedDirect.
- Stamp internal attributes for single and alias model volumes.
- Keep explicit wrong-volume rejection.
- Keep no nodeSelector/affinity mutation.
- Sign resolved delivery annotations with module-private HMAC.
- Verify the signature in node-cache runtime before desired-artifact prefetch
  and CSI NodePublish authorization.
- Add CronJob to the workload delivery admission webhook.

Validation:

- Focused Go tests for modeldelivery and workloaddelivery.

## Slice 3. Docs and e2e alignment

Files:

- `docs/CONFIGURATION*.md`
- `docs/USER_GUIDE*.md`
- `docs/EXAMPLES*.md`
- `docs/FAQ*.md`
- `plans/active/live-e2e-ha-validation/A30_VLLM_SHARED_DIRECT.ru.md`
- `plans/active/live-e2e-ha-validation/RUNBOOK.ru.md`

Implementation:

- Show annotation-first UX.
- Remove explicit CSI volume from user examples.
- State no PVC fallback and no third-party CRD special cases.

Validation:

- `git diff --check`

## Slice 4. Final review

- Run focused tests.
- Run `make verify` if feasible.
- Run `review-gate`; if delegation produced substantial architecture
  findings, run final reviewer.
