# Controller Discipline Reference

## Purpose

This reference holds the durable controller architecture rules for DKP module
controllers. It exists so the project memory does not depend on a specific
active planning bundle staying under `plans/active`.

Read this reference when the task touches `images/controller/internal/*`,
especially lifecycle orchestration, status projection, external workload
adapters, cleanup, and finalizer flow.

Module-specific artifact, upload, storage, or API semantics do not belong in
this reference. Keep those in explicit overlay skills or repo docs.

## Target split

Keep controller code split into these layers:

- `domain`
  - lifecycle states
  - transitions
  - invariants
  - status/condition mapping rules
- `application`
  - use cases such as:
    - `AcceptInput`
    - `StartReconcileStep`
    - `ObserveWorker`
    - `ProjectStatus`
    - `FinalizeDelete`
- `ports`
  - worker runtime
  - published-artifact writer or runtime publisher
  - cleanup executor
  - clock
  - token/session issuer when the module owns that lifecycle
- `adapters`
  - Kubernetes reconcilers
  - concrete Pod/Service/Secret/Job builders and service adapters
  - concrete packagers, publishers, storage, or API clients

## Thin reconciler rule

Reconcilers are Kubernetes adapters only.

Allowed in a reconciler:

- fetch object(s)
- add or remove finalizer
- call one application use case
- persist returned mutations
- requeue using explicit use-case result

Do not keep the following in reconciler files:

- lifecycle branching trees
- state machine logic
- inline `Pod` / `Service` / `Secret` / `Job` construction
- status reason decision trees
- serialization format details for worker result handoff

## First corrective cuts

When a controller is already too fat, corrective cuts should start with the
highest-decision-surface areas first:

1. status/condition projection
2. external workload orchestration
3. credential/session/token issuance
4. delete/finalizer policy
5. repeated K8s object builders

Do not add new feature logic to those areas before checking whether the current
slice should instead reduce their responsibility first.

## Current concrete package map

Keep the remaining controller runtime tree readable:

- `internal/bootstrap/*`
  - only controller composition root and manager wiring
- `internal/controllers/*`
  - only concrete reconcilers and their thin persistence/observation shells
- `internal/adapters/k8s/*`
  - only concrete Pod/Service/Secret/Job builders and CRUD/service adapters
- `internal/support/*`
  - only real shared helpers such as model-object helpers, cleanup-handle
    persistence helpers, and canonical owner-based resource naming / owner
    labels policy

Do not reintroduce a flat top-level patchwork where controller reconcilers,
Kubernetes resource builders and shared helpers all sit side by side.

Do not keep an `internal/app` package next to `internal/application`. That
pair is semantically ambiguous in a hexagonal tree. Composition root packages
must be named by responsibility, for example `internal/bootstrap`.

Do not keep four generic folders named around one vague word such as
`lifecycle` across `internal/`, `application/`, `domain/`, and `ports/`
when each one actually owns a narrower responsibility. Use role-based names
such as `statusprojection`, `deliveryplan`, `lifecyclepolicy`, and `workloadrun`.

Do not recreate package-local `names.go` wrappers for Pod/Service/Secret/Job
names unless they add a real new boundary. Canonical resource naming belongs in
`internal/support/resourcenames`.

Do not mirror shared runtime input through adapter-local `Request`
or `OwnerRef` wrapper types unless the adapter truly needs a different contract.
Concrete adapters should consume the shared application/port contract directly
and keep only adapter-specific planning helpers.

Do not keep a separate `runtime.go` proxy over the same concrete adapter object
when that file only forwards to `Service`/CRUD methods and wraps the same
handle/delete closure. In that case the adapter should implement the shared port
itself and keep internal CRUD as unexported helper methods on the same object.

Do not repeat the same controlled K8s object create/reuse shell
(`SetControllerReference -> Create -> AlreadyExists -> Get`) across multiple
adapter packages. If the flow is the same and only the concrete object shape is
different, keep one shared helper under `internal/adapters/k8s/*`.

