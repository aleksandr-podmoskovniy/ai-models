# Review

Исторический cumulative review журнала corrective workstream.

Ниже встречаются старые package names и промежуточные seams из ранних slices.
Этот файл не должен использоваться как current architecture map; current source
of truth теперь:

- `PLAN.ru.md`
- `TARGET_LAYOUT.ru.md`
- `images/controller/STRUCTURE.ru.md`

## Findings

- Critical blockers не найдены.

## Coverage

- Зафиксирован target split:
  - `domain`
  - `application`
  - `ports`
  - `adapters`
- Зафиксированы first cuts по самым плохим текущим файлам.
- Зафиксированы hard quality gates и verify hook points.
- Зафиксирована test strategy по branch/state matrix, а не по file-oriented
  happy path.
- Slice 1 реализован:
  - `Makefile` теперь включает controller quality gates в `verify` / `verify-ci`
  - добавлены scripts и explicit allowlists в `tools/`
  - `make verify` проходит с новыми gates на текущем debt baseline
- Slice 2 реализован bounded cut:
  - `modelpublish` lifecycle/status projection вынесен в
    `images/controller/internal/application/publication`
  - для нового application package добавлен `BRANCH_MATRIX.ru.md`
  - `images/controller/internal/modelpublish/reconciler.go` больше не держит
    inline status builders и condition assembly
  - `modelpublish` снят с LOC / complexity / thin-reconciler allowlists после
    прохождения verify gates
  - исправлен parsing bug в `tools/test-controller-coverage.sh`, который
    проявился только после появления первого реального `application` package
- Slice 3 реализован bounded cut:
  - `publicationoperation` decision logic вынесена в
    `images/controller/internal/application/publication`
    через `start_publication.go` и позже перенесённые в domain runtime
    decisions
  - adapter shell разделён на `options.go`, `source.go`, `upload.go`,
    `status.go`, `persistence.go`
  - `images/controller/internal/publicationoperation/reconciler.go` теперь thin
    shell и снят с LOC / complexity / thin-reconciler allowlists
  - adapter tests добавлены на unmanaged ignore, malformed request fail-closed,
    terminal no-op и malformed worker result
- Slice 4 реализован bounded cut:
  - `images/controller/internal/uploadsession/session.go` удалён полностью
  - upload-session policy вынесена в
    `images/controller/internal/application/publication/issue_upload_session.go`
  - adapter shell разделён на `request.go`, `service.go`, `resources.go`,
    `pod.go`, `status.go`, `names.go`, `options.go`
  - добавлены replay/fail-closed tests для existing session, partial
    `AlreadyExists`, malformed secret state, builder args/env/volumes
  - `uploadsession` снят с LOC / complexity allowlists после прохождения verify
    gates
- Slice 5 реализован bounded cut:
  - delete/finalizer decision tables вынесены в
    `images/controller/internal/application/deletion`
  - `modelcleanup` adapter разделён на `options.go`, `observation.go`,
    `persistence.go`, `reconciler.go`
  - `modelcleanup/reconciler.go` больше не держит inline finalizer policy,
    cleanup-handle branching и job-state transition logic
  - добавлены branch-matrix и use-case tests для `application/deletion`, плюс
    adapter tests на stale-finalizer delete path, invalid cleanup handle,
    running cleanup job и failed cleanup job
  - cleanup job create path теперь fail-closed на invalid job build и не падает
    на `AlreadyExists` race
- Slice 6 реализован bounded cut:
  - `publicationoperation/contract.go` удалён полностью
  - contract surface разделена на `types.go`, `constants.go`,
    `configmap_codec.go`, `configmap_mutation.go`
  - persisted ConfigMap protocol остался прежним, но validation, codec и
    mutation helpers больше не смешаны в одном 369 LOC файле
  - добавлены fail-closed tests на malformed `request.json` / `result.json`,
    default `Pending`, explicit managed marker semantics и cleanup behavior of
    `SetRunning` / `SetFailed` / `SetSucceeded`
  - `publicationoperation` снят с LOC allowlist после прохождения verify gates
- Slice 7 реализован bounded cut:
  - `images/controller/internal/sourcepublishpod/pod.go` удалён полностью
  - package разделён на `types.go`, `validation.go`, `names.go`, `build.go`
  - source-specific worker policy вынесена в
    `images/controller/internal/application/publication/plan_source_worker.go`
  - adapter validation больше не смешана с Pod rendering и naming helpers
  - добавлены direct negative tests на invalid owner/identity, unsupported
    source kinds, unsupported auth branches и `Upload` session guard
  - branch-matrix покрытия расширены тестами на HF/HTTP planning и upload/auth
    rejection для source-worker path
  - `sourcepublishpod` снят с complexity allowlist после прохождения verify
    gates
  - package-local `go test`, controller quality gates и `go test ./...` в
    `images/controller` проходят
