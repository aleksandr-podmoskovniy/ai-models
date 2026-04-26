## Review gate — 2026-04-24

### Findings

Критичных замечаний по текущему reduction slice нет.

Дополнительный финальный reviewer (`Aegis`) не нашёл конкретных
code-regression findings, но потребовал закрыть evidence/hygiene gaps:

- зафиксировать reviewer output в текущем bundle;
- убрать untracked root-generated `.VSCodeCounter/` из рабочей поверхности;
- добавить focused tests для нового `archiveio` boundary;
- явно оставить residual tests для `uploadsessionstate` terminal guards и
  `completeDirectUpload` mismatch paths, если они не входят в текущий
  reduction slice.

Дополнительный reviewer по continuation slice (`Sentinel`) не нашёл конкретных
findings в `publishworker` archive-flow consolidation:

- helper остался package-local и не стал cross-context artifact framework;
- local-vs-staged IO split сохранён;
- staged cleanup остаётся после успешного publish;
- touched files в slice: `134`, `57`, `121`, `225` строк;
- существующие `publishworker` tests покрывают local tar/zip/tar.zst, staged
  tar/zip/tar.zst, archive reader wiring, zip `SizeBytes` и staged cleanup.

Финальный reviewer после последних slices (`Aegis the 2nd`) не нашёл
конкретных findings:

- `publishworker` consolidation остался package-local и сохранил
  local-vs-staged byte paths, zip `SizeBytes` и staged cleanup timing;
- publication control-plane cleanup является deletion-only вокруг dead DTO и
  empty plan object;
- controller config file-size cleanup является прямым переносом helper между
  соседними файлами package `main` без semantic drift;
- residual risks совпадают с уже зафиксированными test gaps:
  `uploadsessionstate` terminal guards и `completeDirectUpload` mismatch path.

Финальный reviewer после continuation slices (`Sentinel the 2nd`) не нашёл
code-level regressions, но нашёл bundle drift:

- `TASK.ru.md` отставал от реализованных DMCR/sourcefetch/hooks slices;
- drift исправлен: scope и acceptance criteria теперь включают sourcefetch
  HuggingFace test cleanup, DMCR direct-upload service/test splits,
  garbagecollection test splits и hooks sync-artifacts-secrets test split;
- единственный `>=350` Go file после scan — generated
  `api/core/v1alpha1/zz_generated.deepcopy.go`.

Continuation read-only reviewers после hand-written LOC cleanup:

- `repo_architect` подтвердил safe production slice: удалить fake
  upload-session application hop и оставить validation в существующих
  `publishop` / `ingestadmission` boundaries;
- `integration_architect` заблокировал runtime delivery / node-cache /
  modeldelivery reductions как topology/auth/copy-count work, не cleanup;
- `backend_integrator` подтвердил safe package-local cleanup в
  `catalogcleanup` DMCR GC request Secret mutator без изменения backend
  protocol/storage semantics.
- `integration_architect` по Slice 19 разрешил только same-package
  consolidation generated archive plumbing: direct ranged object-source fast
  path должен остаться отдельным, descriptor identity должен сохраниться
  byte-for-byte, а pipe helper обязан сохранить precedence `writeErr` перед
  `closeErr`.

Финальный reviewer по latest continuation (`Aegis the 3rd`) не нашёл
correctness или boundary-regression findings. Единственный low missing-check
gap был закрыт дополнительным seam-level test:

- `Service.GetOrCreate` теперь явно проверяет invalid request path и отсутствие
  Secret writes до materialization;
- прежние residual gaps по `uploadsessionstate` terminal guards и DMCR
  complete mismatch остаются открытыми, но это не новые регрессии.

Финальный reviewer по Slice 19 (`Harbor the 3rd`) не нашёл blocking findings:

- direct ranged object-source fast path остался отдельным;
- descriptor construction и normalized compression сохранены в helper без
  изменения field content;
