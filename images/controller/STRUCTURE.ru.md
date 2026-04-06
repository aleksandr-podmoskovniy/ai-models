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

## 3. `internal/bootstrap/`

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

## 4. `internal/domain/publishstate/`

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
- Почему не в `publishrunner`: adapter не должен решать бизнес-исход.

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

## 5. `internal/application/`

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

## 6. `internal/ports/publishop/`

Порты — shared boundary между use cases/domain и concrete adapters.

### [operation_contract.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/operation_contract.go)

- Назначение: shared operation contract primitives.
- Почему здесь: это reusable port contract.
- Почему не в `publishrunner`: adapter не должен владеть shared contract.
- Почему не `ports/publication`: пакет содержит не “всю publication”, а
  operation/runtime contract для controller-owned execution boundary.

### [ports.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/ports.go)

- Назначение: shared runtime interfaces и worker/session handles.
- Почему здесь: это reusable seam between controller and concrete adapters.
- Почему не в domain: это infrastructural contracts, а не business vocabulary.
- Почему source worker и upload session теперь оба идут через `GetOrCreate`:
  concrete runtime adapters не должны расходиться по surface area без реальной
  поведенческой причины; раньше отдельный `Get` у `sourceworker` только держал
  лишнюю асимметрию и подталкивал `publishrunner` к лишнему create-vs-observe
  split.
- Почему не держит `OperationStore`: persisted `ConfigMap` protocol сейчас не
  является сменным shared seam; это controller-local storage boundary внутри
  `publishrunner`, и выносить его в shared ports без второго адаптера было
  ложной абстракцией.

### Tests

#### [operation_contract_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/operation_contract_test.go)

- Назначение: проверяет operation contract primitives и ownership helpers.

#### [ports_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/ports/publishop/ports_test.go)

- Назначение: проверяет runtime-port shapes и handle contracts.

## 7. `internal/publishedsnapshot/`

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

## 8. `internal/artifactbackend/`

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

## 9. `internal/controllers/`

Здесь живут только concrete reconcilers и их тонкие observation/persistence
shell files.

### `internal/controllers/catalogstatus/`

#### [options.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/options.go)

- Назначение: только setup/runtime options и thin reconciler types.
- Почему здесь: options принадлежат concrete controller.
- Почему здесь нет `RequeueAfter`: polling cadence для status owner не является
  module/runtime contract. Это локальная lifecycle policy и она должна жить
  рядом с reconcile path, а не притворяться operator option.

#### [reconciler.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/reconciler.go)

- Назначение: thin reconcile shell для `Model` / `ClusterModel` status owner.
- Почему здесь: это Kubernetes adapter.
- Почему не в application: содержит `client.Get`, requeue, adapter wiring.

#### [io.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/io.go)

- Назначение: весь adapter-side IO для status controller:
  `ConfigMap -> domain Observation`, operation create, cleanup-handle
  persistence, status patch.
- Почему здесь: это единая concrete read/write boundary вокруг status owner.
- Почему не split на `observation.go` и `persistence.go`: отдельной
  самостоятельной границы там не было; это был искусственный micro-split.

#### Tests

#### [reconciler_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/reconciler_test.go)

- Назначение: проверяет public status projection, request creation и corrupted
  operation replay behavior.

#### [test_helpers_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/catalogstatus/test_helpers_test.go)

- Назначение: держит adapter-local operation/status fixtures и result builders.
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

### `internal/controllers/publishrunner/`

Это concrete durable execution boundary вокруг operation `ConfigMap`. Здесь
должны оставаться только три реальные ответственности: outer reconcile shell,
persisted `ConfigMap` protocol и source/upload runtime orchestration.

- Почему не `publicationops`: `ops` слишком vague и не объясняет роль пакета.
  Здесь controller именно запускает и ведёт durable publication run, поэтому
  concrete controller boundary названа `publishrunner`.

#### [options.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/options.go)

- Назначение: controller options и setup.
- Почему здесь нет `RequeueAfter`: polling ожидания `worker-result.json` — это
  локальная lifecycle policy source-worker branch, а не внешний setup knob;
  держать её в options было той же ошибкой, что и раньше в `catalogstatus`.

#### [reconciler.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/reconciler.go)

- Назначение: thin outer reconcile shell.
- Логика внутри: только object load, branch dispatch, minimal failure/result
  persistence и controller-runtime `Result`.

#### [configmap_protocol.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/configmap_protocol.go)

- Назначение: весь concrete `ConfigMap` protocol boundary:
  keys/constants, decode/encode, mutate persisted state, validate persisted
  status, view persisted status as domain operation view.
- Почему здесь: это одна concrete storage protocol boundary для
  `publishrunner`.
- Почему не split на `constants.go`, `configmap_codec.go`,
  `configmap_mutation.go`, `status.go`: отдельной архитектурной выгоды от
  такого дробления не было; один bounded protocol file оказался честнее и
  компактнее.
- Почему внутри файла всё ещё есть create/read/mutate helpers: это один
  persisted protocol boundary; здесь важнее сохранить один honest seam, чем
  снова разнести его по нескольким псевдослоям.
- Почему внутри файла теперь есть generic decode/store helpers: это не новый
  слой, а локальный способ убрать ручной JSON/unmarshal/marshal boilerplate из
  того же bounded protocol seam; helper’ы не переживают границу файла и не
  превращаются в shared utility package.

#### [source.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/source.go)