- Slice 8 реализован bounded cut:
  - `publicationoperation` persisted protocol теперь fail-closed на unknown
    source type и semantically invalid upload payload
  - `publicationoperation` больше не silently no-op'ит corrupt terminal state:
    `Succeeded` без валидного persisted `result` и unsupported persisted phase
    переводятся в controlled failure
  - persisted upload payload теперь требует `command`, `repository` и
    `expiresAt`; `SetFailed` очищает stale `result.json`, а не только
    `upload.json`
  - replay tests добавлены на `AwaitingResult -> Succeeded`, malformed
    persisted upload payload on running operation, identical upload-status
    no-op replay и expired upload terminal replay
  - package-local `go test`, controller quality gates, `go test ./...` в
    `images/controller` и repo-level `make verify` проходят
- Slice 9 реализован bounded cut:
  - `publicationoperation` получил explicit adapter-local ports:
    `operationStore`, `sourceWorkerRuntime`, `uploadSessionRuntime`
  - concrete `ConfigMap` persistence вынесена в `store.go`, а source/upload
    worker/session service wiring вынесен в `runtime.go`
  - `source.go` и `upload.go` больше не конструируют concrete adapter services
    и не обновляют operation `ConfigMap` напрямую
  - source/upload reconcile paths теперь работают через narrow handles и
    store/runtime interfaces, а concrete adapter tests отдельно фиксируют
    store cleanup semantics и worker/session create-delete behavior
  - package-local `go test`, controller quality gates, `go test ./...` в
    `images/controller` и repo-level `make verify` проходят после cut
- Slice 10 реализован bounded cut:
  - появился первый реальный `internal/domain/publication` package со своим
    `BRANCH_MATRIX.ru.md`, в который вынесены terminal phase semantics,
    worker/session runtime decisions и status/condition projection для
    published model lifecycle
  - status projection moved behind the domain seam и больше не живёт в
    отдельном application facade файле
  - runtime decision tables moved behind the domain seam и больше не живут в
    отдельном application facade файле
  - `AcceptedStatus` и `IsTerminalOperationPhase` теперь опираются на новый
    domain seam, а не на package-private projection helpers
  - status projection evidence перенесена из `application/publication` в
    domain-level tests, что лучше совпадает с target split `domain ->
    application -> adapters`
  - package-local `go test`, controller quality gates, `go test ./...` в
    `images/controller` и repo-level `make verify` проходят после cut
- Slice 11 реализован bounded cut:
  - удалён repo junk `.VSCodeCounter`, который не относится к module runtime и
    только загрязнял worktree
  - stale test filenames после refactor приведены к текущим bounded seams:
    `configmap_protocol_test.go`, `build_test.go`,
    `service_roundtrip_test.go`
  - дублирующий `application/publication/observe_publication_test.go` сначала
    был заменён thin facade wiring test, а затем сам facade-only test тоже был
    признан низкосигнальным и удалён; behavioral evidence остаётся у
    `domain/publication` и adapter-level tests
  - branch-matrix evidence синхронизирована с новым cleanup shape
  - controller quality gates, targeted package tests, repo-level `make verify`
    и `git diff --check` проходят после cleanup
- Slice 12 реализован bounded cut:
  - shared publication operation contracts вынесены из adapter-local
    `publicationoperation` в `images/controller/internal/ports/publication`
  - `OperationStore`, `SourceWorkerRuntime`, `UploadSessionRuntime` и
    worker/session handles теперь живут в shared ports package, а concrete
    ConfigMap/Pod/Session implementations остались adapter-local
  - `publicationoperation.Request` больше не протекает напрямую в shared
    runtime ports: он конвертируется на boundary через explicit shared port
    request mapper
  - добавлены package-local tests для new shared handles и адаптированы
    runtime tests на shared ports seam
  - package-local `go test`, controller quality gates, `go test ./...` в
    `images/controller` и repo-level `make verify` проходят после cut
