## 1. Current phase

Phase-2 publication/runtime baseline уже перенесён на ai-models-owned shell.
Текущий slice не вводит новый runtime behavior, а нормализует repo-local
working surfaces после большого reset workstream.

## 2. Orchestration

`solo`

Это multi-file, но bounded normalization slice:

- live runtime contracts уже landed;
- главный риск сейчас в consistency/hygiene, а не в новом architectural выборе;
- explicit subagent delegation здесь добавила бы overhead больше, чем signal.

## 3. Slices

### Slice 1. Archive oversized predecessor and open a compact continuation bundle

Цель:

- убрать giant historical log из `plans/active/`;
- сделать current continuation bundle короткой рабочей поверхностью.

Файлы/каталоги:

- `plans/archive/2026/phase-reset-own-modelpack-and-remove-mlflow/*`
- `plans/active/phase-reset-baseline-normalization/*`

Проверки:

- `find plans/active -maxdepth 2 -name TASK.ru.md | sort`

Артефакт результата:

- oversized predecessor больше не засоряет active surface;
- current workstream продолжается в новом compact bundle, который явно
  опирается на archived predecessor
  `plans/archive/2026/phase-reset-own-modelpack-and-remove-mlflow/`.

### Slice 2. Rewrite controller evidence and structure docs to current live state

Цель:

- превратить `TEST_EVIDENCE.ru.md` обратно в current coverage inventory;
- выровнять `STRUCTURE.ru.md` под already-landed native publisher and
  streaming-first byte path;
- убрать stale historical assertions, которые уже не совпадают с live code.

Файлы/каталоги:

- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`
- при необходимости `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- `rg -n "ErrRemoteWorkspaceRequired" images/controller/TEST_EVIDENCE.ru.md images/controller/STRUCTURE.ru.md images/controller/README.md`

Артефакт результата:

- controller docs описывают current live runtime и testing discipline, а не
  intermediate migration steps;
- stale/local fallback wording удалён или явно ограничен archived predecessor
  bundle.

### Slice 3. Validate final normalized baseline

Цель:

- подтвердить, что repo-local surfaces и machine checks согласованы после
  cleanup.

Файлы/каталоги:

- touched files from slices 1-2

Проверки:

- `cd images/controller && go test ./...`
- `make verify`
- `git diff --check`

Артефакт результата:

- compact active bundle и current docs проходят repo-level verify без drift.

### Slice 4. Split residual test hotspots by decision surface

Цель:

- убрать оставшиеся test-file hotspots, которые уже балансировали на LOC-budget
  и смешивали несколько решений в одном файле;