Do not repeat the same workload `Pod` shell (`EmptyDir` workspace, `/tmp`
mount, registry CA volumes/mounts) across multiple adapters. If the structure
is the same and only command/env/extra mounts differ, keep one shared helper
under `internal/adapters/k8s/*`.

Do not invent a shared persisted-bus layer unless there is a real second
adapter behind that seam. If one controller can observe working Pods directly
and read their termination result, keep that path direct instead of inserting
another store-shaped hop.

When the package map changes materially, update the repo-local inventory in
`images/controller/STRUCTURE.ru.md`. New files should be defendable by:

- purpose;
- why the file belongs in that package;
- why the responsibility does not belong to a neighboring layer.

## Quality gates

The controller must satisfy repo-level gates wired into `make verify`.

Relevant files:

- `Makefile`
- `tools/install-gocyclo.sh`
- `tools/check-controller-complexity.sh`
- `tools/check-controller-loc.sh`
- `tools/check-controller-test-loc.sh`
- `tools/check-thin-reconcilers.sh`
- `tools/test-controller-coverage.sh`
- `tools/check-controller-test-evidence.sh`

Current thresholds:

- `gocyclo <= 15`
- non-test controller file length `<= 350` lines unless explicitly allowlisted
- controller test file length `<= 350` lines unless explicitly allowlisted
- thin reconciler rule enforced unless explicitly allowlisted

Temporary debt must be explicit through allowlists under `tools/`, not hidden in
the code or in chat context.

Controller verification must stay explicit in naming and ownership:

- controller-specific deadcode checks must have a first-class target
  (`deadcode-controller`)
- if hooks and controller checks share one shell script, controller output
  must run first and be labeled as required
- review should treat misleading verification output as architecture/tooling
  debt, not as a harmless wording issue

## Test evidence

Lifecycle code is not validated by happy-path adapter tests alone.

Required evidence shape for controller slices:

- state transition matrix
- negative branches
- idempotency
- reconcile replay / retry behavior
- deletion and finalizer races
- malformed worker result paths

Shared fixture discipline:

- shared controller test scheme/object/fake-client fixtures belong under
  `internal/support/testkit`
- package-local `test_helpers_test.go` may keep only adapter-local builders,
  canned runtime payloads, and assertions
- adapter-heavy controller tests should be split by decision family rather than
  accreting into one large `reconciler_test.go`
- helper files must not become a second hidden business-logic layer
- shared ports should be implemented in the concrete adapter package that owns
  the underlying CRUD/build behavior, not in a controller-side shim
- keep controller test evidence centralized in
  `images/controller/TEST_EVIDENCE.ru.md`; do not reintroduce scattered
  package-local `BRANCH_MATRIX.ru.md` files that create uneven rules across the
  tree

If the change mostly moves logic between packages, tests still need to prove
that the decision surface is preserved.

## Published artifact direction

If the module owns publication or delivery, controller work must stay aligned
with these rules:

- public contract stays close to user intent
- controller-owned published artifact or published state stays canonical
- concrete packaging/materialization brands stay behind adapters
- runtime consumption should depend on the published artifact/state, not on
  direct backend internals

## Large-ingest discipline

For large-object ingest or staging, keep these rules stable:

- public `status` remains the only platform truth for lifecycle state;
- raw storage plus optional audit/provenance records stay internal and must not
  become a second lifecycle truth;
- when bytes can bypass the control plane directly to object storage or another
  durable sink, prefer one shared control-plane gateway over one heavy runtime
  Pod/Service/Ingress trio per ingest;
- bulk bytes should bypass the controller or gateway after bounded preflight;
- ingest concurrency and publish/materialize concurrency must be bounded
  separately;
- preflight checks before full ingest must stay small and explicit:
  authz, owner binding, quota, declared size, allowed format, lightweight
  source probe;
- deep malware/content scanning is a later async stage; do not describe
  preflight as a security guarantee it cannot provide;
- the copy budget for large ingest must be explicit in the task bundle and
  review, and anything above that budget must be treated as debt.

Use task-local bundles for slice-specific implementation detail, but keep these
rules stable here.