- Slice 13 реализован bounded cut:
  - shared `internal/ports/publication` теперь владеет не только runtime/store
    interfaces, но и operation contract primitives: `Phase`, `Owner`,
    `Request`, `Result`, `Status`
  - `ConfigMapNameFor` позже был признан naming-policy helper, а не port
    contract primitive, и вынесен в `internal/support/resourcenames`
  - `publicationoperation/types.go` сокращён до compatibility shim над shared
    contract, так что adapter package больше не держит вторую полную копию
    operation contract
  - stale duplicate `internal/domain/publicationoperation/*` удалён полностью;
    он больше не создаёт shadow contract и не ломает controller coverage gate
  - validation tests для operation contract перенесены рядом с shared seam в
    `internal/ports/publication/operation_contract_test.go`
  - targeted package tests, controller quality gates, `go test ./...` в
    `images/controller` и repo-level `make verify` проходят после cut
- Slice 14 реализован bounded cleanup cut:
  - удалён speculative `internal/ports/materialization`, который не был
    подключён ни к одному runtime adapter и только увеличивал patchwork
  - `modelpackinit` переведён на прямую зависимость от bounded
    `domain/materialization` contract
  - удалены low-signal duplicate tests:
    `application/publication/facade_test.go` и
    `publicationoperation/type_aliases_test.go`
  - branch-matrix/docs/active bundles синхронизированы с новым минимальным
    shape
  - targeted package tests, controller branch-matrix/coverage gates и
    `git diff --check` проходят после cleanup
- Slice 15 реализован bounded reduction cut:
  - удалены file-only compatibility shims:
    `application/materialization/types.go` и
    `modelpackinit/types.go`
  - полностью удалён мёртвый `internal/ports/materialization/*`, который после
    runtime cleanups больше нигде не использовался
  - low-signal delegation/alias tests больше не держат patchwork:
    `application/publication/facade_test.go` и
    `publicationoperation/type_aliases_test.go` удалены
  - targeted package tests, controller quality gates, `go test ./...` в
    `images/controller` и `git diff --check` проходят после cut
- Slice 16 реализован bounded reduction cut:
  - removed `application/publication` source-acceptance proxy file and its
    low-signal tests; those tiny rules now live directly in `modelpublish`
  - removed the one-off `publicationoperation` request-context wrapper and
    inlined `OperationContext` construction at the two real runtime call sites
  - deleted duplicate operation-phase mappers by relying on the shared phase
    value contract between persisted operation state and domain publication
    phases
  - removed dead `uploadsession.IsRunning` helper together with its test-only
    dependency
  - targeted package tests, controller quality gates, `go test ./...` в
    `images/controller` и `git diff --check` проходят после cut
  - после этого slice controller tree составляет `6512` non-test Go LOC и
    `7107` test Go LOC против предыдущего bounded baseline `6723` / `7437`;
    reduction есть, но workstream всё ещё далёк от user-target “сократить как
    минимум вдвое”
- Slice 17 реализован bounded reduction cut:
  - removed the entire detached runtime-materialization graph:
    `internal/application/materialization/*`,
    `internal/domain/materialization/*`,
    `internal/modelpackinit/*`
  - archived the superseded active runtime-materializer bundle once its code
    was removed from the live tree
  - controller README and active cleanup bundle were synced so the repo no
    longer claims a live runtime implementation that is not connected to any
    consumer path
  - controller quality gates, deadcode, `go test ./...`, `make verify` and
    `git diff --check` pass after the cut
  - controller tree now stands at `5985` non-test Go LOC and `6572` test Go
    LOC; this is materially smaller than the earlier `6512` / `7107`, but the
    workstream still remains far from the user-target “cut at least in half”
- Follow-up reduction cut after Slice 17:
  - downstream consumer `modelpublish` switched from
    `publicationoperation` compatibility aliases to direct
    `internal/ports/publication` imports
  - production compatibility shim
    `images/controller/internal/publicationoperation/types.go` removed
    completely; only test-local aliases remain in
    `publicationoperation/test_helpers_test.go`
  - publication operation ConfigMap codec/mutation/store helpers now speak the
    shared operation contract directly instead of going through a local alias
    layer
  - `go test ./...` in `images/controller`, controller quality gates,
    `make deadcode`, `make controller-coverage-artifacts`, repo-level
    `make verify` and `git diff --check` pass after the cut
  - controller tree now stands at `5950` non-test Go LOC and `6587` test Go
    LOC; production code shrank again, while the overall workstream still
    remains far from the user-target “cut at least in half”
