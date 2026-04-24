## 1. Заголовок

Audit-first reduction program: убирать дублирование и лишний код без blanket rewrite

## 2. Контекст

Пользователь требует продолжать cleanup и радикально сокращать код, но live
repo уже не содержит очевидного массового dead code: `deadcode`,
complexity/LOC gates и `make verify` проходят.

Значит следующий шаг нельзя делать как repo-wide delete/rewrite "всё пополам":
нужен audit-first workstream, который режет только defendable duplication и
oversized live boundaries.

Первый bounded slice уже сделан в `modelpack/oci` direct-upload transport:

- общий sealing completion path схлопнут;
- single-call `uploadNext*` helpers убраны обратно в loops;
- protocol и checkpoint semantics не менялись.

На continuation шаге read-only subagents и локальный audit сошлись на следующем
безопасном reduction target:

- `images/controller/internal/adapters/k8s/uploadsessionstate` держит один
  secret-backed codec/mutator cluster, но logic split размазан по нескольким
  файлам;
- `SaveMultipartSecret` и `Client.SaveMultipart` дублируют один mutation path;
- terminal-phase writes частично дублируются между free functions и `Client`;
- этот slice можно уменьшить без смены secret schema, session lifecycle
  semantics и runtime/store boundary.

Пользователь уточнил численный ориентир: текущие Go code lines около `54 643`,
целевое состояние около `25 000`. Это меняет workstream из локального cleanup в
длительную reduction program: каждый continuation diff должен давать измеримый
net-negative LOC и оставлять ranked backlog крупных deletion candidates.

## 3. Постановка задачи

Нужно продолжать bounded reduction workstream серией защищённых slices, а не
unsafe blanket rewrite:

- удерживать canonical bundle для audit-first code reduction;
- продолжать сокращать defendable duplication в уже выбранных live boundaries;
- текущим continuation slice сжать `uploadsessionstate` codec/mutator cluster;
- следующим continuation slice убрать точное archive helper duplication между
  `sourcefetch` и `modelpack/oci`;
- следующим bounded implementation slice сжать package-local upload archive
  publication workflow в `dataplane/publishworker`;
- удалить fake DTO в publication control-plane без переноса public status
  ownership в controller boundary;
- удалить оставшийся fake upload-session application hop, оставив validation в
  существующих `publishop` / `ingestadmission` boundaries;
- сжать package-local `catalogcleanup` DMCR GC request secret mutation без
  изменения backend protocol/storage semantics;
- сжать same-package `modelpack/oci` generated archive layer plumbing без
  изменения artifact format, `DMCR` protocol, raw/ranged byte paths или
  runtime topology;
- выполнить sourcefetch-local HuggingFace test fixture cleanup без изменения
  production HuggingFace byte path;
- привести hand-written Go files к `<350` physical LOC через same-package
  test/file splits;
- оставить тот же published/runtime behavior, secret schema, session lifecycle
  semantics и test evidence.

## 4. Scope

- `plans/active/repo-code-reduction-audit/*`
- `images/controller/internal/adapters/modelpack/oci/direct_upload_transport.go`
- `images/controller/internal/adapters/modelpack/oci/direct_upload_transport_raw.go`
- `images/controller/internal/adapters/modelpack/oci/direct_upload_transport_raw_flow.go`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
- `images/controller/internal/adapters/sourcefetch/*archive*`
- `images/controller/internal/adapters/sourcefetch/tar_gzip.go`
- `images/controller/internal/adapters/modelpack/oci/materialize_support.go`
- `images/controller/internal/adapters/modelpack/oci/materialize_layers.go`
- `images/controller/internal/adapters/modelpack/oci/publish_archive_source*.go`
- `images/controller/internal/adapters/modelpack/oci/publish_object_source*.go`
- `images/controller/internal/support/archiveio/*`
- `images/controller/internal/dataplane/publishworker/upload*.go`
- `images/controller/cmd/ai-models-controller/config.go`
- `images/controller/cmd/ai-models-controller/resources.go`
- `images/controller/internal/application/publishplan/issue_upload_session*.go`
- `images/controller/internal/adapters/k8s/uploadsession/*.go`
- `images/controller/internal/ports/publishop/operation_contract.go`
- `images/controller/internal/adapters/sourcefetch/*huggingface*_test.go`
- `images/dmcr/internal/directupload/*.go`
- `images/dmcr/internal/garbagecollection/*_test.go`
- `images/hooks/pkg/hooks/sync_artifacts_secrets/*_test.go`
- при необходимости новые/соседние helpers в тех же packages
- узкие direct-upload tests в `images/controller/internal/adapters/modelpack/oci/*`
- узкие upload-session-state tests в
  `images/controller/internal/adapters/k8s/uploadsessionstate/*`

## 5. Non-goals

