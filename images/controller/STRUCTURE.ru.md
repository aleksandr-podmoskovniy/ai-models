# Структура `images/controller`

Этот документ отвечает на три вопроса по каждому каталогу и файлу:

1. зачем он существует;
2. почему он лежит именно здесь;
3. почему его ответственность не должна быть в соседнем слое.

Если файл нельзя убедительно защитить по этой схеме, он должен быть удалён,
слит с соседним или перенесён.

## 1. Корень `images/controller/`

### [README.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/README.md)

- Назначение: короткий canonical overview runtime tree.
- Почему здесь: это entrypoint-документ именно для controller image root.
- Почему не в `docs/`: он описывает локальный runtime tree, а не весь модуль.

### [STRUCTURE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/STRUCTURE.ru.md)

- Назначение: файл-инвентарь и аргументация по структуре.
- Почему здесь: это локальная карта controller runtime, а не общая repo doc.
- Почему не в bundle: bundle меняется по slice’ам, а структура tree должна
  иметь устойчивый source of truth рядом с кодом.

### [TEST_EVIDENCE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/TEST_EVIDENCE.ru.md)

- Назначение: единый inventory decision-coverage для domain/application пакетов.
- Почему здесь: evidence относится ко всему controller tree, а не к одному
  package.
- Почему не как `BRANCH_MATRIX.ru.md` в каждом пакете: локальные matrix-файлы
  создавали лоскутное правило “где-то есть, где-то нет”. Один controller-level
  документ проще, однозначнее и легче проверяется в `verify`.

### [go.mod](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/go.mod)

- Назначение: отдельный module boundary для controller runtime.
- Почему здесь: executable controller code живёт целиком в этом image root.
- Почему не на repo root: репозиторий остаётся DKP module root, а не единый Go
  monorepo-operator.

### [go.sum](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/go.sum)

- Назначение: dependency lock для controller Go module.
- Почему здесь: следует за `go.mod`.
- Почему не общий: backend, hooks и API имеют свои отдельные dependency
  boundaries.

### [werf.inc.yaml](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/werf.inc.yaml)

- Назначение: image build shell для controller runtime.
- Почему здесь: image wiring должен жить рядом с исходниками image.
- Почему не в `templates/`: build shell и Kubernetes manifests не должны
  смешиваться.

### [kitops.lock](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/kitops.lock)

- Назначение: pinned metadata для phase-2 runtime-owned `KitOps` binary.
- Почему здесь: phase-2 `ModelPack` implementation теперь принадлежит
  dedicated runtime image рядом с controller tree.
- Почему не в `images/backend/`: phase-2 publication/upload/cleanup runtime
  больше не должен жить в backend image.

### [tools/install-kitops.sh](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/tools/install-kitops.sh)

- Назначение: build-time installer pinned `KitOps` CLI for phase-2 runtime
  image.
- Почему здесь: tool installation shell допустим как build-time glue рядом с
  controller tree.
- Почему не в backend scripts: текущий phase-2 execution path больше не
  backend-owned.

## 2. `cmd/`

`cmd/` должен оставаться thin executable shell.

### [cmd/ai-models-controller/main.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-controller/main.go)

- Назначение: минимальный process entrypoint.
- Почему здесь: это стандартный Go `cmd` main package.
- Почему не в `internal/bootstrap`: `main` не является reusable bootstrap logic.

### [cmd/ai-models-controller/run.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-controller/run.go)

- Назначение: CLI/env parsing и wiring runtime options.
- Почему здесь: это outer shell вокруг `internal/bootstrap`.
- Почему не в `internal/bootstrap`: bootstrap должен принимать уже нормализованные options,
  а не заниматься argv/env parsing.

### `cmd/ai-models-artifact-runtime/`

- Назначение: отдельный thin executable shell для one-shot phase-2 runtime.
- Почему здесь: manager и data-plane execution больше не должны притворяться
  одним бинарём.
- Почему не в `internal/dataplane`: dataplane packages не должны знать про
  argv/env parsing и process entrypoints.

#### [cmd/ai-models-artifact-runtime/main.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-artifact-runtime/main.go)

- Назначение: минимальный entrypoint для runtime binary.

#### [cmd/ai-models-artifact-runtime/dispatch.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-artifact-runtime/dispatch.go)

- Назначение: explicit dispatch between `publish-worker`, `upload-session` and
  `artifact-cleanup`.
- Почему здесь: routing one-shot commands — outer shell responsibility.
- Почему не в `internal/bootstrap`: manager bootstrap не должен знать про
  phase-2 runtime entrypoints.

#### [cmd/ai-models-artifact-runtime/common.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-artifact-runtime/common.go)

- Назначение: локальные command constants для runtime binary.
- Почему отдельно: manager binary больше не должен держать phase-2 one-shot
  command names.

#### [cmd/ai-models-artifact-runtime/publish_worker.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-artifact-runtime/publish_worker.go)

- Назначение: thin CLI shell for controller-owned publish worker runtime.
- Почему здесь: one-shot data-plane execution belongs to dedicated runtime
  binary entrypoint layer.