- Slice 18 реализован bounded structural rewrite cut:
  - concrete reconciler packages now live under `internal/controllers/*`:
    `catalogstatus`, `publicationops`, `catalogcleanup`
  - concrete K8s runtime adapters now live under `internal/adapters/k8s/*`:
    `sourceworker`, `uploadsession`, `cleanupjob`, with shared OCI registry
    rendering in `adapters/k8s/ociregistry`
  - shared helper duplication is now centralized under `internal/support/*`:
    `cleanuphandle`, `modelobject`, `resourcenames`
- Follow-up virtualization alignment cut after Slice 18:
  - `application/publishobserve` now owns not only reconcile gate and runtime
    observation, but also source-vs-upload runtime orchestration through the
    shared publication ports
  - `controllers/catalogstatus/reconciler.go` no longer branches between
    `sourceworker` and `uploadsession`; it delegates the runtime execution path
    to one application use case and keeps only object loading plus persistence
    shell
  - docs and target-layout notes were synced so the repo no longer claims that
    `catalogstatus` itself performs runtime port calls
  - targeted package tests, `go test ./...` in `images/controller`,
    `make verify`, and `git diff --check` pass after the cut
- Final follow-up alignment cut for `catalogstatus`:
  - `controllers/catalogstatus/io.go` no longer imports `domain/publishstate`
    directly and no longer assembles status mutation policy inline
  - `application/publishobserve` now plans controller status / cleanup-handle
    mutations from domain observations, while the controller keeps only the
    persistence shell and update ordering
  - after this cut, concrete controller packages no longer call
    `publishstate.ProjectStatus` directly; domain projection rules stay behind
    application seams
  - targeted package tests, `go test ./...` in `images/controller`,
    `make verify`, and `git diff --check` pass after the cut
  - duplicated `Model` / `ClusterModel` status/kind/request helpers were
    removed from the controller packages and moved into `support/modelobject`
  - duplicated owner-name/label normalization and OCI env/volume rendering were
    removed from `sourceworker` / `uploadsession` / `cleanupjob`
  - new shared helper seams now have direct unit coverage in
    `support/modelobject`, `support/resourcenames` and
    `adapters/k8s/ociregistry`, instead of staying untested only because the
    repo gate does not require them
  - `go test ./...` in `images/controller` continues to pass after the package
    rewrite
- Slice 35 реализован bounded tooling/structure cut:
  - removed the ambiguous `internal/app` name and replaced it with the explicit
    composition-root package `internal/bootstrap`
  - renamed `app.go` / `app_test.go` to `bootstrap.go` / `bootstrap_test.go`
    so the tree no longer keeps a second “app” concept beside
    `internal/application`
  - controller deadcode verification is now explicit and controller-first
    through `deadcode-controller`, `deadcode-hooks` and an updated
    `tools/check-controller-deadcode.sh`
  - repo-local skills and review rules now treat ambiguous package naming and
    misleading verify output as concrete findings instead of documentation-only
    nits
- Slice 36 реализован bounded evidence cleanup cut:
  - removed scattered package-local `BRANCH_MATRIX.ru.md` files from
    `internal/domain/publication`, `internal/application/publication` and
    `internal/application/deletion`
  - replaced them with one controller-level evidence source of truth in
    `images/controller/TEST_EVIDENCE.ru.md`
  - replaced the verify hook from `check-controller-branch-matrix` to
    `check-controller-test-evidence`, so the gate now enforces complete
    controller evidence coverage without keeping local markdown files inside
    half the packages
- Slice 37 реализован bounded naming/structure cut:
  - removed the generic repeated `publication` package naming across
    `internal/`, `application/`, `domain/`, and `ports/`
  - introduced explicit role-based names:
    `publishedsnapshot`, `publishplan`, `publishstate`, `publishop`
  - rewired all controller imports and tests to the new package map
  - synced controller README / structure inventory / test-evidence inventory
    and repo-local skills so the new naming becomes the durable project rule
  - controller tree now stands at `5933` non-test Go LOC and `6782` test Go
    LOC
