# Plan

## Current phase

Этап 2: `Model` / `ClusterModel`, controller publication/deletion flow и
внутренний `DMCR` publication backend уже live, нужен runtime follow-up по
physical GC без `dmcr` rollout.

## Orchestration

`full`

Задача меняет runtime behavior внутреннего `DMCR`, storage safety и boundary
между hook/templates и in-pod runtime. Нужны read-only reviews до реализации:

- `integration_architect`: проверить безопасность zero-rollout maintenance
  gate, DKP/runtime wiring, global-vs-local ownership and rollout semantics;
- `backend_integrator`: проверить DMCR-specific implementation seam: registry
  HTTP write blocking, direct-upload write blocking, GC runner and storage
  safety;
- `repo_architect`: проверить, что добавляемая boundary не превращается в
  очередной монолит и помогает сократить/упростить cleanup lifecycle.

## Slices

### Slice 1. Reframe delete decision around queued GC

- Цель:
  - убрать ожидание completed physical GC из `FinalizeDelete`;
  - сделать GC request enqueue terminal step для backend artifact delete path.
- Файлы:
  - `images/controller/internal/application/deletion/*`
  - `images/controller/internal/controllers/catalogcleanup/*`
- Проверки:
  - `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`
- Артефакт:
  - controller delete flow снимает finalizer после enqueue request и остаётся
    idempotent на retry.

### Slice 2. Coalesce GC in always-on dmcr-cleaner loop

- Цель:
  - сделать `dmcr-cleaner` постоянным loop sidecar;
  - отделить queued request от switched active GC request;
  - запускать maintenance cycle только после debounce window.
- Файлы:
  - `images/dmcr/internal/garbagecollection/*`
  - `images/dmcr/cmd/dmcr-cleaner/*`
  - `templates/dmcr/deployment.yaml`
