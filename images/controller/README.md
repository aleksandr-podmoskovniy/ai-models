# images/controller

`images/controller/` is the canonical root for executable controller code of the
`ai-models` module.

Detailed folder/file inventory and rationale live in
[STRUCTURE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/STRUCTURE.ru.md).
Controller-level decision/test evidence lives in
[TEST_EVIDENCE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/TEST_EVIDENCE.ru.md).

Rules:
- phase 2 controller code lives in the module rooted here;
- `go.mod` stays on this directory root;
- `cmd/` stays a thin executable shell;
- domain logic stays under `internal/*` until there is a real external consumer;
- shared controller ports live under `internal/ports/*` and must not stay
  buried inside adapter-local packages;
- concrete reconcilers live under `internal/controllers/*`;
- concrete Pod/Service/Secret builders and CRUD adapters live under
  `internal/adapters/k8s/*`;
- shared helper code may live under `internal/support/*` only when it removes
  real duplication across controller/adapters and does not become a second
  business-logic layer;
- public DKP API types still live in top-level `api/`.

Current phase-2 slice implemented here:
- `kitops.lock` and `tools/install-kitops.sh` for the pinned `KitOps` binary
  that now belongs to the dedicated phase-2 runtime image instead of the
  backend runtime;
- `cmd/ai-models-controller/*` for thin manager-only shell;
- `cmd/ai-models-artifact-runtime/*` for thin one-shot phase-2 runtime shell:
  `publish-worker`, `upload-session`, and `artifact-cleanup`;
- `internal/artifactbackend` for backend-facing publication requests and
  results that stay independent from any concrete artifact backend product;
- `internal/ports/modelpack` for replaceable `ModelPack` publication/removal
  contract;
- `internal/adapters/modelpack/kitops` for the current `KitOps`-based
  implementation of that contract;
- `internal/adapters/sourcefetch` for safe `HuggingFace` and `HTTP` source
  acquisition and archive hardening, with one canonical remote ingest entrypoint
  over shared HTTP transport and archive preparation instead of split
  orchestration in the worker;
- `internal/adapters/modelformat` for source-agnostic input-format validation
  rules applied before packaging;
- `internal/adapters/modelprofile/safetensors` and
  `internal/adapters/modelprofile/gguf` for ai-inference-oriented metadata
  extraction from normalized model directories, with current live logic based
  on real weight sizes, task-to-endpoint mapping, quantization/precision
  inference, and minimum-launch estimation;
- `internal/adapters/k8s/sourceworker` for controller-owned worker Pods that turn
  accepted remote URLs into backend-owned stored artifacts, while reserving
  `Upload` for a dedicated session workflow;
  the package also implements the shared source-worker runtime port directly,
  consumes the shared `publishop.OperationContext` without adapter-local
  request mirrors, and does not keep a second runtime-proxy layer or
  constructor path over the same concrete adapter; it now drives the concrete
  Pod through one direct `CreateOrGet` cycle instead of a separate replay read
  path before the same create/reuse flow, and projected auth supplements now go
  through one direct `CreateOrUpdate` path instead of adapter-local CRUD;
- `internal/adapters/k8s/uploadsession` for controller-owned upload session
  supplements:
  worker `Pod`, `Service`, short-lived auth `Secret`, and user-facing upload
  command projection for `spec.source.upload`; the package also implements
  the shared upload-session runtime port directly, consumes the shared
  `publishop.OperationContext` without local request wrappers or a separate
  request-mapping file, and does not
  keep a second runtime-proxy layer or constructor path over the same concrete
  adapter; replay now goes through the same direct ensure/create-or-get path
  for `Secret`, `Service`, and `Pod` instead of a separate pre-read branch;
- `internal/adapters/k8s/ociregistry` for shared OCI registry auth/CA env and
  volume rendering used by worker/session/cleanup paths;
- `internal/adapters/k8s/ownedresource` for the single canonical
  owned-resource lifecycle shell reused by controlled worker/session
  supplements: create/reuse plus ignore-not-found delete;
- `internal/adapters/k8s/workloadpod` for the single canonical workspace
  `EmptyDir` + `/tmp` mount and registry-CA volume/mount shell reused by
  worker/upload pod adapters;
- `internal/dataplane/publishworker` for the controller-owned publication
  runtime that fetches sources, computes resolved metadata, publishes a
  `ModelPack`, and writes the final result into the worker Pod termination
  message;
- `internal/dataplane/uploadsession` for the controller-owned HTTP upload
  session runtime; it hands the final publication result back through the
  upload Pod termination message after a successful upload;
- `internal/dataplane/artifactcleanup` for the controller-owned published
  artifact removal runtime;
- `internal/publishedsnapshot` for immutable published-artifact snapshots used
  as controller handoff between publish, cleanup, and delete steps;
- `internal/ports/publishop` for shared publication operation runtime
  contracts, operation contract primitives, and worker/session handles reused
  across adapters; both concrete runtime adapters now use one `GetOrCreate`
  contract instead of diverging by extra read-only methods;
- `internal/domain/publishstate` for publication lifecycle state, condition and
  observation decisions;
- `internal/application/publishplan` for source-worker and upload-session
  planning use cases;
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
  controller path for `Model` / `ClusterModel`; it now owns cleanup Job
  materialization directly because there is no second cleanup adapter and the
  old `adapters/k8s/cleanupjob` package was only an unnecessary extra boundary;
- `internal/controllers/catalogstatus` for thin `Model` / `ClusterModel`
  publication lifecycle ownership: planning worker/session runtime,
  observing working Pods, projecting public status, and persisting cleanup
  handles without an intermediate persisted bus;
- `internal/bootstrap` for manager/bootstrap wiring.

Naming rule:
- do not keep four different packages named `publication` across
  `application/`, `domain/`, `ports/` and `internal/`; role-based names such
  as `publishplan`, `publishstate`, `publishop`, and `publishedsnapshot` are
  required so the tree stays explicit and closer to virtualization-style
  ownership.

Still intentionally out of scope:
- publication paths beyond the current live input matrix:
  - `HuggingFace URL -> Safetensors`
  - `HTTP URL -> Safetensors archive or GGUF file/archive`
  - `Upload -> Safetensors archive or GGUF file/archive`
  into internal `ModelPack/OCI` through the current Go dataplane and
  implementation adapter;
- richer input formats beyond the current fail-closed `Safetensors` and `GGUF`
  rules shared across `HuggingFace`, `HTTP`, and `Upload` sources;
- richer source auth flows beyond the current minimal projection contract:
  `HuggingFace` supports a projected token secret and `HTTP` supports projected
  `authorization` or `username`+`password` material, but broader source
  integrations and richer auth/session handoff stay out of scope;
- live runtime integration with `ai-inference`, concrete init-container
  pod mutation/runtime injection for materializers, and any speculative
  materialization adapter code before there is a real consumer path; runtime
  delivery stays a future bounded workstream and must remain adapter-agnostic
  when it lands;
- richer publication hardening beyond the current implementation adapter:
  implementation switching and stronger trust/promotion semantics.