- pipe path сохраняет `writeErr` precedence перед `closeErr`;
- два missing-check gaps закрыты seam-level tests для descriptor identity и
  `writeErr` / `closeErr` ordering.

### Что проверено

- создан отдельный canonical bundle для audit-first reduction workstream;
- canonical active bundle reused и расширен notes/plan вместо создания sibling
  source of truth;
- read-only delegation использована для выбора следующего slice, а не после
  хаотичных code edits;
- `modelpack/oci` scope остался bounded и не меняет `DMCR` protocol /
  checkpoint semantics;
- `uploadsessionstate` cleanup остался внутри одной boundary и не меняет:
  - secret data/annotation schema;
  - phase semantics;
  - package-level contract для внешних callers;
  - runtime/store boundary.
- shared `archiveio` helper вынес только повторяющиеся filesystem/archive
  primitives и не меняет archive selection или publication contracts;
- DMCR direct-upload test cleanup меняет только same-package test harness,
  production code не затронут.
- publication control-plane cleanup не переносит public status ownership в
  controller boundary; status mutation move сознательно отложен после
  `api_designer` warning;
- upload-session validation cleanup удаляет fake application hop, но сохраняет
  `publishop.Request.Validate` и `ingestadmission.ValidateUploadSession` как
  источники validation rules;
- `catalogcleanup` GC request cleanup остался package-local и не выносит DMCR
  wire keys в shared/public contract;
- `modelpack/oci` generated archive cleanup остался same-package helper; raw
  layer path и direct ranged object-source dispatch не изменены;
- generated archive descriptor fields, compression normalization,
  range slicing и `CloseWithError(writeErr)` ordering сохранены;
- generated archive helper теперь имеет focused tests на exact descriptor
  fields и pipe close/error precedence;
- controller config file-size cleanup не меняет env/flag parsing и bootstrap
  option semantics;
- `.VSCodeCounter/` распознан как локальный generated artifact от внешнего LOC
  счётчика и добавлен в `.gitignore`, без удаления пользовательских данных.

### Reduction evidence

- по трём live transport-файлам
  `direct_upload_transport.go` +
  `direct_upload_transport_raw.go` +
  `direct_upload_transport_raw_flow.go`
  LOC уменьшился с `739` до `735`;
- по `uploadsessionstate` continuation cluster
  `client.go` +
  `secret.go` +
  `secret_parse.go` +
  `multipart_state.go` +
  `phase_mutation.go` +
  `token_storage.go`
  LOC уменьшился с `902` до `792`;
- archive helper consolidation убрал локальные копии из `sourcefetch`,
  `modelpack/oci` и `publishworker`, включая duplicated ranged `ReaderAt`;
- `images/dmcr/internal/directupload/service_test.go` (`1140` строк) заменён
  focused same-package test files без потери исходных `go test -list Test`
  сценариев;
- текущие изменённые Go-файлы суммарно уменьшились с `10581` до `10014` строк:
  net `-567` physical Go lines с учётом новых `secret_codec.go`,
  `archiveio`, focused `archiveio` tests, `publishworker`
  archive-flow helper, publication control-plane DTO cleanup и controller
  config file-size cleanup, sourcefetch HuggingFace test fixture cleanup,
  upload-session fake-hop removal, `catalogcleanup` GC request mutator cleanup,
  `modelpack/oci` generated archive helper and seam tests, DMCR direct-upload
  file splits, garbagecollection test splits и hooks test split;
- net считается по всем modified/deleted/untracked Go paths текущего worktree,
  а не по `git diff --stat`, который не учитывает untracked новые files.
- после Slice 20 и reviewer gap fixes общий physical Go LOC в `images/`
  уменьшился с `69100` до `68346`: net `-754` physical Go lines за
  continuation turn (`prod: 39335 -> 39201`, `tests: 29765 -> 29145`).

### Что именно упрощено

- common sealing completion path теперь один;
- single-call `uploadNext*` helpers схлопнуты обратно в loops;
- raw/described direct-upload branches оставляют только реально отличающуюся
  семантику;