- Почему не в `sourceworker`: `sourceworker` строит Pod и runtime contract, а
  не исполняет process entrypoint.

#### [cmd/ai-models-artifact-runtime/upload_session.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-artifact-runtime/upload_session.go)

- Назначение: thin CLI shell for controller-owned upload session HTTP runtime.
- Почему здесь: это executable process boundary отдельного runtime binary.
- Почему не в `uploadsession`: adapter materializes K8s resources; HTTP server
  runtime lives in dataplane/cmd.

#### [cmd/ai-models-artifact-runtime/artifact_cleanup.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/cmd/ai-models-artifact-runtime/artifact_cleanup.go)

- Назначение: thin CLI shell for controller-owned artifact cleanup runtime.
- Почему здесь: cleanup execution is a process entrypoint dedicated runtime
  binary, not reconciler policy.
- Почему не в `catalogcleanup`: controller decides *when* to run cleanup, but
  actual artifact removal is dataplane runtime code.

## 3. `internal/cmdsupport/`

### [common.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/cmdsupport/common.go)

- Назначение: shared env/flags/signal/logger/termination helpers for manager
  and runtime binaries.
- Почему здесь: это реально reusable executable glue, не привязанный только к
  одному `cmd` package.
- Почему не в `support/`: helper не нужен controller domain/adapters и не
  должен выглядеть как ещё один business-support layer.

## 4. `internal/bootstrap/`

`internal/bootstrap` — bootstrap layer. Он собирает manager и controllers, но
не содержит доменной логики publication/delete.

- Почему не `internal/app`: рядом уже есть `internal/application`, и пара
  `app/application` создаёт ложную двусмысленность в гексагональном дереве.
  Composition root должен называться по ответственности, а не по общему слову.

### [internal/bootstrap/bootstrap.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/bootstrap/bootstrap.go)

- Назначение: composition root controller runtime.
- Почему здесь: это application bootstrap, не adapter и не domain.
- Почему не в `cmd/`: `cmd` должен оставаться тонким и не держать wiring дерева.

### [internal/bootstrap/bootstrap_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/bootstrap/bootstrap_test.go)

- Назначение: проверка bootstrap contract.
- Почему здесь: тестирует именно `internal/bootstrap`.
- Почему не в controller packages: это не lifecycle behavior, а runtime wiring.

## 5. `internal/domain/publishstate/`

Это чистый domain слой publication lifecycle. Здесь нет Kubernetes API CRUD,
нет `client.Client`, нет `ConfigMap` serialization.

- Почему не `domain/publication`: generic имя `publication` уже использовалось
  в нескольких соседних слоях и делало tree двусмысленным. Здесь хранится
  именно lifecycle/state semantics, поэтому пакет должен называться
  `publishstate`.

### [internal/domain/publishstate/operation.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/operation.go)

- Назначение: publication phases и terminal semantics.
- Почему здесь: это domain vocabulary.
- Почему не в adapters: concrete adapters не должны определять lifecycle terms.

### [internal/domain/publishstate/status.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/status.go)

- Назначение: projection rules `Observation -> ModelStatus`.
- Почему здесь: это чистое доменное решение.
- Почему не в `controllers/catalogstatus`: reconciler только читает и пишет,
  но не должен владеть rule table.

### [internal/domain/publishstate/conditions.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/conditions.go)

- Назначение: canonical condition/status assembly.
- Почему здесь: условия — часть domain semantics каталога.
- Почему не в API package: это runtime decision logic, а не API type shape.

### [internal/domain/publishstate/runtime_decisions.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/runtime_decisions.go)

- Назначение: интерпретация worker/session observation.
- Почему здесь: это domain decision table.
- Почему не в controller adapter: adapter не должен решать бизнес-исход.

### Tests

#### [operation_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/operation_test.go)

- Назначение: проверяет phase vocabulary и terminal/equality semantics.

#### [status_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/status_test.go)

- Назначение: проверяет `Observation -> public status` projection и ready/fail
  branches.

#### [runtime_decisions_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/domain/publishstate/runtime_decisions_test.go)

- Назначение: проверяет decision tables для source-worker и upload-session
  observations.

Decision evidence для пакета централизована в
[TEST_EVIDENCE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/TEST_EVIDENCE.ru.md),
а не в локальном `BRANCH_MATRIX.ru.md`.

## 6. `internal/application/`

`application` — use-case слой. Он связывает domain и input contract, но не
строит Kubernetes resources.

### `internal/application/publishplan/`

- Почему не `application/publication`: в application слое здесь не “вся
  publication”, а именно planning/selecting use cases. Generic имя повторяло
  домен и порты без добавления смысла.

#### [start_publication.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/publishplan/start_publication.go)

- Назначение: выбрать execution mode для source.
- Почему здесь: это orchestration use case между spec и runtime path.
- Почему не в domain: зависит от текущего implementation path.

#### [plan_source_worker.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/publishplan/plan_source_worker.go)

