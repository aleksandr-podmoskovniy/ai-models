# NOTES

## Drift matrix

Заполняется по итогам audit against restored ADR.

Формат:

- area
- restored ADR expectation
- current implementation / docs
- impact
- decision: `rework-now` | `defer` | `keep-out-of-contract`

### Public API shape

- area: public CRD spec/status shape
- restored ADR expectation:
  - `spec.modelType`, `spec.artifact`, `spec.usagePolicy`, `spec.launchPolicy`
  - first iteration artifact source is OCI-only
  - controller writes technical profile into `status.resolved`
  - reference: `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:121`
  - reference: `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:236`
  - reference: `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:366`
- current implementation / docs:
  - current API is `source-oriented` with `spec.source`, `spec.package`,
    `spec.publish`, `spec.runtimeHints`, `spec.access`
  - `status` exposes `source`, `upload`, `artifact`, `metadata`
  - source variants are `HuggingFace | Upload | OCIArtifact`
  - references:
    - `api/core/v1alpha1/types.go:21`
    - `api/core/v1alpha1/types.go:53`
    - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md:36`
- impact:
  - current public contract is not ADR-compatible;
  - implementation and design docs already encode a different API model.
- decision: `rework-now`

### Conditions and lifecycle model

- area: `status.conditions` and phases
- restored ADR expectation:
  - core conditions are `ArtifactResolved`, `MetadataResolved`, `Validated`,
    `Ready`
  - aggregated phase is intentionally narrow: `Pending | Ready | Failed`
  - reference:
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:479`
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:567`
- current implementation / docs:
  - conditions include `UploadReady`, `ArtifactStaged`, `ArtifactPublished`,
    `AccessConfigured`, `BackendSynchronized`, `CleanupCompleted`
  - phases include `WaitForUpload`, `Publishing`, `Syncing`, `Deleting`
  - references:
    - `api/core/v1alpha1/conditions.go:21`
    - `api/core/v1alpha1/types.go:218`
    - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md:167`
- impact:
  - lifecycle expresses controller orchestration internals instead of ADR
    catalog semantics.
- decision: `rework-now`

### Immutability model

- area: immutable spec fields
- restored ADR expectation:
  - immutable by meaning: `spec.modelType`, whole `spec.artifact`,
    speculative decoding block, and launch policy values
  - reference:
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:619`
- current implementation / docs:
  - immutable fields are `spec.source`, `spec.package`, `spec.publish`,
    `spec.runtimeHints`
  - reference:
    - `api/core/v1alpha1/types.go:21`
    - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md:242`
- impact:
  - immutability protects the wrong public identity surface.
- decision: `rework-now`

### Model / ClusterModel semantic alignment

- area: scope-specific semantics
- restored ADR expectation:
  - `Model` and `ClusterModel` differ only by visibility level; schema is the
    same
  - ADR only requires explicit namespace for cluster-scoped secret refs when
    relevant to artifact pull
  - reference:
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:119`
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:389`
- current implementation / docs:
  - `ClusterModel` now requires explicit `spec.access`
  - `ClusterModel` adds HF-specific namespace validation for `authSecretRef`
  - references:
    - `api/core/v1alpha1/clustermodel.go:32`
    - `api/core/v1alpha1/clustermodel.go:34`
- impact:
  - cluster object semantics are shaped by source-oriented publication design,
    not by artifact-first catalog contract.
- decision: `rework-now`

### Design bundle drift

- area: active phase-2 design docs
- restored ADR expectation:
  - artifact-first catalog for `ai-inference`
  - controller validates artifact access, resolves digest, calculates technical
    profile, validates user constraints
  - reference:
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:141`
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:640`
- current implementation / docs:
  - active design bundle is centered on `source` variants, upload handoff,
    `ModelKit`, `payload-registry` as canonical publish plane, `BackendMirror`,
    `Publisher` and runtime delivery adapters
  - references:
    - `plans/active/design-model-catalog-controller-and-publish-architecture/API_CONTRACT.ru.md:45`
    - `plans/active/design-model-catalog-controller-and-publish-architecture/TARGET_ARCHITECTURE.ru.md:7`
    - `plans/active/design-model-catalog-controller-and-publish-architecture/TARGET_ARCHITECTURE.ru.md:124`
- impact:
  - docs currently reinforce the wrong implementation direction.