- зафиксировать split в canonical test evidence.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*_test.go`
- `images/controller/internal/dataplane/publishworker/*_test.go`
- `images/controller/internal/controllers/workloaddelivery/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker ./internal/controllers/workloaddelivery`
- `make verify`

Артефакт результата:

- test hotspots режутся по decision surface, а не по accidental helper reuse;
- evidence перечисляет уже новые test files instead of deleted monoliths.

### Slice 5. Normalize second-order helper and matrix hotspots

Цель:

- убрать оставшиеся helper-only test monoliths, где в одном файле смешивались
  runtime fakes, reconciler wiring и payload builders;
- разрезать вторичные API/matrix tests там, где после первой волны split всё
  ещё оставались stale file names или cross-file helper drift.

Файлы/каталоги:

- `images/controller/internal/dataplane/uploadsession/*_test.go`
- `images/controller/internal/controllers/catalogstatus/*_test.go`
- `images/controller/internal/adapters/modelpack/oci/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/uploadsession ./internal/controllers/catalogstatus ./internal/adapters/modelpack/oci`
- `make verify`

Артефакт результата:

- `uploadsession` больше не держит giant helper-only file и разделён на
  session-info, probe/init, multipart completion и handoff rejection;
- `catalogstatus` helper tree разделён на runtime fakes, reconciler setup и
  runtime-handle payload helpers;
- `modelpack/oci` layer matrix больше не смешивает publish/materialize,
  validation и archive helper shell в одном файле;
- canonical evidence и structure docs ссылаются только на живые test files.

### Slice 6. Tighten remaining lifecycle and helper hotspots

Цель:

- убрать смешение backend-artifact и upload-staging delete policy в одном
  deletion test file;
- разрезать `k8s/uploadsession` lifecycle matrix на create/reuse, state
  projection и delete semantics;
- ужать helper-only seam в `sourcefetch` mirror tests по persisted store vs
  staging transport responsibilities.

Файлы/каталоги:

- `images/controller/internal/application/deletion/*_test.go`
- `images/controller/internal/adapters/k8s/uploadsession/*_test.go`
- `images/controller/internal/adapters/sourcefetch/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/application/deletion ./internal/adapters/k8s/uploadsession ./internal/adapters/sourcefetch`
- `make verify`

Артефакт результата:

- `finalize_delete` tests разделены на backend-artifact policy и
  upload-staging lifecycle;
- `k8s/uploadsession` tests больше не держат mixed lifecycle matrix в одном
  файле;
- `sourcefetch` mirror helper seam разделён на snapshot-store и staging
  transport helpers;
- current evidence перечисляет только живые file names и не хранит stale
  references к удалённым lifecycle monoliths.

### Slice 7. Normalize residual runtime-helper hotspots

Цель:

- убрать mixed runtime routing, fail-closed checks и tiny clock helper из
  одного `publishobserve` test file;
- разделить `modelpack/oci` helper-only shell на registry harness и trivial
  file assertions, чтобы текущий helper surface оставался defendable.

Файлы/каталоги:

- `images/controller/internal/application/publishobserve/*_test.go`
- `images/controller/internal/adapters/modelpack/oci/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/application/publishobserve ./internal/adapters/modelpack/oci`
- `make verify`

Артефакт результата:

- `publishobserve` больше не держит mixed ensure-runtime test file и режется
  на source-worker path, upload-session path, fail-closed/runtime-error
  branches, clock normalization и thin helper seam;
- `modelpack/oci` helper-only shell разделён на registry harness и tiny file
  assertions вместо одного oversized support file;
- canonical evidence и structure docs ссылаются только на живые helper and
  runtime proof files.

### Slice 8. Tighten remaining upload-admission and cleanup-helper hotspots

Цель:

- убрать смешение upload-session validation, probe classification и
  probe-shape checks в одном `ingestadmission` test file;
- разделить `catalogcleanup` helper-only shell на reconciler/build seam и
  cleanup runtime-state/assert helpers.

Файлы/каталоги:

- `images/controller/internal/domain/ingestadmission/*_test.go`
- `images/controller/internal/controllers/catalogcleanup/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/domain/ingestadmission ./internal/controllers/catalogcleanup`
- `make verify`

Артефакт результата:

- `ingestadmission` tests больше не держат mixed upload file и режутся на
  session validation, probe classification и probe-shape checks;
- `catalogcleanup` helper tree больше не держит один oversized helper-only
  file и разделён на reconciler/build helpers vs cleanup runtime-state/assert
  helpers;
- canonical evidence и structure docs перечисляют только живые file names и
  helper seams.

### Slice 9. Normalize remaining registry and mirror staging helper seams

Цель:

- разделить `modelpack/oci` registry harness на server shell и registry-state
  handler helpers;
- разделить `sourcefetch` mirror staging helper surface на staging
  client/object-read seam и HTTP multipart upload seam.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*_test.go`
- `images/controller/internal/adapters/sourcefetch/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci ./internal/adapters/sourcefetch`
- `make verify`

Артефакт результата:

- `modelpack/oci` helper surface больше не держит один mixed registry file и
  разделён на registry server vs registry state helpers;
- `sourcefetch` mirror staging helpers больше не смешивают object-read client
  contract и HTTP multipart upload shell в одном файле;
- canonical evidence и structure docs перечисляют только живые helper seams.

### Slice 10. Normalize catalogmetrics and registry state helper surfaces

Цель:

- разделить `catalogmetrics` test surface на state-metric emission,
  incomplete-status behavior и thin gather/assert helpers;
- разделить `modelpack/oci` registry-state helper surface на dispatch shell,
  upload-state handlers и manifest/blob content handlers.

Файлы/каталоги:

- `images/controller/internal/monitoring/catalogmetrics/*_test.go`
- `images/controller/internal/adapters/modelpack/oci/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/monitoring/catalogmetrics ./internal/adapters/modelpack/oci`
- `make verify`

Артефакт результата:

- `catalogmetrics` больше не держит один mixed collector proof file и режется
  на metric emission, incomplete-status behavior и helper seam;
- `modelpack/oci` registry-state helpers больше не смешивают dispatch,
  upload-session state и manifest/blob content handlers в одном файле;
- canonical evidence и structure docs перечисляют только живые file names и
  helper seams.

### Slice 11. Normalize direct HF and sourceworker service proof surfaces

Цель:

- разделить `sourcefetch` direct `HuggingFace` proofs на object-source happy
  path vs fail-closed planning вместо одного mixed fetch file;
- разделить `k8s/sourceworker` service proofs на owner identity, auth
  projection и concurrency/queued-handle semantics вместо одного mixed service
  shell;
- синхронизировать canonical evidence/structure surfaces под новые live file
  names.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*_test.go`
- `images/controller/internal/adapters/k8s/sourceworker/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/sourceworker ./internal/adapters/sourcefetch`
- `cd images/controller && go test ./...`
- `make verify`

Артефакт результата:

- `sourcefetch` direct `HuggingFace` tests больше не смешивают happy-path
  object-source planning с fail-closed planning proofs в одном файле;
- `k8s/sourceworker` service tests больше не смешивают owner identity, auth
  projection и concurrency semantics в одном service proof file;
- canonical evidence и structure docs перечисляют только живые filenames для
  direct `HuggingFace` и `sourceworker` service surfaces.

### Slice 12. Tighten residual code and helper seams under current LOC ceiling

Цель:

- разрезать оставшиеся production files, которые ещё смешивают несколько
  responsibilities в одной boundary, несмотря на уже приемлемый общий LOC;
- убрать крупнейший helper-only `direct upload` test monolith, чтобы test tree
  оставался согласованным с production cleanup.

Файлы/каталоги:

- `images/controller/internal/domain/publishstate/*.go`
- `images/controller/internal/adapters/uploadstaging/s3/*.go`
- `images/controller/internal/adapters/sourcefetch/*.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*.go`
- `images/controller/internal/adapters/modelpack/oci/*_test.go`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/controller/STRUCTURE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/domain/publishstate ./internal/adapters/uploadstaging/s3 ./internal/adapters/sourcefetch ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/modelpack/oci`
- `make verify`

Артефакт результата:

- `publishstate` condition/status surface больше не смешивает phase status
  builders, ready snapshot projection и generic condition-setter helpers в
  одном файле;
- `uploadstaging/s3` object I/O больше не смешивает operation methods и
  validation/error formatting shell;
- `sourcefetch` archive inspection больше не держит entrypoint, tar-walk,
  path normalization и summary helpers в одном mixed file;
- `uploadsessionstate` secret surface больше не смешивает secret shaping,
  secret decoding и parse helpers в одном файле;
- `modelpack/oci` direct upload tests больше не держат один oversized
  helper-only file, а разделены по server/backend/protocol seams;
- canonical evidence и structure docs отражают новые live file names.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: predecessor уже архивирован, а
continuation bundle создан. Если Slice 2 окажется спорным, можно вернуться к
historical docs без риска для runtime behavior.

## 5. Final validation

- `cd images/controller && go test ./...`
- `make verify`
- `git diff --check`