- `uploadsessionstate` secret codec/mutator cluster больше не размазан по пяти
  small files;
- `Client.SaveMultipart` и `SaveMultipartSecret` теперь используют один mutation
  path;
- duplicated uploaded-part validation теперь одна;
- phase terminal writes больше не дублируются между free functions и `Client`.
- archive target path normalization, tar reader selection, tar entry extraction
  и ranged ZIP `ReaderAt` живут в одном `internal/support/archiveio` package;
- hidden `NewTarReader` close-discard helper удалён; tar extraction теперь
  использует `NewClosableTarReader` и закрывает gzip/zstd wrapper;
- direct-upload HTTP tests используют общий harness для service/server,
  start-token и complete-request flow.
- `publishworker` local/staged archive publication paths теперь используют
  один package-local `publishUploadArchive` workflow; source-specific code
  только готовит local path или staged object reader/range metadata.
- `publishop.Phase` / `publishop.Status` удалены как unused internal DTO.
- `publishplan.UploadSessionPlan`, `IssueUploadSession` и
  `UploadSessionIssueRequest` удалены; upload-session lifecycle валидирует
  request напрямую через `publishop` и `ingestadmission`.
- invalid upload-session request path теперь покрыт через `Service.GetOrCreate`
  и подтверждает отсутствие Secret writes.
- `images/controller/cmd/ai-models-controller/config.go` уменьшен с `365` до
  `334` строк переносом selector JSON parsing в существующий config support
  surface; в `images/controller` не осталось Go-файлов `>=350` physical LOC.
- HuggingFace sourcefetch tests используют same-package fixture helpers для
  global stub/baseURL restore и стандартных `owner/model` fixtures; production
  sourcefetch path не изменён.
- DMCR direct-upload `service_test.go` заменён focused test files по
  auth/start, completion default policy, recovery/cleanup, verification и
  support harness; все `go test -list Test` сценарии сохранены.
- DMCR direct-upload `service.go` разрезан same-package на HTTP handlers,
  completion orchestration, sealed storage helpers и server wrapper; новых
  interfaces/contracts не появилось.
- Garbagecollection и sync-artifacts-secrets oversized tests разрезаны через
  same-package support test files; среди hand-written Go-файлов больше нет
  `>=350` physical LOC.
- `catalogcleanup` create/refresh paths для DMCR GC request Secret используют
  один local mutator для labels/annotations/token payload; timestamp arm,
  `done` cleanup и stale direct-upload token cleanup покрыты тестом.
- `modelpack/oci` synthetic archive descriptors используют один same-package
  helper для digest/diffID/size/descriptor fields.
- `modelpack/oci` synthetic archive range streams используют один helper для
  pipe/compression/offset/limit и сохраняют `writeErr` before `closeErr`
  semantics.
- direct ranged object-source fast path остался отдельным, чтобы interrupted
  upload по uncompressed object-source не получил лишний generated archive
  pipe.
- reviewer gaps по Slice 19 закрыты tests:
  `TestDescribeGeneratedArchiveLayerPreservesDescriptorFields`,
  `TestCloseGeneratedArchivePipePrefersWriteError` и
  `TestCloseGeneratedArchivePipeReturnsCloseErrorWithoutWriteError`.
- Slice 20 удалил `application/publishplan` из build graph:
  source-worker plan shaping теперь same-package в `adapters/k8s/sourceworker`,
  runtime-mode selection и runtime `GetOrCreate` orchestration — в
  `controllers/catalogstatus`, а `application/publishobserve` остался
  boundary для runtime-handle to public-status observation/mutation semantics.
- `publishobserve/status_mutation.go`, `observe_source_worker.go`,
  `observe_upload_session.go` и `runtime_result.go` не переносились в
  controller boundary, поэтому public status ownership не смещён.
- Удалены wrapper-only tests вокруг бывшего `EnsureRuntimeObservation` hop;
  remaining evidence остаётся на domain observation decisions,
  `publishobserve` handle mapping и controller reconcile paths.
