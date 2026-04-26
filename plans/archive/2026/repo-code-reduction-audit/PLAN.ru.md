## 1. Current phase

Cross-phase corrective engineering hygiene over landed publication/runtime
baseline.

Это не новый product slice, а systematic code reduction continuation поверх
живого baseline.

## 2. Orchestration

`full`

Причина:

- пользовательский запрос repo-wide и intentionally broader, чем один diff;
- безопасный continuation slice выбирается только после read-only audit
  нескольких boundaries;
- уже использованы несколько read-only subagents для narrowing следующего slice;
- перед завершением нужен `review-gate`, а при substantial continuation с
  delegation возможен дополнительный `reviewer`.

Read-only reviews до реализации:

- `repo_architect` — проверить следующий высокий LOC target без boundary drift;
- `explorer` — найти локальные duplication clusters и безопасный continuation
  slice.

## 3. Slices

### Slice 1. Зафиксировать reduction workstream

Цель:

- явно отрезать audit-first cleanup от unsafe blanket rewrite narrative.

Файлы/каталоги:

- `plans/active/repo-code-reduction-audit/*`

Проверки:

- manual consistency review

Артефакт результата:

- compact task bundle с первым defendable reduction target.

### Slice 2. Схлопнуть duplicated direct-upload lifecycle

Цель:

- вынести общее `prepare/client/repository`, abort guard и sealing completion
  pieces из raw/described transport paths без появления новой бессмысленной
  abstraction layer.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/direct_upload_transport.go`
- `images/controller/internal/adapters/modelpack/oci/direct_upload_transport_raw.go`
- `images/controller/internal/adapters/modelpack/oci/direct_upload_transport_raw_flow.go`
- при необходимости новый helper file в том же package

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`

Артефакт результата:

- direct-upload flow короче и читается как один общий lifecycle с
  raw/described-specific branch logic только там, где semantics реально
  различаются.

### Slice 3. Выбрать continuation slice по read-only audit

Цель:

- не гадать следующий target по LOC вслепую;
- зафиксировать, почему следующий reduction шаг идёт в конкретную boundary.

Файлы/каталоги:

- `plans/active/repo-code-reduction-audit/*`

Проверки:

- manual consistency review

Артефакт результата:

- notes с read-only findings и выбранный следующий bounded slice.

### Slice 4. Сжать `uploadsessionstate` codec/mutator cluster

Цель:

- убрать duplicated mutation code;
- свернуть unnecessary file split внутри одного secret-backed codec/mutator
  cluster без смены package contract.

Файлы/каталоги:

- `images/controller/internal/adapters/k8s/uploadsessionstate/secret.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/secret_parse.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/multipart_state.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/phase_mutation.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/token_storage.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/client.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/secret_test.go`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession`

Артефакт результата:

- один compact secret codec/mutator unit плюс более тонкий `Client`, без drift
  в secret schema и phase semantics.

### Slice 5. Убрать duplicate archive helpers

Цель:

- вынести общий archive path / tar reader / extract helpers из `sourcefetch` и
  `modelpack/oci` в один support package;
- удалить локальные копии без изменения security policy и archive selection.

Файлы/каталоги:

- `images/controller/internal/support/archiveio/*`
- `images/controller/internal/adapters/sourcefetch/archive_extract.go`
- `images/controller/internal/adapters/sourcefetch/tar_gzip.go`
- `images/controller/internal/adapters/sourcefetch/*archive*`
- `images/controller/internal/adapters/modelpack/oci/materialize_support.go`
- `images/controller/internal/adapters/modelpack/oci/materialize_layers.go`
- `images/controller/internal/adapters/modelpack/oci/publish_archive_source*.go`
- `images/controller/internal/adapters/modelpack/oci/publish_object_source*.go`

Проверки:

- `cd images/controller && go test ./internal/support/archiveio ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci`

Артефакт результата:

- один shared helper для archive filesystem semantics и меньше duplicate code в
  adapter packages.

### Slice 6. Сжать DMCR direct-upload HTTP tests

Цель:

- убрать repeated `NewService` / `httptest.Server` / start-token /
  complete-request boilerplate из одного крупного test file;
- оставить сценарии проверки direct upload behavior без изменения production
  code.

Файлы/каталоги:

- `images/dmcr/internal/directupload/service_test.go`

Проверки:

- `cd images/dmcr && go test ./internal/directupload`

Артефакт результата:

- один same-package test harness и меньше повторяющегося setup в HTTP tests.

### Slice 7. Repo-level validation

Цель:

- подтвердить, что reduction не сломал остальной repo guardrail.

Файлы/каталоги:

- все затронутые surfaces этого bundle

Проверки:

- `make verify`
- `git diff --check`

Артефакт результата:

- reduction slice safe to keep.

### Slice 8. Сжать upload archive publication flow внутри `publishworker`

Цель:

- убрать duplicated local/staged archive publication workflow;
- оставить helper package-local, потому что это publication policy, а не
  generic archive IO;
- сохранить streaming byte path и staged cleanup timing.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/upload.go`
- `images/controller/internal/dataplane/publishworker/upload_streaming.go`
- `images/controller/internal/dataplane/publishworker/upload_stage_archive_streaming.go`
- при необходимости новый package-local helper file в
  `images/controller/internal/dataplane/publishworker/`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci`

Артефакт результата:

- один package-local archive upload publish workflow;
- archive suffix classification uses shared archive primitive checks;
- local and staged archive paths keep distinct IO boundaries.

### Slice 9. Подготовить следующий крупный controller-local service collapse

Цель:

- зафиксировать будущий крупный target без реализации в этом diff;
- не смешивать `publication_control_plane` collapse с текущим byte-path helper
  cleanup.

Файлы/каталоги:

- `plans/active/repo-code-reduction-audit/*`

Проверки:

- manual consistency review

Артефакт результата:

- notes/plan явно называют следующий target:
  `controllers/catalogstatus` + `application/publishplan` +
  `application/publishobserve` + `domain/publishstate` + `ports/publishop`.

### Slice 10. Убрать fake DTO в publication control-plane

Цель:

- удалить неиспользуемые internal operation DTO;
- убрать пустой `UploadSessionPlan`, который не был реальным application
  output contract;
- оставить upload-session lifecycle validation-only без изменения runtime
  semantics.

Файлы/каталоги:

- `images/controller/internal/ports/publishop/operation_contract.go`
- `images/controller/internal/application/publishplan/issue_upload_session.go`
- `images/controller/internal/application/publishplan/issue_upload_session_test.go`
- `images/controller/internal/adapters/k8s/uploadsession/service.go`
- `images/controller/internal/adapters/k8s/uploadsession/lifecycle.go`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/ports/publishop ./internal/domain/ingestadmission`
- `cd images/controller && go test ./internal/controllers/catalogcleanup ./internal/application/deletion ./internal/adapters/k8s/directuploadstate`

Артефакт результата:

- `publishop.Phase` / `publishop.Status` удалены как unused internal DTO;
- `IssueUploadSession` возвращает только validation error;
- upload-session adapter больше не прокидывает фиктивный plan object.

### Slice 11. Закрыть file-size violation в controller config

Цель:

- привести `images/controller/cmd/ai-models-controller/config.go` к `<350`
  physical LOC;
- не менять env/flag parsing и bootstrap option semantics.

Файлы/каталоги:

- `images/controller/cmd/ai-models-controller/config.go`
- `images/controller/cmd/ai-models-controller/resources.go`

Проверки:

- `cd images/controller && go test ./cmd/ai-models-controller`
- physical file-size scan по `images/controller/**/*.go`

Артефакт результата:

- `config.go` уменьшен до `334` строк;
- в `images/controller` не осталось Go-файлов `>=350` physical LOC.

### Slice 12. Сжать HuggingFace sourcefetch test fixtures

Цель:

- убрать repeated global stub/baseURL restore boilerplate в HuggingFace tests;
- вынести только boring same-package test fixtures, не скрывая сценарии
  handlers/assertions;
- не менять production HuggingFace fetch/mirror byte path.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*huggingface*_test.go`
- новый same-package test helper при необходимости в
  `images/controller/internal/adapters/sourcefetch/`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch`

Артефакт результата:

- повторяющиеся `huggingFaceBaseURL` / `fetchHuggingFaceInfoFunc` /
  `fetchHuggingFaceProfileSummaryFunc` restore blocks заменены локальными
  test helpers;
- production sourcefetch code не затронут.

### Slice 13. Разрезать DMCR direct-upload service tests

Цель:

- убрать `service_test.go` file-size violation без изменения сценариев;
- разрезать tests по decision surfaces: auth/start, default completion policy,
  verification helpers, recovery/cleanup и shared support;
- не менять production `directupload` code и protocol semantics.

Файлы/каталоги:

- `images/dmcr/internal/directupload/service_test.go`
- новые same-package `*_test.go` в `images/dmcr/internal/directupload/`

Проверки:

- `cd images/dmcr && go test ./internal/directupload`

Артефакт результата:

- один крупный `service_test.go` заменён на несколько focused test files;
- test harness остаётся same-package и не становится скрытым production layer.

### Slice 14. Разрезать DMCR direct-upload service по file boundary

Цель:

- убрать `service.go` production file-size violation;
- оставить direct-upload control flow same-package и линейным;
- не вводить новые interfaces/coordinators и не переносить HTTP DTO в
  public/backend contract.

Файлы/каталоги:

- `images/dmcr/internal/directupload/service.go`
- `images/dmcr/internal/directupload/completion.go`
- новые same-package production files в `images/dmcr/internal/directupload/`

Проверки:

- `cd images/dmcr && go test ./internal/directupload`

Артефакт результата:

- HTTP handlers, completion orchestration, sealed-object persistence и server
  wrapper разнесены по focused files;
- `handleComplete` step order, logging, verification, dedupe/link/cleanup
  behavior не меняются.

### Slice 15. Разрезать DMCR garbagecollection oversized tests

Цель:

- убрать крупные `directupload_inventory_test.go` и `runner_test.go`
  file-size violations;
- вынести fake prefix store/dynamic client и log assertion helpers в
  same-package support test files;
- не менять garbagecollection production behavior.

Файлы/каталоги:

- `images/dmcr/internal/garbagecollection/directupload_inventory_test.go`
- `images/dmcr/internal/garbagecollection/runner_test.go`
- новый same-package support test file в
  `images/dmcr/internal/garbagecollection/`

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`