- Slice 19 реализован bounded test-architecture cleanup cut:
  - shared controller test scheme/object/fake-client fixtures now live under
    `internal/support/testkit`
  - `catalogstatus` and `catalogcleanup` stopped carrying their own copies of
    scheme bootstrap, base model fixtures and fake-client wiring
  - `publicationops` switched to the same shared scheme/fake-client test seam,
    so the three largest controller test packages no longer bootstrap their
    sandbox in different ways
  - `catalogcleanup` reconcile tests now use package-local test helpers for
    adapter-local options, delete-object fixtures and cleanup-job fixtures
    instead of duplicating those blocks in every test body
  - a real duplicate test on failed cleanup-job projection was removed instead
    of being kept under another name
  - branch/test strategy and repo-local controller skills now pin the
    `support/testkit + package-local test_helpers_test.go` discipline for
    future slices
  - controller tree now stands at `6062` non-test Go LOC and `6428` test Go
    LOC; production LOC grew slightly because the shared testkit is a real Go
    package, but overall controller tree still shrank by removing far more
    duplicated test code than it added
- Slice 20 реализован bounded structure-audit cut:
  - added repo-local controller inventory in
    `images/controller/STRUCTURE.ru.md`
  - each remaining folder/file in controller tree now has explicit purpose and
    placement rationale
  - controller README and repo-local skills now point future slices to that
    inventory, so package-map decisions do not depend on chat memory alone
- Slice 21 реализован bounded structural reduction cut:
  - `catalogstatus` observation+persistence shell collapsed into one honest
    adapter IO file: `io.go`
  - `catalogcleanup` observation+persistence shell collapsed into one honest
    adapter IO file: `io.go`
  - `publicationops` persisted `ConfigMap` protocol collapsed into one honest
    concrete protocol file: `configmap_protocol.go`
  - package map no longer pretends there are extra architectural seams where
    there were only tiny helper splits
  - full controller `go test`, controller gates, deadcode and repo-level
    `make verify` pass after the cut
  - controller tree now stands at `5944` non-test Go LOC and `6428` test Go
    LOC

## Residual risks

- Bundle сам по себе не исправляет текущий fat controller; это только
  corrective execution plan.
- Перед началом implementation надо будет отдельно не допустить второй active
  slug на ту же тему и не смешать refactor с новыми feature slices.
- Coverage и branch-matrix gates пока future-facing для ещё не созданных
  `domain` / `application` packages; их нужно ужесточать по мере реального
  разрезания controller runtime.
- `controllers/publishrunner` всё ещё самый жирный concrete package: ConfigMap
  protocol, store wiring и runtime wiring сидят рядом и должны быть разрезаны
  дальше.
- `StartPublication` и source-type-to-execution-mode mapping всё ещё живут в
  `application/publishplan`; этот slice сознательно не тащит их в domain.
- `catalogstatus` projection всё ещё зависит от `publishrunner` reconcile,
  если corrupted terminal state уже лежит в operation и `publishrunner`
  ещё не успел перевести его обратно в `Failed`.
- В replay coverage `publishrunner` всё ещё не закрыты более широкие
  worker/session recreation races и codec/store split пока не вынесен за
  пределы adapter package.
- `catalogcleanup` всё ещё не выделяет explicit ports для cleanup runtime/store;
  это сознательно deferred, чтобы не смешивать следующий seam-cut с текущим
  bounded slice.
- `controllers/publishrunner` tests are no longer a single monolith, but
  package-level adapter coverage is still relatively heavy and must keep
  following decision-family split instead of drifting back into one mega-file.
- `STRUCTURE.ru.md` itself is now another maintained artifact; if future
  package changes land without updating it, the document will drift and lose
  value.
- `controllers/publishrunner` всё ещё самый жирный production package; теперь
  это уже не проблема file-level patchwork, а remaining package-level
  orchestration complexity around source/upload/store/runtime seams.
- shared `OperationStore` turned out to be a fake seam: there is still no
  second store adapter behind the persisted operation `ConfigMap` protocol, so
  keeping that interface under `internal/ports/publishop` was architecture
  debt rather than useful hexagonal reuse.

## Latest slice

- Slice 22 split `publicationops` reconcile coverage into explicit
  `core/source-worker/upload-session` decision groups and removed the 1k+
  `reconciler_test.go` monolith.
- The same slice extracted shared worker-result completion mapping into
  `worker_result.go`, so source/upload runtime branches no longer duplicate
  backend result decode and snapshot projection.
- Slice 23 collapsed persisted protocol tests into one
  `configmap_protocol_test.go` file and removed duplicated mutation/status test
  files around the same `ConfigMap` boundary.
- Slice 24 moved shared runtime port implementations out of
  `controllers/publicationops` and into concrete `adapters/k8s/sourceworker`
  and `adapters/k8s/uploadsession`, deleting `publicationops/runtime.go` and
  its test.