- Проверки:
  - `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- Артефакт:
  - queued requests копятся без немедленного GC, active switch поднимается
    только из always-on loop, sidecar command больше не зависит от GC mode.

### Slice 3. Runtime write gate instead of Helm-driven readonly

- Цель:
  - ввести bounded runtime write gate, shared by `dmcr` registry server and
    `dmcr-direct-upload`;
  - активировать gate из `dmcr-cleaner` на время active GC cycle;
  - запускать registry `garbage-collect` без ConfigMap/PodTemplate mutation.
- Файлы:
  - `images/dmcr/internal/maintenance/*` или соседняя justified boundary;
  - `images/dmcr/cmd/dmcr/*`;
  - `images/dmcr/internal/directupload/*`;
  - `images/dmcr/internal/garbagecollection/*`;
  - `templates/dmcr/deployment.yaml`;
  - `templates/dmcr/configmap.yaml`.
- Проверки:
  - `cd images/dmcr && go test ./cmd/dmcr ./internal/directupload ./internal/garbagecollection ./internal/maintenance`
- Артефакт:
  - active GC blocks write paths in-process, reads stay available, and no
    Helm checksum changes are needed for GC activation/deactivation.

### Slice 4. Remove hook-driven GC mode

- Цель:
  - убрать `dmcr_garbage_collection` hook as maintenance switch;
  - удалить `garbageCollectionModeEnabled` from template-driven GC lifecycle;
  - оставить cleanup request Secrets as runtime-only control plane.
- Файлы:
  - `images/hooks/pkg/hooks/dmcr_garbage_collection/*`;
  - `templates/_helpers.tpl`;
  - `templates/dmcr/configmap.yaml`.
- Проверки:
  - `cd images/hooks && go test ./cmd/ai-models-hooks ./pkg/hooks/...`;
  - `make helm-template`;
  - double-render checksum check for `Deployment/dmcr`.
- Артефакт:
  - GC request state no longer changes `aiModels.internal.dmcr.*` render
    output and cannot cause `Deployment/dmcr` rollout.

### Slice 5. Align docs and cleanup surface

- Цель:
  - описать deferred/coalesced zero-rollout DMCR GC;
  - убрать устаревшие mentions of Helm-driven maintenance mode;
  - сократить obsolete cleanup docs/tests where behavior moved to runtime gate.
- Файлы:
  - `images/dmcr/README.md`;
  - `docs/CONFIGURATION.ru.md`;
  - `docs/CONFIGURATION.md`;
  - focused tests around removed hook behavior.
- Проверки:
  - `rg -n "maintenance/read-only|garbageCollectionModeEnabled|dmcr_garbage_collection|dmcr-cleaner" images/dmcr/README.md docs/CONFIGURATION.ru.md docs/CONFIGURATION.md templates images`
- Артефакт:
  - runtime docs не обещают per-delete or per-GC `dmcr` recreation.

### Slice 6. HA ack quorum and gate contraction

- Цель:
  - закрыть advisory-only HA tail: leader cleaner waits for runtime ack quorum
    before destructive cleanup;
  - сохранить boundary: `dmcr` и `dmcr-direct-upload` only read/write local
    `emptyDir` control files and never get Kubernetes credentials;
  - сделать full active cleanup window bounded by one timeout and keep gate
    lease alive for at least that window plus safety margin;
  - удалить dead direct Lease checker and template-owned duplicate gate
    defaults.
- Файлы:
  - `images/dmcr/internal/maintenance/*`;
  - `images/dmcr/internal/garbagecollection/*`;
  - `images/dmcr/cmd/dmcr/*`;
  - `images/dmcr/cmd/dmcr-direct-upload/*`;
  - `images/dmcr/cmd/dmcr-cleaner/*`;
  - `templates/dmcr/deployment.yaml`;
  - `templates/dmcr/rbac.yaml`;
  - `templates/_helpers.tpl`.
- Проверки:
  - `cd images/dmcr && go test ./cmd/dmcr ./cmd/dmcr-cleaner/... ./cmd/dmcr-direct-upload ./internal/maintenance ./internal/directupload ./internal/garbagecollection`;
  - `make helm-template`;
  - render check: only `dmcr-garbage-collection` and `kube-rbac-proxy` mount
    projected serviceaccount token;
  - double-render checksum check for `Deployment/dmcr`.
- Артефакт:
  - active GC is blocked until quorum of pod-scoped ack Leases for the current
    gate sequence exists; stale ack cannot satisfy a new gate sequence.

### Slice 7. DMCR runtime secret restart contract

- Цель:
  - закрыть residual stale rollout tail у `checksum/secret`;
  - оставить checksum helper deterministic, но сузить его до реальных
    DMCR pod runtime inputs;
  - убрать из PodTemplate digest client/controller projection secrets and
    unused CA copy.
- Файлы:
  - `templates/_helpers.tpl`;
  - `templates/dmcr/deployment.yaml`.
- Проверки:
  - `make helm-template`;
  - double-render check for `Deployment/dmcr` `checksum/secret`;
  - `make kubeconform`;
  - `git diff --check`.
- Артефакт:
  - `checksum/secret` зависит от restart-relevant auth/TLS contract:
    resolved auth salt, write/read password checksums, write/read usernames and
    TLS cert/key fingerprints;
  - `ai-models-dmcr-auth-write`, `ai-models-dmcr-auth-read`,
    `ai-models-dmcr-ca`, service host and client-only dockerconfig changes do
    not trigger DMCR rollout.

## Rollback point

После Slice 2 можно безопасно остановиться на queued/coalesced GC with old
Helm-driven maintenance. После начала Slice 3 `dmcr`, `dmcr-direct-upload`,
`dmcr-cleaner`, hook и templates должны оставаться согласованными вместе.

## Final validation

- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./cmd/dmcr ./cmd/dmcr-cleaner/... ./cmd/dmcr-direct-upload ./internal/maintenance ./internal/directupload ./internal/garbagecollection`
- `cd images/hooks && go test ./cmd/ai-models-hooks ./pkg/hooks/...`
- `make helm-template`
- double-render check: `Deployment/dmcr` checksums stable across two renders
- `make kubeconform`
- `git diff --check`
- `make verify`

## Notes

- Slice 1 partially landed in archived `plans/archive/2026/dmcr-cleanup-rollout-restart`:
  controller now creates queued delete-triggered GC requests and
  `checksum/secret` no longer hashes generated Secret template output.
- Read-only reviews completed:
  - `repo_architect`: delete render-time GC switch chain instead of migrating
    it; keep new runtime seam tiny; remove obsolete hook, done annotation and
    cleaner pause mode where safe.
  - `backend_integrator`: pod-local gate is unsafe in HA; use a cluster-visible
    maintenance gate, drop `registryMaintenanceModeEnabled`, block registry and
    direct-upload writes, delete hook/template rollout plumbing.
  - `integration_architect`: do not fork upstream registry; use extension seam.
    Direct-upload must keep `/parts` available for resumability. The strict HA
    ack protocol can be a later tightening if the first implementation uses a
    cluster-visible gate.
- Implementation decision for this slice:
  - add a dedicated `coordination.k8s.io/Lease` maintenance gate separate from
    the executor lease;
  - `dmcr-cleaner` creates/renews the cluster-visible gate immediately before
    physical cleanup, mirrors it into a pod-local `emptyDir` file, and releases
    it after cleanup;
  - only `dmcr-cleaner` has Kubernetes API credentials; `dmcr` and
    `dmcr-direct-upload` read only the local mirror file and reject mutating
    requests while that gate is live;
  - no PodTemplate/ConfigMap mutation is used for GC activation.
- Validation after implementation:
  - `cd images/dmcr && go test ./cmd/dmcr ./cmd/dmcr-cleaner/... ./cmd/dmcr-direct-upload ./internal/maintenance ./internal/directupload ./internal/garbagecollection`;
  - `cd images/hooks && go test ./cmd/ai-models-hooks ./pkg/hooks/...`;
  - `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`;
  - `make helm-template`;
  - double render check for `Deployment/dmcr` `checksum/config` and
    `checksum/secret`;
  - `make kubeconform`;
  - `git diff --check`;
  - `make verify`.
- Post-review fixes:
  - runtime ack path now has writable `dmcr-maintenance-gate` `emptyDir` mount
    in `dmcr` and `dmcr-direct-upload`, without adding Kubernetes token mounts;
  - `LeaseGate.Activate` now refuses active gates held by another identity
    instead of treating contention as success;
  - added focused test for conflicting maintenance gate activation.
- Post-review validation:
  - `cd images/dmcr && go test ./internal/maintenance ./internal/garbagecollection`;
  - `cd images/dmcr && go test ./cmd/dmcr ./cmd/dmcr-cleaner/... ./cmd/dmcr-direct-upload ./internal/maintenance ./internal/directupload ./internal/garbagecollection`;
  - `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`;
  - `make helm-template`;
  - `make kubeconform`;
  - render evidence: `dmcr` and `dmcr-direct-upload` still do not mount
    `dmcr-kube-api-access`, and their `dmcr-maintenance-gate` mount is writable;
  - `git diff --check`;
  - `make verify`.
- First final review adjustment:
  - сетевые контейнеры `dmcr` и `dmcr-direct-upload` не получают Kubernetes
    token and read only the pod-local maintenance gate file;
  - только `dmcr-garbage-collection` mirrors the cluster Lease into the shared
    `emptyDir`;
  - current residual HA risk is bounded by mirror interval plus maintenance
    delay, not by a strict per-pod acknowledgement protocol.
- Continuation read-only findings for Slice 6:
  - `integration_architect`: current gate is advisory across HA replicas;
    strict per-pod ack quorum is the smallest DKP-safe tightening, but ack must
    flow through local files and cleaner-owned Leases, not Kubernetes token in
    network containers or readiness/service routing.
  - `backend_integrator`: full destructive cleanup window must be bounded, not
    only the registry subprocess; maintenance gate lifetime must cover that
    whole window.
  - `repo_architect`: `LeaseChecker` is dead production code after file mirror;
    gate defaults should move out of Helm where they only repeat binary
    defaults; duplicated Lease helper logic is structural drift to reduce in a
    follow-up or shared utility.
- Slice 6 implementation evidence:
  - added gate sequence to the cluster-visible maintenance Lease and mirrored
    gate file;
  - `dmcr` and `dmcr-direct-upload` write pod-local ack files after observing
    the current gate sequence;
  - each cleaner sidecar mirrors complete local runtime acks into a pod-scoped
    Lease;
  - leader cleaner waits for `--maintenance-gate-ack-quorum` before destructive
    cleanup and releases the gate without cleanup when quorum is missing;
  - active cleanup now runs under one full-cycle timeout and gate duration is
    clamped to that timeout plus safety margin;
  - removed direct `LeaseChecker` and legacy `dmcr-gc-done` / `Complete`
    protocol from live code.
- Validation after Slice 6:
  - `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup`;
  - `cd images/dmcr && go test ./cmd/dmcr ./cmd/dmcr-cleaner/... ./cmd/dmcr-direct-upload ./internal/maintenance ./internal/directupload ./internal/garbagecollection`;
  - `cd images/hooks && go test ./cmd/ai-models-hooks ./pkg/hooks/...`;
  - `make helm-template`;
  - `make kubeconform`;
  - render evidence: HA `Deployment/dmcr` has `--maintenance-gate-ack-quorum=2`;
  - render evidence: `dmcr` and `dmcr-direct-upload` do not mount
    `dmcr-kube-api-access`, while `dmcr-garbage-collection` and
    `kube-rbac-proxy` do;
  - double render check: `Deployment/dmcr` `checksum/config` and
    `checksum/secret` stayed stable;
  - `git diff --check`;
  - `make verify`.
- Continuation read-only findings for Slice 7:
  - `repo_architect`: helper must stay dedicated because re-including
    `dmcr/secret.yaml` re-executes random TLS/password generators; however it
    currently mixes pod runtime inputs with client/controller distribution
    secrets and causes unnecessary rollouts.
  - `integration_architect`: live `lookup` data alone leaves stale rollout
    risk when desired auth/TLS material changes in the same render; the helper
    should hash a narrow restart contract and use stable bootstrap sentinels
    when Secrets do not exist yet.
- Slice 7 implementation evidence:
  - `dmcrPodSecretChecksum` now hashes only chart-managed restart checksums
    stored on `ai-models-dmcr-auth` and `ai-models-dmcr-tls`;
  - auth projection Secrets, DMCR CA copy, service host and client-only
    dockerconfig fields no longer participate in `Deployment/dmcr`
    `checksum/secret`;
  - absent auth/TLS Secrets use stable bootstrap checksums on first install and
    deterministic recovery checksums derived from the previous PodTemplate
    checksum when a live Deployment already exists.
- Validation after Slice 7:
  - `make helm-template`;
  - double render check for every `tools/kubeconform/renders/*.yaml`
    `checksum/secret`;
  - `make kubeconform`;
  - `git diff --check`;
  - `make verify`.
- Final reviewer finding after initial Slice 7:
  - bootstrap sentinels in `Deployment/dmcr` without matching Secret-level
    persisted restart contract could cause one extra post-bootstrap rollout
    when the next render sees real generated Secret data.
- Post-review fix for Slice 7:
  - added `ai.deckhouse.io/dmcr-pod-secret-checksum` annotations to runtime
    auth/TLS Secrets and made `Deployment/dmcr` depend on those annotations;
  - unannotated pre-existing Secrets still fall back to live data fingerprints
    for one safe migration rollout;
  - missing Secrets with an existing Deployment produce a deterministic
    recovery checksum from the previous PodTemplate checksum, causing a single
    recovery rollout without a second follow-up rollout;
  - added `tools/helm-tests` validation for the restart annotation contract and
    projection/CA Secret exclusion.
- Post-review validation for Slice 7:
  - `python3 -m unittest tools/helm-tests/validate_renders_test.py`;
  - `make helm-template`;
  - double render check for `checksum/secret` and
    `ai.deckhouse.io/dmcr-pod-secret-checksum`;
  - `make kubeconform`;
  - `git diff --check`;
  - `make verify`.
- Final focused reviewer result for Slice 7:
  - previous Medium finding is closed: bootstrap sentinel-to-generated-data
    extra rollout path is removed;
  - `Deployment/dmcr` restart contract is tied only to persisted runtime
    auth/TLS Secret annotations, and projection/CA/client-only Secrets remain
    outside the restart checksum.
- Residual testing gap:
  - no explicit stateful multi-render test covers live `lookup` transitions
    (`missing Secret -> bootstrap annotation -> stable rerender` and
    `existing Deployment checksum -> missing Secret recovery -> stable rerender`);
    current evidence is static render validation plus helper reasoning.