- Назначение: adapter path для source worker operation branch.
- Логика внутри: один entrypoint вокруг `GetOrCreate`, local waiting policy и
  apply source-worker terminal result to the persisted operation state.
- Почему здесь больше нет отдельного create-vs-observe shell: после выравнивания
  runtime contract по одному `GetOrCreate` separate wrapper перестал добавлять
  смысл и только размазывал один и тот же branch по двум функциям.
- Почему здесь есть `sourceWorkerResultPollInterval`: waiting for
  `worker-result.json` — это concrete branch behavior именно source-worker
  flow; переносить его обратно в setup options или shared port было бы ложной
  абстракцией.

#### [upload.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/upload.go)

- Назначение: adapter path для upload session branch.
- Логика внутри: issue/reuse upload session, observe session state, map upload
  terminal result and expiry/failure branches into the persisted operation.

#### [worker_result.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/worker_result.go)

- Назначение: один shared helper для decode `worker-result.json` и перевода его
  в domain `PublicationSuccess`, плюс нормализация fallback failure message.
- Почему здесь: это shared concrete helper именно для `publishrunner` runtime
  branches; он одинаково нужен `source` и `upload`, но не является ни domain,
  ни store, ни K8s adapter boundary.
- Почему не дублировать в `source.go` и `upload.go`: эта duplication уже была
  ошибкой и только раздувала два соседних adapter branch файла.

#### Tests

#### [reconcile_core_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/reconcile_core_test.go)

- Назначение: invariant/fail-closed lifecycle cases without runtime-branch
  specifics.

#### [reconcile_source_worker_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/reconcile_source_worker_test.go)

- Назначение: source-worker family including start, waiting and terminal-result
  branches.

#### [reconcile_upload_session_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/reconcile_upload_session_test.go)

- Назначение: upload-session family including issue, ready/running, expiry and
  terminal-result branches.

#### [configmap_protocol_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/configmap_protocol_test.go)

- Назначение: one concrete persisted-protocol boundary: decoder/accessor,
  upload payload, mutation and status-validation families.

#### [test_helpers_test.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/controllers/publishrunner/test_helpers_test.go)

- Назначение: minimal bootstrap and canonical scenario fixtures.
- Почему отдельно: он не строит второй shadow API над production contract, а
  только стабилизирует test input shape for the package.

Почему package всё ещё требует следующего круга reduction:
- persisted `ConfigMap` protocol и source/upload branches пока живут рядом;
- это допустимо, пока обе части образуют один durable controller boundary;
- любой новый file-level split без новой реальной границы должен считаться
  ошибкой, а не улучшением.

## 10. `internal/adapters/k8s/`

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
  `SetControllerReference -> Create -> AlreadyExists -> Get` и
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

#### [service.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/service.go)

- Назначение: concrete source-worker adapter: shared runtime port plus internal
  Pod/auth-secret CRUD.
- Почему не держит свои `*NameFor()` helpers: resource naming — общий support
  concern, а не `sourceworker`-specific API.
- Почему без отдельного `runtime.go`, `NewRuntime` и отдельного `Get`: сам
  `Service` уже является concrete runtime adapter, а второй constructor path и
  лишний read-only runtime method только дублировали wiring и расходились по
  surface area с `uploadsession` без новой границы.
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

Это concrete session supplements для `spec.source.type=Upload`.

#### [options.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/options.go)

- Назначение: normalize/validate runtime options и хранить package-level
  constants upload-session supplements.

#### [request.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/request.go)

- Назначение: validate shared `publishop.OperationContext` и map его в upload
  session plan.
- Почему не через local request type: upload adapter больше не зеркалит shared
  port contract локальными `Request/OwnerRef`.

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
- Почему не open-code create-or-get: owned Pod create/reuse path такой же
  concrete K8s adapter shell, как и для session `Secret` / `Service`, поэтому
  он использует `adapters/k8s/ownedresource`.
- Почему не open-code workspace/registry Pod shell: общий `/tmp` + registry CA
  volumes/mounts вынесен в `adapters/k8s/workloadpod`.

#### [status.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/status.go)

- Назначение: session aggregate shape и derivation of upload status from created
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

## 11. `internal/support/`

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
  owner-label rendering/extraction и label normalization.
- Почему здесь: эти правила реально shared между `publishrunner`,
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

## 12. Что сейчас ещё выглядит самым спорным

Главный remaining кандидат на следующий reduction cut:

- [service.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/sourceworker/service.go)
- [service.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/adapters/k8s/uploadsession/service.go)

Почему он ещё допустим:
- это реальные concrete CRUD adapters, не скрытые доменные слои;
- после выноса runtime port implementation они уже лежат в правильном слое.

Почему его всё равно надо дальше резать:
- между `sourceworker` и `uploadsession` всё ещё заметен повтор по owner/request
  translation и service lifecycle shell;
- следующий cut должен либо убрать повтор, либо доказать, что он неизбежен из-за
  разного supplement model.

## 13. Что не должно появляться снова

- новый top-level patchwork package рядом с `controllers/`, `adapters/`,
  `support/`, если его роль уже укладывается в существующий слой;
- shared helper в `support/`, если он начинает принимать business decisions;
- duplicated scheme/model fixtures по controller packages;
- inline Kubernetes resource build inside reconciler files;
- новый implementation-specific contract в public-facing packages, если это
  можно удержать в `ports` или `adapters`.