- Назначение: нормализовать `HF`/`HTTP` в worker plan.
- Почему здесь: это use-case planning, а не Kubernetes Pod rendering.
- Почему не в `sourceworker`: adapter должен принимать уже спланированный input.

#### [issue_upload_session.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/publishplan/issue_upload_session.go)

- Назначение: валидация и planning upload session.
- Почему здесь: это use-case policy.
- Почему не в `uploadsession`: adapter лишь materialize’ит session resources.

#### Tests

#### [start_publication_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/publishplan/start_publication_test.go)

- Назначение: проверяет выбор execution mode и fail-closed behavior для source
  kinds.

#### [plan_source_worker_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/publishplan/plan_source_worker_test.go)

- Назначение: проверяет source worker plan normalization, auth secret mapping и
  guarded failures.

#### [issue_upload_session_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/publishplan/issue_upload_session_test.go)

- Назначение: проверяет upload session issuance policy и rejection branches.

Назначение блока: decision tests use-case слоя.
Decision evidence для пакета централизована в
[TEST_EVIDENCE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/TEST_EVIDENCE.ru.md),
а не в локальном `BRANCH_MATRIX.ru.md`.

### `internal/application/deletion/`

#### [ensure_cleanup_finalizer.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/deletion/ensure_cleanup_finalizer.go)

- Назначение: решить, нужен ли finalizer.
- Почему здесь: это use-case policy, не Kubernetes CRUD.

#### [finalize_delete.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/deletion/finalize_delete.go)

- Назначение: decision table delete flow по observation cleanup job.
- Почему здесь: это delete orchestration policy.
- Почему не в controller adapter: reconciler только исполняет это решение.

#### Tests

#### [ensure_cleanup_finalizer_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/deletion/ensure_cleanup_finalizer_test.go)

- Назначение: проверяет правила постановки и пропуска finalizer.

#### [finalize_delete_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/application/deletion/finalize_delete_test.go)

- Назначение: проверяет delete decision table по cleanup observation и failure
  branches.

Decision evidence для пакета централизована в
[TEST_EVIDENCE.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/TEST_EVIDENCE.ru.md),
а не в локальном `BRANCH_MATRIX.ru.md`.

## 7. `internal/ports/publishop/`

Порты — shared boundary между use cases/domain и concrete adapters.

### [operation_contract.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/operation_contract.go)

- Назначение: shared operation contract primitives.
- Почему здесь: это reusable port contract.
- Почему не в controller adapter: adapter не должен владеть shared contract.
- Почему не `ports/publication`: пакет содержит не “всю publication”, а
  operation/runtime contract для controller-owned execution boundary.

### [ports.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/ports.go)

- Назначение: shared runtime interfaces и worker/session handles.
- Почему здесь: это reusable seam between controller and concrete adapters.
- Почему не в domain: это infrastructural contracts, а не business vocabulary.
- Почему source worker и upload session теперь оба идут через `GetOrCreate`:
  concrete runtime adapters не должны расходиться по surface area без реальной
  поведенческой причины; раньше отдельный `Get` у `sourceworker` только держал
  лишнюю асимметрию и раздувал reconcile flow.
- Почему handles несут `TerminationMessage`: worker/session завершаются
  one-shot pod’ом, а controller читает результат прямо из termination log без
  промежуточного отдельного хранилища состояния.

### Tests

#### [operation_contract_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/operation_contract_test.go)

- Назначение: проверяет operation contract primitives и ownership helpers.

#### [ports_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/ports_test.go)

- Назначение: проверяет runtime-port shapes и handle contracts.

## 8. `internal/ports/modelpack/`

### [contract.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/modelpack/contract.go)

- Назначение: replaceable `ModelPack` publication/removal contract.
- Почему здесь: выбор concrete implementation (`KitOps`, `Modctl`, native impl)
  должен оставаться отдельным портом.
- Почему не в `publishop`: publish execution contract и `ModelPack`
  implementation contract — разные boundaries.

## 9. `internal/publishedsnapshot/`

`publishedsnapshot` — immutable handoff model между publish, status и cleanup.

### [snapshot.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/publishedsnapshot/snapshot.go)

- Назначение: canonical published snapshot model.
- Почему здесь: это cross-cutting controller handoff model.
- Почему не в API: это internal runtime shape, не public CRD schema.
- Почему не просто `internal/publication`: generic имя конфликтовало с
  `application/`, `domain/` и `ports/` слоями; здесь хранится именно snapshot
  published artifact/result.

### [snapshot_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/publishedsnapshot/snapshot_test.go)

- Назначение: validates snapshot contract.

## 10. `internal/artifactbackend/`

Это boundary к backend artifact plane.

### [contract.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/artifactbackend/contract.go)

- Назначение: input/output contract backend-side publication worker result.
- Почему здесь: backend integration contract должен быть отдельным от K8s adapter.
- Почему не в `publication`: snapshot — domain handoff, backend contract —
  transport boundary к внешнему исполнителю.

### [location.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/artifactbackend/location.go)

