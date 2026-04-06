# PLAN

## Current phase

Этап 2. Corrective architecture and quality discipline before the next phase-2
feature slices.

## Orchestration

- mode: `full`
- read-only subagents before writing the plan:
  - controller boundary audit
  - quality-gate / verify-pipeline audit

## Audit conclusions

### Controller boundary audit

- At bundle start the most problematic files were:
  - `images/controller/internal/modelpublish/reconciler.go` — 536 LOC
  - `images/controller/internal/publicationoperation/reconciler.go` — 409 LOC
  - `images/controller/internal/uploadsession/session.go` — 606 LOC
- Additional problematic boundaries:
  - `images/controller/internal/publicationoperation/contract.go` mixes domain
    contract with ConfigMap serialization/storage format
  - `images/controller/internal/sourcepublishpod/pod.go` still mixes request
    validation, naming, env/volume/arg assembly, and Pod rendering
- Current package split is still mostly adapter-oriented, not use-case oriented.
- Remaining concrete adapter package map is still patchwork:
  - top-level controller reconcilers (`modelpublish`, `modelcleanup`,
    `publicationoperation`) are mixed with concrete K8s worker/session/job
    adapters (`sourcepublishpod`, `uploadsession`, `cleanupjob`) at the same
    directory depth;
  - repeated helper logic still exists in:
    - `modelpublish/*` vs `modelcleanup/*` (`Model` / `ClusterModel`
      object-kind/status helpers and controller shell patterns)
    - `sourcepublishpod/*` vs `uploadsession/*` vs `cleanupjob/*`
      (owner UID naming, OCI registry env/volume rendering, label truncation)
- Reconcile layer currently mixes:
  - state decisions;
  - K8s object IO;
  - worker/session orchestration;
  - status projection;
  - cleanup handle persistence.
- Good seeds already present:
  - `images/controller/internal/publication/snapshot.go`
  - `images/controller/internal/artifactbackend/contract.go`
  - `images/controller/internal/cleanuphandle/handle.go`
- First corrective cut should be around:
  - `modelpublish`
  - `publicationoperation`
  - then `uploadsession`

### Quality-gate audit

- Current `Makefile` has no complexity/LOC/coverage-specific controller gates.
- `gpu-control-plane` uses two patterns worth porting:
  - explicit module-level `-coverprofile` outputs under `artifacts/coverage`;
  - root `deadcode` tool installation and a dedicated verify target wired into
    `make verify`.
- Best hook points are:
  - `ensure-tools`
  - new verify targets under `verify`
  - helper scripts under `tools/`
- CI hook point also exists in `.github/workflows/build.yaml` and should call
  the same `make verify` / dedicated controller verify target rather than
  duplicate logic.
- Existing repo shell is good enough to add:
  - `gocyclo`
  - custom controller size check
  - controller test coverage gate
  - controller test-evidence inventory check
  - deadcode install/check shell
  - explicit coverage-dir/module test targets

### Current reduction baseline

- after the latest cleanup cuts, `images/controller` now has:
  - `5790` non-test Go LOC;
  - `6060` test Go LOC.
- Current bounded reduction slice should:
  - remove ambiguous package names such as `internal/app` beside
    `internal/application`;
  - remove generic repeated package names such as `publication` across
    `internal/`, `application/`, `domain/`, and `ports/` when those packages
    actually own different responsibilities;
  - keep objective deadcode tooling active;
  - keep controller deadcode as an explicit, first-class verify signal rather
    than implicit output behind hooks tooling;
  - replace scattered package-local `BRANCH_MATRIX.ru.md` files with one
    controller-level evidence source of truth;
  - rewrite remaining concrete package layout to `controllers/`,
    `adapters/k8s/`, `support/`;
  - delete shared helper duplication during the move instead of carrying
    old-vs-new trees in parallel;
  - keep the long-running workstream target of reducing controller production
    code at least by half from the current ~6.0k LOC baseline.
- `gpu-control-plane` additionally uses:
  - `COVERAGE_DIR` + package-scoped `-coverprofile` artifacts under
    `artifacts/coverage`;
  - explicit `ensure-deadcode` + standalone `deadcode` target wired into
    `verify`.
- For `ai-models`, the right transfer is:
  - keep existing threshold-based controller coverage gate;
  - add bounded controller coverage artifact collection, not repo-wide noise;
  - add `deadcode` to `ensure-tools`/`verify`;
  - use `deadcode` findings only together with import-graph and targeted tests
    before deleting lifecycle code.

## Slice 1. Hard Architecture Target

Цель:

- зафиксировать target package layout and ownership.

Артефакты:

- `TARGET_LAYOUT.ru.md`
- `USE_CASES.ru.md`

Проверки:

- manual consistency check against current controller packages

## Slice 2. Quality Gates And Test Discipline

Цель:

- зафиксировать exact quality gates and test strategy for future slices.

Артефакты:

- `QUALITY_GATES.ru.md`
- `TEST_STRATEGY.ru.md`

Проверки:

- manual consistency check against current `Makefile` and `tools/`

## Slice 3. Refactor Order And First Cut Plan

Цель:

- превратить target architecture в executable corrective sequence.

Артефакты:

- `IMPLEMENTATION_ORDER.ru.md`

Проверки:

- manual consistency check against current large files and package boundaries

## Slice 4. Port `gpu-control-plane` Coverage/Deadcode Discipline

Цель:

- встроить reproducible coverage artifacts и deadcode detection в current
  controller verify shell.

Артефакты:

- `Makefile`
- `tools/*`
- current bundle notes/review

Проверки:

- targeted coverage artifact generation
- `deadcode` against current controller package patterns

## Slice 5. Deadcode-Driven Reduction Cut

Цель:

- удалить реально мёртвые функции, wrappers и compatibility seams, подтверждённые
  deadcode/import graph/tests.

Артефакты:

- `images/controller/internal/*`
- active bundle notes/review

Проверки:

- package-local `go test`
- controller quality gates
- `deadcode`

## Rollback point

Each slice is bounded and can stop safely after:

- package-local tests;
- controller quality gates;
- `git diff --check`.

No new feature work may be stacked on a slice before those checks pass.

## Final validation

- package-local `go test`
- controller quality gates
- controller deadcode check
- `go test ./...` in `images/controller`
- `make verify`
- `git diff --check`