- Reviewer gaps по Slice 20 закрыты focused tests:
  `runtime_mode_test.go` для moved reconcile gate,
  `runtime_observation_test.go` для controller-local runtime error branches и
  staged-upload positive branch в `sourceworker/validation_test.go`.
- Обновлены `images/controller/README.md`, `STRUCTURE.ru.md` и
  `TEST_EVIDENCE.ru.md`, чтобы repo docs больше не ссылались на удалённый
  `publishplan`.

### Gate handling

- первая попытка binpack в один `secret.go` выбила controller file-size gate;
- split скорректирован до `secret.go` (`342` lines) +
  `secret_codec.go` (`258` lines), после чего quality gate снова зелёный.

### Проверки

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/bootstrap ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession`
- `cd images/controller && go test ./internal/support/archiveio ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker`
- `cd images/controller && go test ./internal/support/archiveio`
- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/ports/publishop`
- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/ports/publishop ./internal/domain/ingestadmission`
- `cd images/controller && go test ./internal/controllers/catalogcleanup ./internal/application/deletion ./internal/adapters/k8s/directuploadstate`
- `cd images/controller && go test ./internal/adapters/modelpack/oci -run 'TestAdapterPublishAndMaterializeArchiveSourceLayer|TestAdapterPublishAndMaterializeZipArchiveSourceLayer|TestAdapterPublishAndMaterializeZipArchiveSourceReaderLayer|TestAdapterPublishAndMaterializeZstdArchiveSourceLayer|TestAdapterPublishAndMaterializeZstdArchiveSourceReaderLayer|TestAdapterPublishObjectSourceUsesRangeReadsOnInterruptedUpload'`
- `cd images/controller && go test ./internal/adapters/modelpack/oci -run 'TestDescribeGeneratedArchiveLayerPreservesDescriptorFields|TestCloseGeneratedArchivePipePrefersWriteError|TestCloseGeneratedArchivePipeReturnsCloseErrorWithoutWriteError'`
- `cd images/controller && go test ./cmd/ai-models-controller`
- `cd images/controller && go test ./internal/adapters/sourcefetch`
- `cd images/controller && go test ./internal/controllers/catalogstatus ./internal/application/publishobserve ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/ports/publishop ./internal/domain/ingestadmission`
- `cd images/dmcr && go test ./internal/directupload`
- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/hooks && go test ./pkg/hooks/sync_artifacts_secrets`
- physical LOC scan: only generated
  `api/core/v1alpha1/zz_generated.deepcopy.go` remains `>=350` LOC
- `make verify`
- `git diff --check`

### Residual risks

- repo-wide target `~54 643 -> ~25 000` Go code lines не достижим только
  exact duplicate cleanup; нужна отдельная крупная reduction program по
  publication pipeline и/или delivery boundary.
- high-signal candidates уже известны, но intentionally не смешаны в этот diff:
  - small `catalogcleanup` runtime resource inventory helper;
  - Helm render-only S3/TLS helper dedupe;
  - `dataplane/publishworker` только после отдельного byte-path review;
  - publication pipeline collapse между `sourcefetch`, `publishworker` и
    `modelpack/oci`;
- delivery/node-cache может дать большой LOC win, но сейчас есть phase-boundary
  inconsistency между repo instructions и development docs; этот вопрос нельзя
  резать incidental cleanup diff.
- у `uploadsessionstate` всё ещё нет focused test на guard behavior
  `MarkPublishingSecret` / `MarkCompletedSecret` для уже terminal secret и на
  точный `stagedHandle` preservation split между
  `MarkPublishingFailedSecret` и client terminal mutations.
- у общего `completeDirectUpload` path всё ещё нет targeted failure test на
  mismatched digest/size в ответе `DMCR complete`; protocol preservation здесь
  пока подтверждается косвенно.
- `api/core/v1alpha1/zz_generated.deepcopy.go` остаётся выше пользовательского
  физического LOC лимита как generated artifact; вручную не редактировался.
