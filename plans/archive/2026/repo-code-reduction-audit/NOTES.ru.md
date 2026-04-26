## Read-only audit notes — 2026-04-24

### Candidate 1. `dataplane/publishworker`

Read-only review (`repo_architect`) подтвердил высокий LOC и локальное
дублирование в cluster:

- `upload.go`
- `upload_streaming.go`
- `upload_stage_streaming.go`
- `upload_stage_archive_streaming.go`
- `huggingface_streaming.go`
- `huggingface_streaming_layers.go`

Вывод:

- boundary defendable и может быть следующим большим reduction target;
- нельзя смешивать этот slice с `sourcefetch` или `modelpack/oci`;
- лучший будущий ход там — схлопнуть archive fast-path и object-source publish
  helpers.

### Candidate 2. `adapters/k8s/uploadsessionstate`

Read-only review (`explorer`) и локальный audit показали более безопасный
continuation slice:

- `SaveMultipartSecret` и `Client.SaveMultipart` держат один mutation path;
- terminal-phase writes частично дублируются между free functions и `Client`;
- один secret codec/mutator cluster размазан по:
  - `secret.go`
  - `secret_parse.go`
  - `multipart_state.go`
  - `phase_mutation.go`
  - `token_storage.go`

Выбор текущего slice:

- сначала режем `uploadsessionstate`, потому что там меньше risk of boundary
  drift и уже есть package-local safety net в `secret_test.go`;
- `publishworker` остаётся следующим крупным кандидатом после этого slice.

### Candidate 3. Cross-package archive helper duplication

Read-only review (`explorer`) отдельно подтвердил точное code-copy duplication
между:

- `sourcefetch/archive_extract.go`
- `sourcefetch/tar_gzip.go`
- `modelpack/oci/materialize_support.go`
- `modelpack/oci/publish_archive_source.go`
- `modelpack/oci/publish_archive_source_zip.go`

Вывод:

- это сильный deletion-heavy target, но он уже пересекает две живые boundaries;
- current slice специально не трогает его, чтобы не смешивать reduction в
  `uploadsessionstate` с cross-package helper consolidation;
- после текущего пакета это один из лучших кандидатов на следующий bounded
  continuation diff.

### Numeric target update

Пользователь задал целевой ориентир: `~25 000` Go code lines вместо текущих
`~54 643`.

Read-only reviews уточнили:

- exact duplication alone не даст нужный масштаб;
- большой primary surface должен быть publication pipeline:
  `sourcefetch` + `publishworker` + `modelpack/oci` + adjacent profile/format
  helpers;
- отдельно нужно решить phase boundary по delivery/node-cache, потому что там
  большой LOC, но docs/instructions расходятся по тому, является ли это phase-1
  baseline или phase-2 expansion;
- безопасные short-term wins остаются полезными как LOC budget reduction, но
  не заменяют архитектурный крупный surface.

### Candidate 4. DMCR direct-upload tests

Read-only duplicate audit нашёл крупный локальный test-only cluster:

- `images/dmcr/internal/directupload/service_test.go`
- repeated `NewService`, `httptest.NewServer`, start/decode/complete flow.

Выбор continuation slice:

- это безопасный LOC win без изменения production behavior;
- narrow validation: `cd images/dmcr && go test ./internal/directupload`.

### Continuation constraints update

Пользователь повторно зафиксировал engineering constraints для дальнейшего
сокращения:

- hexagonal architecture / SOLID сохраняются;
- монолитные "artifact IO" / "runtime wiring" frameworks запрещены;
- файлы должны оставаться `<350` LOC;
- cyclomatic complexity по repo gate должна оставаться `<15`;
- формы decomposition сверяются с
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/virtualization-artifact`.

Reference observation:

- `virtualization-artifact` использует bounded controller/service/internal
  packages по feature context;
- это поддерживает feature-local helpers и handler/service boundaries;
- это не поддерживает склейку разных runtime/storage/auth concerns в один
  cross-domain package.

### Continuation read-only reviews

`integration_architect`:

- нельзя резать сейчас `DMCR` trust/verification semantics как LOC cleanup;
- нельзя резать `nodecache` / `modeldelivery` / `nodecacheruntime` /
  `runtimehealth` как phase-1 cleanup, это topology decision;
- нельзя объединять `ociregistry`, `storageprojection` и upload-session auth в
  generic projection framework;
- safe phase-1 reductions: package-local `publishworker` helper collapse,
  already-scoped `archiveio`, `uploadsessionstate`, test-only cleanup.

`explorer`:

- лучший немедленный implementation slice:
  `dataplane/publishworker` upload archive consolidation;
- дублируются local/staged archive publication flow:
  suffix classification, archive inspection, input-format guard, profile
  summary, one tar `PublishLayer`, `resolveAndPublishWithLayers`, cleanup;
- expected impact: `-90..-140` net LOC;
- helper должен остаться package-local, потому что это publication policy, а
  не generic archive IO.

`repo_architect`:

- самый крупный правильный следующий target:
  `publication_control_plane` around `controllers/catalogstatus` +
  `application/publishplan` + `application/publishobserve` +
  `domain/publishstate` + `ports/publishop`;
- там много in-process DTO/decision seams и меньше настоящих replaceable
  boundaries;
- expected impact: `-700..-1100` production LOC, но это отдельный крупный
  controller-local service collapse;
- byte-path publication collapse больше, но не должен быть первым из-за риска
  boundary drift.

### Current continuation decision

Текущий кодовый slice:

- сначала выполнить bounded `publishworker` archive-flow consolidation;
- не смешивать его с `publication_control_plane`;
- после успешной проверки открыть отдельный controller-local service slice для
  `catalogstatus` / `publishplan` / `publishobserve` / `publishstate` /
  `publishop`.

### Publication control-plane narrowing

Additional read-only reviews по первому controller-local target:

- `api_designer`: не переносить `publishobserve/status_mutation.go` в
  `controllers/catalogstatus` без отдельного API decision; этот код формирует
  public `ModelStatus` contract из runtime observations и cleanup/requeue
  decisions, поэтому перенос в controller может вернуть ownership public status
  semantics обратно в presentation boundary;
- `repo_architect` и `explorer`: считали move технически возможным как LOC win,
  но только если это будет отдельный status-plan unit, а не смешивание с
  controller IO.

Decision:

- status mutation move не входит в текущий implementation slice;
- вместо этого выполнен deletion-only cleanup fake DTO:
  `publishop.Phase` / `publishop.Status`, пустой `UploadSessionPlan` и
  прокидывание фиктивного plan object через upload-session lifecycle;
- это не меняет API/status semantics и не смешивает controller, domain и
  persistence concerns.

### Controller file-size cleanup

Additional local scan после `make verify` нашёл один untouched file-size
violation относительно пользовательского правила `<350` physical LOC:

- `images/controller/cmd/ai-models-controller/config.go` — `365` строк.

Decision:

- не разбивать bootstrap option assembly на новый framework;
- перенести selector JSON parsing в существующий config support surface
  `resources.go`, где уже живёт parsing config-derived Kubernetes resource
  requirements;
- результат: `config.go` `334` строки, `resources.go` `112` строк, Go-файлов
  `>=350` physical LOC в `images/controller` не осталось.

### Next continuation reviews

`integration_architect`:

- не расширять текущий data-plane cleanup в `modelpack/oci` direct-upload
  protocol, HuggingFace mirror production path или runtime delivery/node-cache;
- эти areas владеют auth/storage/trust/copy-count/topology semantics и не
  являются safe LOC cleanup;
- safe continuation должен быть non-byte-path или package-local.

`explorer`:

- safest deletion-heavy next slice: HuggingFace sourcefetch test-fixture
  cleanup;
- затрагивает только same-package tests, repeated global stub/baseURL restore
  blocks и fixture literals;
- не меняет production HuggingFace fetch/mirror behavior, public API/status
  или cross-package boundaries.

Decision:

- текущий implementation slice — `sourcefetch` HuggingFace test fixtures;
- `publishobserve -> catalogstatus` collapse ждёт отдельный `api_designer`
  verdict, потому что ранее status-mutation move был flagged как public status
  ownership risk;
- DMCR `directupload` остаётся следующим file-size target, но production split
  отдельно даст в основном file-size compliance, а не large net LOC reduction.

### Aggressive LOC continuation review

Пользователь повторно указал, что текущий net reduction недостаточен для цели
`~25k` Go code lines.

Read-only reviews по крупным targets:

- `repo_architect`: самый большой immediate target — publication byte-path
  (`sourcefetch` + `publishworker` + `modelpack/oci`), но только если границы
  остаются раздельными: source semantics / publication workflow / OCI sink.
  Blanket merge в generic artifact IO запрещён.
- `integration_architect`: implementation-ready без backend/runtime redesign
  сейчас только controller-local publication control-plane collapse:
  `catalogstatus` + controller-only parts of `publishobserve` +
  `publishplan` + adjacent `publishop`.
- оба review согласны, что `nodecache`, `modeldelivery`, `workloaddelivery`,
  `nodecacheruntime`, `runtimehealth` нельзя резать как cleanup: это topology
  / runtime delivery surface, а не dead code.

Decision:

- текущий implementation slice — controller-local publication control-plane
  collapse;
- не переносить `publishobserve/status_mutation.go`,
  `observe_source_worker.go`, `observe_upload_session.go` и
  `runtime_result.go` в controller: они формируют public status semantics;
- удалить `publishplan` как отдельный fake application hop:
  source-worker planning принадлежит `adapters/k8s/sourceworker`, а
  execution-mode decision принадлежит catalog status orchestration.

`api_designer` follow-up:

- полный `publishobserve -> catalogstatus` collapse небезопасен;
- `ObserveSourceWorker` / `ObserveUploadSession` выбирают fields, которые через
  `publishstate.ProjectStatus` становятся public `status.phase`,
  `status.progress`, `status.upload` и condition reason/message;
- `runtime_result.go` декодирует internal worker output format и не должен
  переезжать в package, который persist-ит public `Model`/`ClusterModel`
  status;
- безопасно двигать только controller-only orchestration вроде
  `reconcile_gate.go` / `ensure_runtime.go`, но не весь observation/status
  bridge.

Decision update:

- full `publishobserve` collapse заблокирован до отдельного API/status
  ownership decision;
- текущий slice остаётся sourcefetch test-only cleanup.

### DMCR direct-upload file-size review

`backend_integrator`:

- safe only as same-package decomposition inside
  `images/dmcr/internal/directupload`;
- do not introduce a new cross-cutting upload workflow/coordinator/interface;
- do not move request/response structs into `contract.go`;
- preserve exact `handleComplete` step order, log event vocabulary,
  sealed-metadata/repository-link behavior and cleanup-on-link-failure;
- lowest-risk next slice: split already-dirty `service_test.go` by test
  decision surface before touching production `service.go`.

Decision:

- implement test-file split first;
- production `service.go` split remains a later file-size compliance slice,
  not a semantic simplification.

After test split:

- `service_test.go` monolith removed;
- tests are split into auth/start, default completion policy,
  recovery/cleanup, verification helpers and support harness;
- all directupload tests remain listed by `go test ./internal/directupload
  -list Test`;
- production `service.go` split completed only as same-package decomposition:
  HTTP handlers, completion orchestration, sealed storage helpers and server
  wrapper;
- no new backend interfaces or contracts were introduced.

### Hand-written file-size cleanup

Additional physical LOC scan showed remaining hand-written violations:

- `images/dmcr/internal/garbagecollection/directupload_inventory_test.go`;
- `images/dmcr/internal/garbagecollection/runner_test.go`;
- `images/hooks/pkg/hooks/sync_artifacts_secrets/main_test.go`.

Actions:

- split direct-upload inventory fake store/build-report helpers into focused
  support test files;
- split garbagecollection runner log assertion helpers;
- split sync-artifacts-secrets snapshot/value assertion helpers.

Result:

- no hand-written Go files remain `>=350` physical LOC;
- only `api/core/v1alpha1/zz_generated.deepcopy.go` remains above that bound as
  generated code.

### Next continuation reviews after hand-written LOC cleanup

`repo_architect` (`Delta the 2nd`):

- `publishworker` archive-flow collapse is only partly finished, but further
  work must stay package-local and must not become generic artifact IO;
- upload-session validation still crosses a fake application hop:
  `publishop.Request.Validate` already owns source-shape checks,
  `ingestadmission.ValidateUploadSession` already owns owner/identity
  admission, while `publishplan.IssueUploadSession` only rewraps the same data;
- recommended immediate safe production slice: remove
  `UploadSessionIssueRequest` / `IssueUploadSession` and let
  `uploadsession` call existing port/domain validation directly;
- non-goals: no `publishobserve`/status collapse, no node-cache/runtime
  topology reduction, no new shared publication framework.

`integration_architect` (`Pulse the 2nd`):

- further runtime delivery / node-cache / modeldelivery reductions are blocked
  as topology/auth/copy-count work, not cleanup;
- `DMCR` completion and `publishworker` byte paths should not be reduced
  further without dedicated backend/runtime review;
- render-only Helm helper dedupe is safe later, but it is template LOC, not the
  current Go LOC target.

Decision:

- current implementation slice removes the fake upload-session application hop;
- validation remains split along existing boundaries:
  `publishop.Request.Validate` for publication request shape and
  `ingestadmission.ValidateUploadSession` for owner/identity/upload admission;
- no public API/status, secret schema, runtime byte path or storage semantics
  change is intended.

`backend_integrator` (`Vector the 3rd`):

- safe backend-facing controller cleanup exists in
  `controllers/catalogcleanup/gc_request.go`;
- create and refresh paths both build the same DMCR GC request labels,
  annotations and optional direct-upload token payload;
- helper must stay package-local in `catalogcleanup`; do not dedupe controller
  and DMCR GC key names into a shared backend contract package;
- preserve immediate-arm semantics: `requested-at` and `switch` use the same
  timestamp, `done` is removed on refresh, empty direct-upload token removes
  only its payload key.

Decision:

- after fake upload-session hop removal, take the local GC request mutator
  cleanup as an additional small safe slice;
- do not touch `DMCR` storage/protocol, `completion.go`, direct-upload byte
  path or public API.

### Next `modelpack/oci` generated archive layer review

`explorer` (`Jason the 2nd`) found a safe same-package production target:

- duplicate generated archive layer descriptor and range-stream plumbing lives
  in `publish_archive_source.go`, `publish_object_source.go`,
  `publish_layers_describe.go` and `publish_layers_stream.go`;
- target helpers must stay in `images/controller/internal/adapters/modelpack/oci`;
- raw layer path, direct ranged object-source path, `modelpack` ports, `DMCR`
  protocol, artifact format and runtime topology must not change;
- preserve `archiveWriter.Close()` ordering and `CloseWithError(writeErr)`
  precedence over close errors;
- preserve descriptor fields exactly: digest, diffID, size, media type, target
  path, base, format and normalized compression.

Decision:

- next implementation slice consolidates only synthetic archive layer
  descriptor/range stream generation behind same-package helpers;
- no cross-package artifact IO abstraction and no publication pipeline merge.
