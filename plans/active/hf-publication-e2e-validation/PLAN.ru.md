## 1. Current phase

Этап 1 для live publication path и одновременно проверка того, насколько
текущий runtime baseline уже готов к целевой прод-картине без лишних
intermediate seams.

## 2. Orchestration

`full`

Причина:

- задача начиналась как эксплуатационная проверка live path;
- live проверка выявила узкий дефект DMCR direct-upload без нового API или
  layout fork;
- continuation меняет storage verification path и runtime defaults, поэтому
  требует read-only проверки границ backend/runtime.

Read-only subagents:

- `backend_integrator`: проверяет безопасную семантику S3 checksum/digest для
  direct-upload verification;
- `integration_architect`: проверяет DKP module runtime defaults, resource/QoS
  boundary и последствия `maxConcurrentWorkers=4`;
- `reviewer`: финальный read-only review по коду, bundle hygiene и residual
  risks.

Subagent findings:

- `ETag`, multipart part checksum, `ChecksumAlgorithm` и `COMPOSITE`
  checksum нельзя использовать как OCI `sha256` digest;
- безопасный fast path: только `ChecksumSHA256` с `ChecksumType=FULL_OBJECT`,
  base64 decode ровно в 32 bytes, затем hex `sha256:<hex>`;
- для AWS multipart SHA256 чаще всего будет composite или отсутствовать,
  поэтому fallback read остаётся обязательным;
- runtime defaults должны оставаться во internal values/templates/env, не в
  public `ModuleConfig`;
- с `maxConcurrentWorkers=4` более безопасный default memory request — `1Gi`,
  а не `512Mi`, при limit `2Gi`.

## 3. Slices

### Slice 1. Зафиксировать bundle и выбрать корректные HF-модели

Цель:

- подобрать две публичные модели разных форматов, которые соответствуют
  текущим правилам отбора файлов;
- не попасть в ложный `GGUF`-сценарий с несколькими квантами/шардами.

Файлы/каталоги:

- `plans/active/hf-publication-e2e-validation/*`

Проверки:

- live `Hugging Face` metadata inspection;
- manual consistency review against current `sourceFetchMode=Direct`.

Артефакт результата:

- две конкретные модели и ожидаемый selected-files set.

### Slice 2. Прогнать обе модели через live cluster publication path

Цель:

- создать временный namespace;
- применить два `Model`;
- дождаться терминального результата и собрать status/log evidence.

Файлы/каталоги:

- live cluster objects only

Проверки:

- `kubectl apply/get/describe/logs`;
- `kubectl get models -o yaml/json`;
- inspection of source-worker Pods and related Secrets/Events if needed.

Артефакт результата:

- живой operational evidence по двум публикациям.

### Slice 3. Восстановить реальный byte path и объёмы

Цель:

- по live status, логам, объектам и known runtime wiring восстановить точный
  путь байтов end-to-end;
- явно посчитать полные копии, packaging и верхние оценки по объёмам.

Файлы/каталоги:

- live cluster evidence;
- `docs/CONFIGURATION.ru.md`;
- touched controller/runtime code only for reading.

Проверки:

- cross-check live evidence against code/docs;
- manual consistency review.

Артефакт результата:

- детальный фактический byte path по `Safetensors` и `GGUF`.

### Slice 4. Сопоставить с целевой картиной и при необходимости закрепить evidence

Цель:

- дать операторское заключение:
  - где текущее поведение совпадает с целью;
  - где ещё есть дрифт/узкие места;
  - насколько текущий путь уже можно считать defendable prod baseline.
- при полезности зафиксировать результаты в `TEST_EVIDENCE`.

Файлы/каталоги:

- `images/controller/TEST_EVIDENCE.ru.md` при необходимости

Проверки:

- `git diff --check` если docs touched;
- `make verify` если repo files touched.

Артефакт результата:

- понятный разбор "как сейчас" против "куда идём".

### Slice 5. Исправить найденный дефект GGUF direct-upload

Цель:

- устранить сбой публикации manifest после успешной загрузки GGUF raw-layer;
- разделить сырой S3 object key и логический путь storage driver в sealed
  metadata.

Файлы/каталоги:

- `images/dmcr/internal/directupload/paths.go`
- `images/dmcr/internal/directupload/paths_test.go`
- `images/dmcr/internal/directupload/service.go`
- `images/dmcr/internal/directupload/service_test.go`

Проверки:

- `go test ./internal/directupload ./internal/registrydriver/sealeds3`
  в `images/dmcr`;
- `make verify`.

Live evidence:

- `tinyllama-gguf-live` полностью загрузил layer
  `ggml-model-q4_0.gguf` размером `636734304` байт;
- direct-upload state зафиксировал digest
  `sha256:3849e8024b234f2ec0f2e3b5b59ea368804486563394886ae03d0a67ae70d504`;
- manifest PUT упал в DMCR с
  `s3aws: invalid path: dmcr/_ai_models/direct-upload/objects/.../data`;
- причина: sealed metadata хранит raw S3 key с root prefix, а registry
  storage driver ожидает логический path `/_ai_models/...` и сам применяет
  `rootdirectory`.

### Slice 6. Настроить publication capacity и убрать лишний S3 read там, где S3 даёт full-object digest

Цель:

- снизить default memory для publication worker до фактического streaming path;
- поднять default `maxConcurrentWorkers` до `4`;
- перед полным пересчётом DMCR digest сначала спрашивать у S3 trusted
  full-object SHA256 checksum;
- не использовать unsafe `ETag` и не принимать multipart `COMPOSITE`
  checksum за полный digest.

Файлы/каталоги:

- `templates/_helpers.tpl`
- `openapi/values.yaml`
- `images/controller/cmd/ai-models-controller/env.go`
- controller tests that assert default resources;
- `images/dmcr/internal/directupload/*`
- `docs/CONFIGURATION.ru.md`

Проверки:

- `go test ./internal/directupload` в `images/dmcr`;
- targeted controller tests for defaults;
- `make verify`.

Acceptance:

- default worker memory request `1Gi`, limit `2Gi`;
- default max concurrent publication workers `4`;
- DMCR accepts only S3 `ChecksumSHA256` with checksum type `FULL_OBJECT`;
- if S3 checksum is missing/unsupported/composite, DMCR falls back to current
  full object read and SHA256 calculation;
- expected digest mismatch still deletes the uploaded object.

## 4. Rollback point

После Slice 3: живой прогон уже выполнен и evidence собран, но никакие
repo-local docs ещё не менялись.

## 5. Final validation

- если изменялись только cluster objects: manual cleanup verification;
- если менялись repo files:
  - `git diff --check`
  - `make verify`
  - `review-gate`

Фактически выполнено:

- `go test ./internal/directupload ./internal/registrydriver/sealeds3` в
  `images/dmcr` — успешно;
- `go test ./cmd/ai-models-controller` в `images/controller` — успешно;
- `make verify` — успешно;
- `git diff --check` — успешно;
- временные `Model` с label `ai-models.deckhouse.io/live-validation=hf-publication`
  удалены;
- временные publication Pods и state Secrets для live проверки отсутствуют.
