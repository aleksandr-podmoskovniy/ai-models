# images/controller

`images/controller/` is the canonical root for executable controller code of the
`ai-models` module.

Detailed folder/file inventory and rationale live in
[STRUCTURE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/STRUCTURE.ru.md).

Rules:
- phase 2 controller code lives in the module rooted here;
- `go.mod` stays on this directory root;
- `cmd/` stays a thin executable shell;
- domain logic stays under `internal/*` until there is a real external consumer;
- shared controller ports live under `internal/ports/*` and must not stay
  buried inside adapter-local packages;
- concrete reconcilers live under `internal/controllers/*`;
- concrete Pod/Service/Secret/Job builders and CRUD adapters live under
  `internal/adapters/k8s/*`;
- shared helper code may live under `internal/support/*` only when it removes
  real duplication across controller/adapters and does not become a second
  business-logic layer;
- public DKP API types still live in top-level `api/`.

Current phase-2 slice implemented here:
- `internal/artifactbackend` for backend-facing publication requests and
  results that stay independent from any concrete artifact backend product;
- `internal/adapters/k8s/sourceworker` for controller-owned worker Pods that turn
  accepted `HuggingFace` and archive-based `HTTP` sources into backend-owned
  stored artifacts, while reserving `Upload` for a dedicated session workflow;
  the package also implements the shared source-worker runtime port directly,
  consumes the shared `publication.OperationContext` without adapter-local
  request mirrors, and does not keep a second runtime-proxy layer over the same
  concrete adapter;
- `internal/adapters/k8s/uploadsession` for controller-owned upload session
  supplements:
  worker `Pod`, `Service`, short-lived auth `Secret`, and user-facing upload
  command projection for `spec.source.type=Upload`; the package also implements
  the shared upload-session runtime port directly, consumes the shared
  `publication.OperationContext` without local request wrappers, and does not
  keep a second runtime-proxy layer over the same concrete adapter;
- `internal/adapters/k8s/cleanupjob` for controller-owned cleanup Job
  materialization;
- `internal/adapters/k8s/ociregistry` for shared OCI registry auth/CA env and
  volume rendering used by worker/session/cleanup adapters;
- `internal/adapters/k8s/ownedresource` for the single canonical
  `SetControllerReference -> Create -> AlreadyExists -> Get` shell reused by
  controlled worker/session supplements;
- `internal/adapters/k8s/workloadpod` for the single canonical workspace
  `EmptyDir` + `/tmp` mount and registry-CA volume/mount shell reused by
  worker/upload pod adapters;
- `internal/publication` for immutable publication snapshots used as controller
  handoff between publish, cleanup, and runtime delivery steps;
- `internal/ports/publication` for shared publication operation runtime
  contracts, operation contract primitives, and worker/session handles reused
  across adapters; controller-local persisted `ConfigMap` protocol stays in
  `controllers/publicationops` until there is a real second store adapter;
- `internal/support/cleanuphandle` for controller-owned backend-specific delete
  state
  that must not leak into public status;
- `internal/support/modelobject` for shared `Model` / `ClusterModel`
  publication-request, kind and status helpers;
- `internal/support/resourcenames` for the single canonical owner-based
  resource naming policy plus owner-label rendering/extraction and label
  normalization across K8s adapters;
- `internal/support/testkit` for shared controller test scheme/object/fake-client
  fixtures; package-local `test_helpers_test.go` should only keep adapter-local
  option/resource builders, not duplicate the same scheme and model fixtures in
  every controller package;
- `internal/controllers/catalogcleanup` for minimal delete-only finalizer
  controller path for `Model` / `ClusterModel`;
- `internal/controllers/publicationops` for controller-owned durable execution
  boundary between source publication requests and backend-backed worker Pods;
- `internal/controllers/catalogstatus` for thin `Model` / `ClusterModel`
  publication lifecycle ownership: operation request creation, public status
  projection, and cleanup handle persistence;
- `internal/app` for manager/bootstrap wiring.

Still intentionally out of scope:
- live backend publication paths beyond
  `HuggingFace|HTTP archive|Upload(HuggingFaceDirectory) -> ModelPack/OCI`
  through the current implementation adapter;
- richer source auth flows beyond the current minimal projection contract:
  `HuggingFace` supports a projected token secret and `HTTP` supports projected
  `authorization` or `username`+`password` material, but broader source
  integrations and richer auth/session handoff stay out of scope;
- live runtime integration with `ai-inference`, concrete init-container
  pod mutation/runtime injection for materializers, and any speculative
  materialization adapter code before there is a real consumer path; runtime
  delivery stays a future bounded workstream and must remain adapter-agnostic
  when it lands;
- `Upload` support for `ModelKit`;
- richer publication hardening beyond the current implementation adapter
  `init/pack/push/inspect` path: direct `ModelKit` ingest, promotion,
  implementation switching, and stronger validation.
