# Plan: pre-rollout defect closure

## 1. Current phase

Phase 1/2 pre-rollout hardening. The task closes known code defects before the
next cluster rollout and e2e pass.

## 2. Orchestration

Mode: `full`.

Reason: this changes metadata/profile semantics, workload controller behavior,
DMCR observability and runtime logging. Read-only reviews:

- `api_designer` for `any-to-any` profile semantics and public status safety.
- `integration_architect` for workload delivery failure/backoff UX.
- `backend_integrator` for DMCR GC output/logging and field dictionary.

## 3. Active bundle disposition

- `capacity-cache-admission-hardening` — keep until the current storage/cache
  changes are committed; latest slice is implemented.
- `live-e2e-ha-validation` — keep as the canonical post-rollout runbook.
- `observability-signal-hardening` — keep; this task consumes part of its
  pending log/DMCR cleanup but does not replace the live metrics workstream.
- `ray-a30-ai-models-registry-cutover` — keep for external manifest/load
  validation, not part of this code slice.
- `pre-rollout-defect-closure` — current implementation workstream.

## 4. Slices

### Slice 1. `any-to-any` model profile

Goal:

- do not expose broad Hugging Face `any-to-any` as public declared task;
- map it to conservative catalog endpoint/features only when local checkpoint
  evidence confirms a multimodal model.

Files:

- `images/controller/internal/adapters/modelprofile/common/*`
- `images/controller/internal/adapters/modelprofile/safetensors/*`
- `images/controller/internal/dataplane/publishworker/*` if profile projection
  needs tests.

Checks:

- targeted modelprofile/publishworker tests.

Status:

- implemented as controller semantics only: `pipeline_tag=any-to-any` becomes
  `TaskHint`, stable HF task labels stay `SourceDeclaredTask`;
- Safetensors derives `image-text-to-text` with multimodal features only when
  `vision_config` / equivalent checkpoint evidence exists;
- weak `any-to-any` falls back to derived text/chat semantics.

### Slice 2. Workload delivery quiet failure UX

Goal:

- keep correct rejection for missing cache mount;
- avoid repeated identical warning events/requeues.

Files:

- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`

Checks:

- targeted workload delivery tests.

Status:

- implemented typed `WorkloadContractError`;
- invalid workload spec writes private blocked annotations and scheduling gate;
- controller returns success for stable invalid spec and emits event only on
  transition;
- blocked state clears when workload mount becomes valid.

### Slice 3. DMCR GC output and SDK warning cleanup

Goal:

- remove raw `registryOutput` from success persistence;
- keep bounded failure/debug evidence;
- suppress known S3 checksum warning noise from normal operator logs.

Files:

- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/internal/logging/*`

Checks:

- targeted DMCR tests.

Status:

- success path persists registry output fingerprint/line count only;
- failure path bounds huge command output to line count, SHA256 and first/last
  lines;
- S3 clients use checksum calculation/validation `WhenRequired` to avoid
  normal-path checksum warning spam against S3-compatible backends.

### Slice 4. Log field dictionary tightening

Goal:

- normalize key runtime fields on touched paths:
  `duration_ms`, `artifactDigest`, `artifactURI`, `sourceType`,
  `request`, `repository`, `phase`, `err`.

Files:

- controller runtime dataplane logging helpers/call sites touched by this task;
- DMCR GC/direct-upload logging touched by this task.

Checks:

- targeted logging tests and package tests.

Status:

- direct-upload uses `slog` through the DMCR default logger instead of
  `log.Printf`;
- touched digest fields use `artifactDigest`;
- changed code keeps duration as `durationMs`, which is normalized by current
  logger handlers to `duration_ms`.

## 5. Read-only subagent findings

- `api_designer`: broad `any-to-any` must not leak as public declared task;
  implement only controller-side normalization and keep CRD unchanged.
- `integration_architect`: missing cache mount is a non-retriable workload
  contract violation; persist workload-local blocked state and emit event only
  on state transition.
- `backend_integrator`: DMCR cleanup must include direct-upload logs and S3
  checksum settings, not just GC success persistence.
- final `reviewer`: live HF path carries broad `any-to-any` through `TaskHint`,
  not `SourceDeclaredTask`; blocked delivery state must clear both on successful
  apply and when a fixed workload is still pending on model readiness; all
  user-fixable workload contract errors should use the same non-retrying
  blocked UX.

Resolution:

- Safetensors broad-task inference now checks both `SourceDeclaredTask` and
  `TaskHint`.
- Workload blocked annotations are cleared on both successful delivery apply
  and normal pending-state reconciliation.
- Cache mount, volume topology, shared PVC and mount-path contract errors are
  typed as `WorkloadContractError`.

## 6. Rollback point

Rollback is reverting this bundle's code changes. No live state migration or
public API change is introduced.

## 7. Final validation

- `cd images/controller && go test ./internal/adapters/modelprofile/safetensors ./internal/dataplane/publishworker ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery`
- `cd images/controller && go test ./...`
- `cd images/dmcr && go test ./...`
- `git diff --check`
- `make verify`
- `review-gate`
- final `reviewer` pass completed; findings resolved before final validation.