- Назначение: canonical OCI artifact reference builder.
- Почему здесь: path construction относится к backend artifact plane.
- Почему не в `ociregistry`: registry env/secret rendering и artifact naming —
  разные ответственности.

### Tests

- [contract_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/artifactbackend/contract_test.go)
- [location_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/artifactbackend/location_test.go)

## 11. `internal/adapters/sourcefetch/`

### [archive.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/archive.go)

- Назначение: safe payload preparation entrypoint for fetched/uploaded model
  inputs.
- Важная деталь: умеет либо распаковывать архив, либо материализовать один
  файл; это нужно для прямого `GGUF` через `HTTP` и `Upload`.
- Почему здесь: это concrete source acquisition adapter concern.
- Почему не в `dataplane/*`: extract/materialize policy переиспользуется
  несколькими runtime paths.

### [tar_gzip.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/tar_gzip.go)

- Назначение: hardened tar/gzip extraction helpers.
- Почему здесь: tar safety — деталь source-fetch adapter implementation.
- Почему не в `support/`: логика не shared вне source acquisition boundary.

### [http.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/http.go)

- Назначение: generic HTTP model payload download with auth and custom CA
  support.
- Почему здесь: concrete adapter for generic remote URL download after source
  type is resolved from the URL.
- Почему не в `publishworker`: worker orchestrates runtime, sourcefetch acquires
  bytes and normalizes them.

### [remote.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/remote.go)

- Назначение: один canonical remote ingest entrypoint для `HuggingFace` и
  generic `HTTP`: скачать, определить входной формат, подготовить локальную
  директорию модели и вернуть нормализованный результат.
- Почему здесь: это всё ещё одна concrete source-acquisition boundary, а не
  responsibility `publishworker`.
- Почему не в `http.go` или `huggingface.go`: provider-specific transport и
  provider-agnostic orchestration больше не должны быть перемешаны.

### [transport.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/transport.go)

- Назначение: общий minimal HTTP transport shell для remote source adapters:
  GET-запрос, разбор ошибки ответа, JSON decode и запись тела в файл.
- Почему отдельно: раньше `http.go` и `huggingface.go` держали один и тот же
  сетевой и file-write boilerplate; это был повтор внутри одной границы.
- Почему не в `support/`: helper остаётся специфичным для source acquisition
  adapters и не нужен всему controller tree.

### [huggingface.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/huggingface.go)

- Назначение: Hugging Face metadata lookup and filtered snapshot download via
  HTTP API.
- Почему здесь: concrete adapter for upstream source acquisition.
- Почему не в domain/application: network IO и upstream response parsing не
  являются use-case policy.

#### Tests

- [archive_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/archive_test.go)
- [http_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/http_test.go)
- [huggingface_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/huggingface_test.go)
- [remote_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/sourcefetch/remote_test.go)

Назначение блока: проверяет safe fetch/extract/materialize policy без
Kubernetes shell.

## 12. `internal/adapters/modelformat/`

### [validation.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelformat/validation.go)

- Назначение: source-agnostic validation and sanitization rules for
  `spec.inputFormat`.
- Почему здесь: это filesystem inspection adapter, общий для `HuggingFace`,
  `HTTP` и `Upload`.
- Почему не в `application/publishplan`: use case выбирает execution mode, но
  не должен владеть file allowlist/rejectlist semantics.
- Почему не в `publishworker`: dataplane runtime orchestrates fetch/unpack/push,
  а не владеет списками допустимых файлов и security policy.

### [detect.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelformat/detect.go)

- Назначение: автоматическое определение `spec.inputFormat`, когда пользователь
  его не указал явно.
- Почему здесь: это тот же adapter boundary над составом файлов модели, а не
  use-case decision layer.
- Почему не в `application/publishplan`: планировщик выбирает execution mode,
  а не распознаёт формат по файлам или списку remote files.

### [validation_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelformat/validation_test.go)

- Назначение: verifies required-file enforcement, benign extra stripping and
  forbidden file rejection for `Safetensors` and `GGUF`.

## 13. `internal/adapters/modelprofile/`

### `internal/adapters/modelprofile/common/`

#### [profile.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelprofile/common/profile.go)

- Назначение: общий расчётный слой для profile adapters:
  endpoint types по `task`, уникализация списков, оценка bytes-per-parameter и
  минимального GPU launch.
- Почему здесь: это общий math/normalization слой для нескольких format
  adapters, но не доменная логика и не controller runtime.

#### [profile_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelprofile/common/profile_test.go)

- Назначение: проверяет общую математику endpoint mapping и launch sizing.

### `internal/adapters/modelprofile/safetensors/`

#### [profile.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelprofile/safetensors/profile.go)

- Назначение: ai-inference-oriented metadata extraction from validated
  `Safetensors` model directories.
- Важная деталь: использует не только `config.json`, но и реальные размеры
  `.safetensors` shard files для более жёсткой оценки `parameterCount` и
  `minimumLaunch`.
- Почему здесь: metamodel calculation depends on concrete files on disk, но
  должна жить отдельно от source-fetch, `ModelPack` publishing и status
  projection.