- decision: `rework-now`

### Controller runtime shell

- area: module rollout and operational baseline
- restored ADR expectation:
  - module contains a controller for `Model` / `ClusterModel`
  - controller checks artifact access, calculates profile, validates constraints
  - reference:
    - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:631`
- current implementation / docs:
  - live controller runtime exists only as local code; no controller image
    wiring, no module templates, no CRD install path
  - metrics and health are disabled; leader election absent
  - references:
    - `images/controller/internal/app/app.go:89`
    - `images/controller/cmd/ai-models-controller/run.go:55`
    - `.werf/stages/bundle.yaml:1`
    - `templates/` currently has no `templates/controller/`
- impact:
  - module cannot yet ship even a minimal phase-2 controller baseline.
- decision: `rework-now`

### Cleanup-only reconciler

- area: live reconcile path already present in code
- restored ADR expectation:
  - deletion semantics matter, but ADR does not prescribe a source-oriented
    publication API
- current implementation / docs:
  - `internal/modelcleanup` already provides a bounded live controller path for
    finalizer-driven cleanup
  - references:
    - `images/controller/internal/modelcleanup/reconciler.go:48`
    - `images/controller/README.md:32`
- impact:
  - cleanup-only reconciliation can be rolled out without locking in the wrong
    public API shape.
- decision: `keep-out-of-contract`

### Backend HF import and MLflow cleanup entrypoints

- area: internal backend capabilities
- restored ADR expectation:
  - backend internals stay behind public contract
- current implementation / docs:
  - backend image already contains real HF import and MLflow cleanup entrypoints
  - references:
    - `images/backend/scripts/ai-models-backend-hf-import.py:17`
    - `images/backend/scripts/ai-models-backend-model-cleanup.py:17`
- impact:
  - these are useful execution primitives, but they must remain internal and not
    redefine the public CRD contract.
- decision: `keep-out-of-contract`

### User target beyond restored ADR

- area: HF-first publication, backend choice, KubeRay materializer
- restored ADR expectation:
  - first iteration is OCI-only public artifact reference
  - no explicit HF/upload source model in public CRD
- current user target:
  - create CR and choose different sources, HF first;
  - publish into either MLflow/S3 or OCI/payload-registry;
  - later use model through KubeRay materializer/agent.
- impact:
  - this is a legitimate future target, but it is broader than restored ADR and
    cannot be silently smuggled into the public API again.
- decision: `defer`

## Chosen bounded implementation slice

- slice: ADR-neutral controller runtime rollout
- why:
  - immediately useful to the user;
  - does not require freezing the wrong source-oriented public contract;
  - reuses the already existing cleanup-only live reconciler;
  - unblocks later API rework and publication execution slices.

## Subagent findings

Заполняется после read-only review.

### integration_architect

- safest first live runtime slice is cleanup-only controller rollout
- controller image/module shell/CRD install path are still missing
- metrics/health/leader election are not wired
- references:
  - `images/controller/internal/app/app.go:79`
  - `images/controller/internal/modelcleanup/reconciler.go:48`
  - `.werf/stages/bundle.yaml:16`
  - `images/hooks/cmd/ai-models-hooks/register.go:19`

### api_designer

- top three contract drifts that block honest ADR alignment:
  - `spec` shape is different (`artifact/modelType/policies` vs
    `source/package/publish/runtimeHints`)
  - `status` and lifecycle differ (`status.resolved` + narrow conditions/phases
    vs upload/publication-oriented state machine)
  - resource identity and scope semantics differ (API group/examples and
    `ClusterModel` access assumptions)
- references:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:237`
  - `api/core/v1alpha1/types.go:26`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md:278`
  - `api/core/v1alpha1/conditions.go:21`
  - `api/core/register.go:19`
  - `api/core/v1alpha1/clustermodel.go:32`

### backend_integrator

- decision from backend/publication side also points to cleanup-only runtime
  rollout first
- reasons:
  - executable ownership already exists in `app` + `modelcleanup`
  - `managedbackend` and `publication` are still library-only planning layers
  - rollout first avoids deepening source-oriented drift before controller is a
    real module runtime
- references:
  - `images/controller/internal/modelcleanup/reconciler.go`
  - `images/controller/internal/app/app.go`
  - `images/controller/internal/publication/snapshot.go`
  - `images/controller/internal/managedbackend/contract.go`
