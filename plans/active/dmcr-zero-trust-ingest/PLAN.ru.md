## 1. Current phase

Этап 1 corrective continuation перед дальнейшей phase-2 работой.

Publication baseline уже landed, но ingest в internal `DMCR` ещё не даёт
production-ready guarantees для zero-trust, resume и large-object economics.
Пока этот контур не доведён, phase-2 runtime/distribution workstreams нельзя
считать надёжным основанием.

## 1a. Execution status

На текущий момент уже выполнено:

- Slice 2: `DMCR` переведён на zero-trust seal без полного backend-copy.
- Slice 3: controller-side direct-upload получил compact `Secret` journal,
  restart-safe resume в `publish-worker` и повторный запуск `sourceworker`
  после падения pod при живом upload state.
- Slice 4: running-state и byte-progress начали подниматься из runtime state
  в observation path и condition messages контроллера.

Следующий открытый срез:

- финально довести public progress/status contract вокруг нового ingest flow:
  - отдельный runtime progress field для sourceworker;
  - честный top-level `status.progress` для publication path;
  - docs/review surface без drift между upload-progress и publish-progress
    narratives.

## 2. Orchestration

`solo`

Причина:

- задача одновременно меняет storage/index model, direct-upload contract,
  controller publication wiring и operator-facing documentation;
- здесь есть архитектурные решения по trust boundary, durable session state и
  physical-vs-logical blob identity.

Ограничение текущей сессии:

- delegation не используется, потому что в текущем runtime она допустима
  только по явному запросу пользователя;
- поэтому bundle исполняется как `solo`, а архитектурные выводы фиксируются
  прямо в текущем plan/doc surface.

## 3. Slices

### Slice 1. Зафиксировать целевой ingest contract и state machine

Цель:

- перестать спорить на уровне общих слов и зафиксировать одну defendable
  схему: кто вычисляет digest, где живут байты, где живёт session metadata и
  как происходит seal/commit.

Файлы/каталоги:

- `plans/active/dmcr-zero-trust-ingest/*`
- при необходимости `docs/CONFIGURATION*.md`
- при необходимости `images/dmcr/README.md`

Проверки:

- `rg -n "direct-upload|digest|session|resume|copy|blob" images/dmcr images/controller docs`

Артефакт результата:

- bundle с явным verdict:
  - zero-trust digest verification inside `DMCR`;
  - no full object rewrite on seal;
  - durable resume semantics.

### Slice 2. Переделать внутреннюю storage/index модель `DMCR`

Цель:

- разорвать текущую жёсткую связь между "канонический published digest" и
  "физический object key, известный до seal", чтобы убрать полный storage copy
  и сохранить published digest contract снаружи.

Файлы/каталоги:

- `images/dmcr/internal/directupload/*`
- `images/dmcr/internal/...` вокруг registry metadata/index
- тесты `images/dmcr/internal/directupload/*_test.go`

Проверки:

- `go test ./images/dmcr/internal/directupload/...`

Артефакт результата:

- `DMCR`, который:
  - хранит durable ingest session;
  - seal'ит уже загруженный физический объект без полной переписи;
  - коммитит logical digest/index metadata отдельно от physical upload key;
  - хранит published digest mapping через `.dmcr-sealed` sidecar и
    `sealeds3` storage driver.

### Slice 3. Переделать controller-side attach/resume/finalize flow

Цель:

- сделать publication worker aware of durable ingest sessions вместо текущего
  best-effort `listParts()` recovery в рамках живой сессии.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`
- при необходимости `images/controller/internal/controllers/catalogstatus/*`

Проверки:

- `go test ./images/controller/internal/adapters/modelpack/oci/...`

Артефакт результата:

- controller-side flow, который:
  - умеет продолжать upload session после restart;
  - не теряет state между reconcile attempts;
  - корректно завершает или abort'ит ingest.

### Slice 4. Добавить progress/status/observability

Цель:

- перестать оставлять оператора наедине с сырыми логами долгой загрузки.

Файлы/каталоги:

- `images/controller/internal/controllers/catalogstatus/*`
- `images/dmcr/internal/directupload/*`
- `docs/CONFIGURATION*.md`
- возможно `images/controller/README.md`

Проверки:

- `go test ./images/controller/internal/controllers/catalogstatus/...`
- `make test`

Артефакт результата:

- bounded progress/state signal по ingest:
  - started / resumed / sealing / committed / failed / aborted;
  - byte or part progress where technically available.
- top-level `status.progress` для sourceworker-driven publication path
  проецируется из machine-readable runtime field, а не из message scraping.

### Slice 5. Удалить старые хвосты и синхронизировать документацию

Цель:

- после cutover не оставить в репозитории вторую "почти такую же" схему
  ingest path.

Файлы/каталоги:

- `images/dmcr/internal/directupload/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `docs/CONFIGURATION*.md`
- `images/dmcr/README.md`
- `images/controller/README.md`

Проверки:

- `make fmt`
- `make test`
- `make verify`

Артефакт результата:

- один канонический ingress path без stale docs, dead code и legacy narrative.
- execution status:
  - контроллерная документация и `CONFIGURATION*` синхронизированы с
    direct-upload checkpoint/resume semantics и bounded progress surface.
  - running-status теперь поднимает machine-readable condition reasons для
    `started/uploading/resumed/sealing/committed` поверх того же checkpoint.

## 4. Rollback point

После Slice 1 можно безопасно остановиться:

- целевой contract/state machine уже зафиксирован;
- код ещё не переписан;
- runtime не находится в полуразобранном состоянии.

После Slice 2 rollback ещё возможен, если storage/index seam уже отделён и
покрыт тестами, но controller-side attach/resume ещё не переключён на новый
contract.

## 5. Final validation

- `make fmt`
- `make test`
- `make verify`