- Почему не в domain: это technical inspection adapter over files on disk.

#### [profile_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelprofile/safetensors/profile_test.go)

- Назначение: verifies parameter/precision/runtime/launch heuristics for the
  `Safetensors` path.

### `internal/adapters/modelprofile/gguf/`

#### [profile.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelprofile/gguf/profile.go)

- Назначение: ai-inference-oriented metadata extraction from validated
  `GGUF` payloads.
- Важная деталь: использует имя и размер `.gguf` файла, чтобы выделять family,
  quantization, приблизительный `parameterCount` и GPU baseline
  `minimumLaunch`.
- Почему здесь: это отдельный concrete file-format adapter с собственными
  runtime/precision heuristics.
- Почему не в `safetensors`: это другой format boundary и другой heuristic set.

#### [profile_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelprofile/gguf/profile_test.go)

- Назначение: verifies `GGUF` family, quantization and runtime heuristics.

## 14. `internal/adapters/modelpack/kitops/`

### [adapter.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelpack/kitops/adapter.go)

- Назначение: concrete `ModelPack` adapter over pinned `KitOps` CLI.
- Почему здесь: tool-specific implementation belongs to an adapter package
  behind `ports/modelpack`.
- Почему не в dataplane runtime: publishworker depends on the `ModelPack` port,
  а не на конкретный tool brand.

### [adapter_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/modelpack/kitops/adapter_test.go)

- Назначение: verifies command assembly, immutable OCI reference logic and
  inspect payload parsing for the adapter.

## 15. `internal/dataplane/`

`dataplane` — controller-owned one-shot runtime execution layer. Он не является
reconciler tree и не должен снова уезжать в backend Python scripts.

### `internal/dataplane/publishworker/`

#### [run.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/publishworker/run.go)

- Назначение: publication runtime for `HuggingFace`, `HTTP` and
  `Upload`.
- Почему здесь: это executable data-plane orchestration, не reconciler и не
  K8s supplement builder.
- Почему здесь нет file allowlist policy: content validation живёт отдельно в
  `adapters/modelformat`, чтобы один и тот же input-format contract применялся
  ко всем source paths.
- Почему не в `cmd/`: `cmd` парсит flags и вызывает dataplane.

#### [support.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/publishworker/support.go)

- Назначение: package-local runtime helpers for workspace lifecycle, result
  shaping and profile/sanitizer glue.
- Почему здесь: это boilerplate конкретно publication runtime package, но не
  main publish flow.
- Почему не в `support/`: логика не shared вне `publishworker` boundary.

#### [run_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/publishworker/run_test.go)

- Назначение: verifies publication runtime result shaping and fail-closed
  behavior around fetch/profile/modelpack adapters.

### `internal/dataplane/uploadsession/`

#### [run.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/uploadsession/run.go)

- Назначение: controller-owned upload HTTP server runtime.
- Важная деталь: сохраняет исходное имя файла из upload request, чтобы архивы и
  прямой `.gguf` не теряли формат при дальнейшей обработке.
- Почему здесь: это one-shot dataplane server, ближе к virtualization uploader
  pattern, чем к backend scripts.
- Почему не в `internal/adapters/k8s/uploadsession`: K8s adapter materializes
  Pod/Service/Secret, но не исполняет HTTP server.

#### [run_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/uploadsession/run_test.go)

- Назначение: verifies base runtime validation, health endpoint, auth guard and
  filename-preserving upload behavior for the upload session server.

### `internal/dataplane/artifactcleanup/`

#### [run.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/artifactcleanup/run.go)

- Назначение: controller-owned published-artifact cleanup runtime.
- Почему здесь: это one-shot execution layer for delete plane.
- Почему не в `catalogcleanup`: reconciler decides and launches; dataplane
  performs artifact removal.

#### [run_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/dataplane/artifactcleanup/run_test.go)

- Назначение: validates cleanup-handle decoding and remover invocation rules.

## 16. `internal/controllers/`

Здесь живут только concrete reconcilers и их тонкие observation/persistence
shell files.

### `internal/controllers/catalogstatus/`

#### [options.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/options.go)

- Назначение: только setup/runtime options и thin reconciler types.
- Почему здесь: options принадлежат concrete controller.
- Почему здесь нет `RequeueAfter`: polling cadence для status owner не является
  module/runtime contract. Это локальная lifecycle policy и она должна жить
  рядом с reconcile path, а не притворяться operator option.
- Важная деталь: controller подписывается на рабочие `Pod`-ы через map-функции,
  а не через `Owns`, потому что `Model` в namespace команды не может быть
  `ownerRef` для pod’а в `d8-ai-models`.

#### [watch.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/watch.go)

- Назначение: переводит события рабочих `Pod`-ов обратно в `Model` или
  `ClusterModel`.
- Почему здесь: это часть concrete controller wiring, а не shared helper.
- Почему не в `support/*`: mapping зависит от текущей publication topology и не
  является общим utility-кодом для всего controller tree.

#### [policy.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/policy.go)