Артефакт результата:

- сценарный direct-upload inventory test file становится `<350` LOC;
- `runner_test.go` становится `<350` LOC;
- fake store helpers не смешаны с test cases.

### Slice 16. Разрезать hooks sync-artifacts-secrets tests

Цель:

- убрать `main_test.go` file-size violation;
- вынести snapshot/value assertion helpers в same-package support test file;
- не менять hook behavior.

Файлы/каталоги:

- `images/hooks/pkg/hooks/sync_artifacts_secrets/main_test.go`
- новый same-package support test file в
  `images/hooks/pkg/hooks/sync_artifacts_secrets/`

Проверки:

- `cd images/hooks && go test ./pkg/hooks/sync_artifacts_secrets`

Артефакт результата:

- hand-written hooks tests укладываются в `<350` LOC.

### Slice 17. Удалить fake upload-session application hop

Цель:

- убрать `publishplan.IssueUploadSession` / `UploadSessionIssueRequest`, потому
  что это не самостоятельный use case, а rewrap существующих validation rules;
- оставить source-shape validation в `publishop.Request.Validate`;
- оставить owner/identity/upload admission в `ingestadmission`.

Файлы/каталоги:

- `images/controller/internal/application/publishplan/issue_upload_session.go`
- `images/controller/internal/application/publishplan/issue_upload_session_test.go`
- `images/controller/internal/adapters/k8s/uploadsession/lifecycle.go`
- `images/controller/internal/application/publishplan/*_test.go`
- `images/controller/internal/adapters/k8s/uploadsession/*_test.go`
- `images/controller/internal/ports/publishop/*_test.go`

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/ports/publishop ./internal/domain/ingestadmission`

Артефакт результата:

- upload-session adapter валидирует `publicationports.Request` и вызывает
  `ingestadmission.ValidateUploadSession` напрямую;
- package `publishplan` больше не содержит upload-session DTO без output
  contract;
- public status/API, secret schema и runtime byte path не меняются.

### Slice 18. Сжать DMCR GC request secret mutation

Цель:

- убрать duplicate labels/annotations/data shaping между create и refresh path;
- оставить helper package-local в `controllers/catalogcleanup`;
- сохранить controller-to-DMCR wire keys как явный internal seam, не shared API.

Файлы/каталоги:

- `images/controller/internal/controllers/catalogcleanup/gc_request.go`
- `images/controller/internal/controllers/catalogcleanup/gc_request_test.go`

Проверки:

- `cd images/controller && go test ./internal/controllers/catalogcleanup`
- `cd images/controller && go test ./internal/application/deletion ./internal/controllers/catalogcleanup ./internal/adapters/k8s/directuploadstate`

Артефакт результата:

- `buildDMCRGCRequestSecret` и `ensureGarbageCollectionRequest` используют
  один local mutator для labels/annotations/token payload;
- `requested-at` и `switch` по-прежнему stamp-ятся одним timestamp;
- `done` удаляется при refresh, пустой token убирает только direct-upload token
  payload.

### Slice 19. Сжать `modelpack/oci` generated archive layer plumbing

Цель:

- убрать duplicated descriptor/range-stream plumbing для synthetic tar archive
  layers;
- оставить helper same-package в `modelpack/oci`;
- не менять raw layer path, direct ranged object-source path, artifact format,
  `DMCR` protocol или runtime topology.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/publish_archive_source.go`