- не обещать repo-wide 50% reduction в одном diff;
- не переписывать сейчас весь controller tree;
- не менять `DMCR` direct-upload API;
- не менять checkpoint/store contract;
- не менять upload-session secret schema или runtime session record contract;
- не ослаблять resume/recovery behavior ради меньшего LOC;
- не смешивать `uploadsessionstate` с dataplane/runtime boundary;
- не менять archive extraction security policy;
- не менять archive layer selection semantics;
- не объединять `sourcefetch`, `publishworker` и `modelpack/oci` в один
  монолитный artifact IO package;
- не объединять auth/storage/runtime projection boundaries ради LOC;
- не резать `nodecache` / `modeldelivery` / `nodecacheruntime` /
  `runtimehealth` как cleanup без отдельного topology decision;
- не переносить upload-session owner/identity admission в adapter-only
  ad-hoc validation;
- не выносить DMCR GC wire keys в shared package и не превращать backend seam в
  публичный/platform contract;
- не создавать новый cross-package artifact IO framework вокруг
  `sourcefetch`/`publishworker`/`modelpack/oci`;
- не менять raw layer path и direct ranged object-source path в `modelpack/oci`;
- не смешивать этот reduction slice с governance edits.

## 6. Затрагиваемые области

- `modelpack/oci` direct-upload flow;
- `uploadsessionstate` secret codec/mutator flow;
- shared archive filesystem helpers между `sourcefetch` и `modelpack/oci`;
- task bundle для нового reduction workstream.

## 7. Критерии приёмки

- новый canonical bundle фиксирует audit-first reduction strategy;
- в `modelpack/oci` стало меньше duplicated direct-upload flow code, а не больше
  helper-on-helper scaffolding;
- в `uploadsessionstate` стало меньше duplicated mutation logic и меньше
  unnecessary file split внутри одного secret codec/mutator cluster;
- raw и described direct-upload paths сохраняют:
  - abort-on-failure semantics;
  - resume/recovery semantics;
  - sealing/complete checkpoint transitions;
  - тот же external protocol с `DMCR`;
- `uploadsessionstate` сохраняет:
  - тот же secret data/annotation schema;
  - тот же phase transition behavior;
  - тот же multipart/probe/token parsing behavior;
  - тот же package-level contract для внешних callers;
- archive helper consolidation сохраняет:
  - отказ от symlink/hardlink/unsafe paths;
  - tar/gzip/zstd/zip handling;
  - selected archive file semantics для publish layers;
- DMCR direct-upload test consolidation сохраняет тот же behavior coverage и
  уменьшает повторяемый HTTP setup/start/complete boilerplate;
- `publishworker` archive-flow consolidation сохраняет:
  - local archive path без object reader;
  - staged archive path с object reader/range reader;
  - zip `SizeBytes` для ranged reader;
  - staged cleanup после успешного publish;
  - unsupported input-format fallthrough;
- controller command files укладываются в `<350` physical LOC без изменения
  env/flag parsing и bootstrap option semantics;
- publication control-plane cleanup не меняет public status/API semantics:
  `publishobserve` observation/status bridge остаётся за пределами
  `catalogstatus`;
- upload-session validation cleanup удаляет только fake DTO/application hop:
  `publishop.Request.Validate` и `ingestadmission.ValidateUploadSession`
  остаются источниками правил;
- `catalogcleanup` GC request cleanup сохраняет immediate-arm semantics:
  один timestamp для `requested-at`/`switch`, удаление `done` при refresh,
  корректный cleanup пустого direct-upload token payload;
- `modelpack/oci` generated archive cleanup сохраняет descriptor fields,
  close/error ordering, compression normalization и range slicing semantics;
- HuggingFace sourcefetch test fixture cleanup не меняет production fetch,
  mirror, auth, range или object-source behavior;
- DMCR direct-upload production split остаётся same-package decomposition:
  нет новых backend interfaces/coordinators, нет изменения `handleComplete`
  step order, sealed metadata, repository link, verification или cleanup
  semantics;
- garbagecollection и hook test splits не меняют production behavior;
- все hand-written Go files укладываются в `<350` physical LOC; generated
  `zz_generated.deepcopy.go` допускается как generated exception;
- узкие direct-upload tests проходят;
- узкие upload-session-state tests проходят;
- узкие archive/sourcefetch/modelpack tests проходят;
- узкие DMCR direct-upload tests проходят;
- узкие DMCR garbagecollection tests проходят;
- узкие hooks sync-artifacts-secrets tests проходят;
- `make verify` проходит.

## 8. Риски

- легко спрятать duplication в слишком generic helper и ухудшить читаемость;
- легко повредить interrupted upload recovery;
- легко перепутать raw digest-building flow и described precomputed-descriptor
  flow.
- легко повредить terminal-phase semantics или cleanup-handle lifecycle в
  `uploadsessionstate`;
- легко получить file binpack без реального уменьшения duplication.
- легко случайно изменить path normalization или symlink rejection в archive
  extraction/publishing.
- легко превратить DMCR direct-upload split в новый fake coordinator/interface
  вместо same-package decomposition.
- легко смешать public status semantics с controller persistence при попытке
  схлопнуть `publishobserve` целиком.