- Назначение: локальные reconcile policy helpers:
  supported source types, ignore rules и skip rules.
- Почему здесь: это policy именно `catalogstatus`, но не domain и не shared
  support.
- Почему не в `reconciler.go`: thin reconciler gate требует держать локальные
  decision helpers рядом, но вне основного reconcile shell.

#### [reconciler.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/reconciler.go)

- Назначение: thin reconcile shell для `Model` / `ClusterModel` status owner.
- Почему здесь: это Kubernetes adapter.
- Почему не в application: содержит `client.Get`, requeue, adapter wiring.
- Что делает теперь: сам планирует и наблюдает `sourceworker` / `uploadsession`
  напрямую, без промежуточной service-state шины.

#### [io.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/io.go)

- Назначение: весь adapter-side IO для status controller:
  `termination-log -> domain Observation`, cleanup-handle persistence, status
  patch и failed/delete projection.
- Почему здесь: это единая concrete read/write boundary вокруг status owner.
- Почему не split на `observation.go` и `persistence.go`: отдельной
  самостоятельной границы там не было; это был искусственный micro-split.

#### Tests

#### [reconciler_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/reconciler_test.go)

- Назначение: проверяет public status projection для running/succeeded/failed
  worker paths и upload wait path без промежуточного persisted store.

#### [test_helpers_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/test_helpers_test.go)

- Назначение: держит adapter-local fake runtimes и termination-result builders.
- Почему отдельно: не размазывает этот шум по каждому test case, но и не
  создаёт второй shared fixture layer поверх `support/testkit`.

### `internal/controllers/catalogcleanup/`

#### [options.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/options.go)

- Назначение: controller config и reconciler shell types.

#### [reconciler.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/reconciler.go)

- Назначение: thin delete/finalizer controller shell.
- Почему здесь: это Kubernetes adapter поверх delete use cases.

#### [io.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/io.go)

- Назначение: весь adapter-side IO для cleanup controller:
  finalizer/job observation и apply finalizer/status/job side effects.
- Почему здесь: это одна concrete delete IO boundary.
- Почему не split на `observation.go` и `persistence.go`: это снова был
  искусственный micro-split без отдельного reusable seam.

#### [job.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/job.go)

- Назначение: controller-local build/inspect shell для cleanup `Job`.
- Почему здесь: cleanup `Job` существует только как деталь delete-flow внутри
  `catalogcleanup`; второго cleanup adapter нет, значит отдельный
  `adapters/k8s/cleanupjob` был ложной границей.
- Почему не в `internal/adapters/k8s`: там должны оставаться только реально
  reusable K8s adapters, а не одноразовые builder packages под один controller.

#### Tests

#### [job_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/job_test.go)

- Назначение: проверяет cleanup job rendering и label/owner wiring.

#### [reconciler_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/reconciler_test.go)

- Назначение: проверяет finalizer lifecycle, malformed handle и failed cleanup
  job branches.

#### [test_helpers_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogcleanup/test_helpers_test.go)

- Назначение: держит controller-local delete fixtures и cleanup job builders.

## 17. `internal/adapters/k8s/`

Это concrete reusable Kubernetes object/service builders и CRUD adapters.

### `internal/adapters/k8s/ociregistry/`

#### [render.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/ociregistry/render.go)

- Назначение: shared OCI registry env/volume rendering.
- Почему здесь: concrete K8s Pod spec fragments for OCI auth/CA.
- Почему не в `artifactbackend`: artifact naming и Pod env rendering — разные
  слои.

#### [render_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/ociregistry/render_test.go)

- Назначение: verify env/volume rendering contract.

### `internal/adapters/k8s/ownedresource/`

Это shared concrete K8s object IO helper для controlled resources.

#### [lifecycle.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/ownedresource/lifecycle.go)

- Назначение: canonical K8s owned-resource lifecycle helpers:
  safe owner wiring, `Create -> AlreadyExists -> Get` и
  `Delete(ignore-not-found)` для controlled objects.
- Почему здесь: это общий K8s adapter lifecycle shell, а не business logic и не
  `support/*` helper.
- Почему не в `sourceworker`/`uploadsession`: один и тот же controlled create
  и delete flow уже повторялся в нескольких adapter packages и не должен снова
  размножаться локально.

#### [lifecycle_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/ownedresource/lifecycle_test.go)

- Назначение: verify create-vs-reuse, owner reference wiring и shared delete
  behavior.

### `internal/adapters/k8s/workloadpod/`

Это shared concrete helper для workload `Pod` shell.

#### [render.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/workloadpod/render.go)

- Назначение: один canonical helper для workspace `EmptyDir` + `/tmp` mount и
  OCI registry CA volume/mount shell.
- Почему здесь: это concrete K8s workload rendering concern, но не business
  logic и не registry-specific policy.
- Почему не в `ociregistry`: `/tmp` workspace не относится к OCI auth/CA
  contract.
- Почему не в `sourceworker`/`uploadsession`: один и тот же Pod shell уже
  повторялся в нескольких adapters.

#### [render_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/workloadpod/render_test.go)