- Slice 25 centralized canonical owner-based resource naming under
  `internal/support/resourcenames`, removed package-local `names.go` shims from
  `sourceworker` and `uploadsession`, and removed
  `ConfigMapNameFor`/`JobNameFor` style name helpers from adapter-local or
  port-local seams that did not justify owning naming policy.
- Slice 26 removed adapter-local request mirrors from `sourceworker` and
  `uploadsession`: both packages now consume shared
  `publication.OperationContext` directly, deleted their local
  `Request`/`OwnerRef` wrappers, and normalized package tests around one
  canonical operation-context fixture per adapter package.
- Slice 27 removed the remaining adapter-local `service -> runtime` proxy split
  in `sourceworker` and `uploadsession`: both packages now implement their
  shared runtime ports directly from `service.go`, while internal Pod/session
  CRUD stayed as unexported helper methods on the same concrete adapter object.
- Slice 28 introduced `adapters/k8s/ownedresource` and moved the repeated
  controlled-resource create/reuse shell
  (`SetControllerReference -> Create -> AlreadyExists -> Get`) out of
  `sourceworker` and `uploadsession`, leaving those adapters with only concrete
  Pod/Service/Secret shape and lifecycle specifics.
- Slice 29 introduced `adapters/k8s/workloadpod` and moved the repeated
  workload `Pod` shell (`EmptyDir` workspace, `/tmp` mount, registry CA
  volumes/mounts) out of `sourceworker` and `uploadsession`, leaving those
  adapters with only command/env and extra supplement differences.
- Slice 30 removed the fake shared `OperationStore` from
  `internal/ports/publication`, deleted the extra `store` object layer inside
  `controllers/publicationops`, collapsed duplicate source/upload
  decision-application shell into one helper, and shrank
  `publicationops/test_helpers_test.go` down to one canonical scenario-fixture
  layer instead of a shadow API of aliases and overlapping builders.
- Slice 31 removed `RequeueAfter` from `catalogstatus.Options`, moved the
  status polling cadence into the reconcile path where the lifecycle policy
  actually belongs, and renamed adapter tests from `runtime_test.go` to
  `service_roundtrip_test.go` after the earlier removal of the dedicated
  `runtime.go` layer.
- Slice 32 removed the same controller-option smell from
  `publicationops.Options`, localized source-worker result polling into the
  source branch itself, and deleted the extra `publicationops/store.go`
  sublayer so persisted operation mutation now lives directly in the thin
  reconcile shell around the single real `ConfigMap` protocol boundary.
- Slice 33 removed `adapters/k8s/cleanupjob` completely. Cleanup Job
  materialization now lives inside `controllers/catalogcleanup`, which is the
  only place that actually owns the delete-flow. This shrinks the K8s adapter
  tree and removes one more fake seam from the controller runtime.
- Slice 34 reduced the heaviest remaining `publicationops` protocol seam
  without splitting it into new fake layers: the protocol file now uses one
  local helper shell for repeated JSON decode/store behavior, and the protocol
  test file was rewritten from a long list of one-off checks into a smaller set
  of maintained protocol families.
- Slice 38 renamed the remaining generic concrete controller package from
  `controllers/publicationops` to `controllers/publishrunner` and rewired the
  live import graph, bootstrap wiring, controller README, structure inventory,
  and bundle notes so only historical slice logs keep the old name.
- Slice 39 refreshed `images/controller/STRUCTURE.ru.md` into a full live
  file-level inventory, especially for controller/adapters test files, so the
  next reviewer pass can challenge each remaining file directly instead of
  working against vague grouped rationale.
- Slice 40 cut three real duplication seams at once: reconcile-only status
  guard logic moved out of `publishrunner/configmap_protocol.go`, adapter-local
  `NewRuntime` proxy constructors disappeared, and `ownedresource` became one
  honest create/reuse+delete lifecycle helper instead of a narrowly named
  create-only file plus open-coded delete branches.
- Slice 41 removed the last asymmetric runtime surface between worker and
  session adapters: both now expose one `GetOrCreate` contract, and tests were
  rewired off private `getOrCreate*` helpers onto the same public adapter
  methods that production code uses.
- Controller tree now stands at `5790` non-test Go LOC and `6060` test Go LOC;
  this slice traded a small LOC increase for a stronger package boundary:
  repeated controlled K8s object IO is now one canonical helper instead of
  three open-coded create/reuse branches spread across worker and upload
  adapters, and repeated workload Pod shell is now one canonical helper instead
  of two local copies.