- `images/controller/internal/adapters/modelpack/oci/publish_object_source.go`
- `images/controller/internal/adapters/modelpack/oci/publish_layers_describe.go`
- `images/controller/internal/adapters/modelpack/oci/publish_layers_stream.go`
- новый same-package helper file при необходимости в
  `images/controller/internal/adapters/modelpack/oci/`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/adapters/sourcefetch ./internal/support/archiveio`

Артефакт результата:

- generated archive descriptors используют один helper для digest/diffID/size;
- generated archive range streams используют один helper для pipe, compression,
  offset/length slicing и close/error ordering;
- `archiveWriter.Close()` и `CloseWithError(writeErr)` semantics сохранены.

### Slice 20. Схлопнуть controller-local publication control-plane hop

Цель:

- удалить `application/publishplan` как отдельный application boundary без
  самостоятельного ownership;
- оставить public status mutation в `application/publishobserve`;
- перенести source-worker planning в существующий K8s sourceworker adapter,
  потому что это concrete runtime object shaping;
- оставить execution-mode decision рядом с catalog status orchestration.

Файлы/каталоги:

- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/application/publishobserve/ensure_runtime.go`
- `images/controller/internal/application/publishobserve/reconcile_gate.go`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`

Проверки:

- `cd images/controller && go test ./internal/controllers/catalogstatus ./internal/application/publishobserve ./internal/adapters/k8s/sourceworker ./internal/ports/publishop`
- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate`

Артефакт результата:

- package `application/publishplan` удалён;
- source-worker plan types/functions живут в sourceworker adapter package;
- `catalogstatus` больше не прокидывает execution mode через отдельный
  planning package;
- status mutation/status observation code не переехал в controller boundary.

## 4. Rollback point

После Slice 3: если continuation slice требует cross-boundary merge или даёт
только cosmetic file binpack без реального упрощения mutation paths,
останавливаемся без code edits.

## 5. Final validation

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/adapters/k8s/uploadsessionstate ./internal/adapters/k8s/uploadsession`
- `cd images/controller && go test ./internal/support/archiveio ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/dataplane/publishworker ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/adapters/k8s/uploadsession ./internal/adapters/k8s/uploadsessionstate ./internal/ports/publishop`
- `cd images/controller && go test ./cmd/ai-models-controller`
- `cd images/controller && go test ./internal/adapters/sourcefetch`
- `cd images/controller && go test ./internal/controllers/catalogstatus ./internal/application/publishobserve ./internal/adapters/k8s/sourceworker ./internal/ports/publishop`
- `cd images/dmcr && go test ./internal/directupload`
- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/hooks && go test ./pkg/hooks/sync_artifacts_secrets`
- `make verify`
- `git diff --check`