- Назначение: verify workspace+registry shell rendering order and append
  behavior.

### `internal/adapters/k8s/sourceworker/`

Это concrete worker Pod adapter для `HF` / `HTTP`.

#### [validation.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/validation.go)

- Назначение: validate concrete source-worker options и map shared
  `publishop.OperationContext` в application plan.
- Почему здесь: adapter должен валидировать свой own runtime input, но не
  дублировать shared request contract локальными `Request/OwnerRef` типами.

#### [build.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/build.go)

- Назначение: concrete Pod rendering.
- Почему не владеет именованием сам: canonical owner-based Pod/Secret naming
  вынесен в `support/resourcenames`, чтобы один и тот же policy не дублировался
  в нескольких adapters.
- Почему не open-code workspace/registry Pod shell: общий `/tmp` + registry CA
  volumes/mounts вынесен в `adapters/k8s/workloadpod`.

#### [auth_secret.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/auth_secret.go)

- Назначение: projected source-auth secret handling.
- Почему отдельно: secret projection — отдельная side effect зона от Pod build.
- Почему не держит ручной `Get/Create/Update`: projected secret теперь тоже
  проходит через один reconcile path с `CreateOrUpdate`, а не через
  adapter-local CRUD shell.

#### [service.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/service.go)

- Назначение: concrete source-worker adapter: shared runtime port plus internal
  Pod/auth-secret CRUD.
- Почему не держит свои `*NameFor()` helpers: resource naming — общий support
  concern, а не `sourceworker`-specific API.
- Почему без отдельного `runtime.go`, `NewRuntime` и отдельного `Get`: сам
  `Service` уже является concrete runtime adapter, а второй constructor path и
  лишний read-only runtime method только дублировали wiring и расходились по
  surface area с `uploadsession` без новой границы.
- Почему больше нет отдельного replay read path перед `CreateOrGet`: найденный
  Pod и созданный Pod всё равно проходят через один и тот же concrete adapter
  contract, поэтому отдельный предварительный `Get` только дублировал lifecycle
  shell без новой семантики.
- Почему handle construction не open-code дважды: local helper inside service
  keeps one concrete source-worker handle path вместо повторяющегося
  `NewSourceWorkerHandle(...)` для найденного и созданного Pod.
- Почему не open-code `SetControllerReference/Create/Get`: controlled create
  flow вынесен в `adapters/k8s/ownedresource`, чтобы такой K8s IO shell не
  дублировался между worker/session adapters.
- Почему delete path тоже не open-code: shared ignore-not-found delete shell
  теперь живёт там же, в `adapters/k8s/ownedresource`.

#### Tests

#### [validation_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/validation_test.go)

- Назначение: проверяет concrete input validation and guarded failures.

#### [build_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/build_test.go)

- Назначение: проверяет worker pod rendering, command/env and mounted
  supplements.

#### [auth_secret_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/auth_secret_test.go)

- Назначение: проверяет projected auth-secret material shape and key mapping.

#### [service_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/service_test.go)

- Назначение: проверяет service-level CRUD/reuse behavior and error mapping.

#### [service_roundtrip_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/service_roundtrip_test.go)

- Назначение: проверяет concrete adapter seam `GetOrCreate -> handle -> Delete`.
- Почему отдельно: это adapter smoke contract, а не “runtime layer test”.

#### [test_helpers_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/test_helpers_test.go)

- Назначение: держит один canonical `OperationContext` и options fixture для
  package-level adapter tests.
- Почему отдельно: убирает repeated inline request literals после отказа от
  local request types и не прячет business logic.

### `internal/adapters/k8s/uploadsession/`

Это concrete session supplements для `spec.source.upload`.

#### [options.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/options.go)

- Назначение: normalize/validate runtime options и хранить package-level
  constants upload-session supplements.

#### [resources.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/resources.go)

- Назначение: build secret/service.
- Почему не владеет naming сама: canonical naming policy для session
  resources вынесен в `support/resourcenames`, чтобы Pod/Service/Secret naming
  жил в одном месте.
- Почему не держит свой create-or-get shell: общий controlled resource create
  flow живёт в `adapters/k8s/ownedresource`, а здесь остаётся только concrete
  shape `Secret` / `Service`.

#### [pod.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/pod.go)

- Назначение: build upload pod.
- Почему не содержит свой naming layer: `uploadsession` использует тот же
  shared resource naming policy из `support/resourcenames`.
- Почему здесь же лежит mapping shared request в upload plan: после удаления
  `request.go` это остался один узкий helper ровно рядом с единственным live
  builder, а не отдельный file-level seam без собственной границы.
- Почему не open-code create-or-get: owned Pod create/reuse path такой же
  concrete K8s adapter shell, как и для session `Secret` / `Service`, поэтому
  он использует `adapters/k8s/ownedresource`.
- Почему не open-code workspace/registry Pod shell: общий `/tmp` + registry CA
  volumes/mounts вынесен в `adapters/k8s/workloadpod`.

#### [status.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/status.go)

- Назначение: derivation of user-facing upload status from ensured session
  resources.

#### [service.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/service.go)

- Назначение: concrete upload-session adapter: shared runtime port plus
  internal CRUD for session `Pod` / `Service` / `Secret`.
- Почему не содержит package-local name helpers: session CRUD использует общий
  owner-based naming support и не должен раздувать adapter-local API.
- Почему без отдельного `runtime.go`, `NewRuntime` и лишнего public `Get`:
  concrete runtime adapter здесь уже сам `Service`, а дополнительные proxy
  wrappers и read-only method без use site только плодили surface area без
  новой границы.
- Почему больше нет отдельного resource replay branch: существующие `Secret`,
  `Service` и `Pod` теперь проходят через тот же direct ensure/create-or-get
  цикл, что и новые resources, поэтому pre-read shell больше не нужен.
- Почему delete path не open-code: shared delete shell теперь живёт в
  `adapters/k8s/ownedresource`, чтобы Pod/Service/Secret cleanup не
  копировался по adapters.

#### Tests

#### [service_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/service_test.go)

- Назначение: проверяет CRUD/reuse behavior и error mapping для session
  supplements.

#### [replay_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/replay_test.go)

- Назначение: проверяет replay and recreate branches around existing session
  resources.

#### [status_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/status_test.go)

- Назначение: проверяет derived upload-status aggregation from created
  resources.

#### [service_roundtrip_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/service_roundtrip_test.go)

- Назначение: проверяет concrete session adapter seam
  `GetOrCreate -> derived upload status -> Delete`.
- Почему отдельно: это smoke on adapter contract, а не отдельный “runtime
  layer test”.

#### [test_helpers_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/test_helpers_test.go)

- Назначение: хранит canonical upload `OperationContext`, options fixture и
  tiny generic helpers вместо repeated session request literals.
- Почему отдельно: делает package tests поддерживаемыми после удаления local
  request wrappers и остаётся чисто test-only layer.

## 18. `internal/support/`

`support` допускается только для реально shared helpers без business policy.

### `internal/support/cleanuphandle/`

#### [handle.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/cleanuphandle/handle.go)

- Назначение: internal cleanup-handle encoding/decoding on model object.
- Почему здесь: shared helper between publication and cleanup controllers.
- Почему не в `catalogcleanup`: публикация тоже пишет этот handle.

#### [handle_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/cleanuphandle/handle_test.go)

- Назначение: round-trip and validation tests for the helper contract.

### `internal/support/modelobject/`

#### [modelobject.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/modelobject/modelobject.go)

- Назначение: shared `Model` / `ClusterModel` helpers.
- Почему здесь: используется несколькими controllers.
- Почему не в API: это runtime helper around API objects, а не type definition.

#### [modelobject_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/modelobject/modelobject_test.go)

- Назначение: verifies helper semantics.

### `internal/support/resourcenames/`

#### [names.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/resourcenames/names.go)

- Назначение: единый canonical policy для owner-based resource naming,
  owner-label rendering/extraction, full owner annotations и label
  normalization.
- Почему здесь: эти правила реально shared между `catalogstatus`,
  `catalogcleanup`, `sourceworker` и `uploadsession`.
- Почему не в каждом adapter package: duplication уже была именно ошибкой;
  package-local `names.go` не несли отдельной архитектурной границы и только
  плодили лишние файлы.

#### [names_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/resourcenames/names_test.go)

- Назначение: verify naming helper behavior.

### `internal/support/testkit/`

#### [testkit.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/support/testkit/testkit.go)

- Назначение: shared test-only scheme/object/fake-client fixtures.
- Почему здесь: это real shared support layer для controller tests.
- Почему не в каждом `*_test.go`: test architecture уже страдала от
  duplicated fixture sprawl.
- Почему не в production package: файл живёт в support, но используется только
  тестами и не вводит business logic.

## 19. Что сейчас ещё выглядит самым спорным

Главный remaining кандидат на следующий reduction cut:

- [auth_secret.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/auth_secret.go)
- [build.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/build.go)

Почему он ещё допустим:
- это всё ещё adapter-local preparation code, не скрытая доменная логика;
- после схлопывания replay/create shell они уже не держат отдельные lifecycle
  ветки.

Почему его всё равно надо дальше резать:
- `sourceworker` всё ещё отделяет auth projection и pod rendering по старому
  file split;
- `uploadsession` уже убрал отдельный request-mapping file, но в `pod.go` всё
  ещё живёт небольшой mapping helper, который при следующем круге можно либо
  оставить как честный local helper, либо схлопнуть дальше, если появится ещё
  один реальный consumer.

## 20. Что не должно появляться снова

- новый top-level patchwork package рядом с `controllers/`, `adapters/`,
  `support/`, если его роль уже укладывается в существующий слой;
- shared helper в `support/`, если он начинает принимать business decisions;
- duplicated scheme/model fixtures по controller packages;
- inline Kubernetes resource build inside reconciler files;
- новый implementation-specific contract в public-facing packages, если это
  можно удержать в `ports` или `adapters`.
