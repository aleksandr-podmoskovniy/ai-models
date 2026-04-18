## 1. Current phase

Это сознательный phase reset.

Текущие repo docs и shell всё ещё отражают старый этап:

- internal managed backend around `MLflow`;
- phase-2 catalog layered on top.

Запрос пользователя переводит baseline к новому центру:

- ai-models-owned publication/runtime architecture;
- no `MLflow` live surface;
- no fallback `KitOps` publisher.

Значит в рамках этого workstream нужно не “подправить phase-1”, а переписать
phase narrative и runtime shell под новый reality.

## 2. Orchestration

Overall workstream: `full`

Почему:

- задача затрагивает module layout, build shell, values/OpenAPI, docs, runtime,
  tooling и product baseline;
- это не один refactor, а coordinated removal plus replacement;
- forward-only migration без fallback требует особенно аккуратной slicing
  discipline.

Execution mode для уже landed slices:

- Slice 1, render/OpenAPI contraction и landed backend-shell deletion slice
  допустимы в `solo`, потому что это bounded removal of stale surfaces без
  нового runtime/API design choice;
- перед native publisher cutover workstream возвращается в `full`.

Read-only reviews, которые должны быть закрыты до execution slices:

- `repo_architect`
  - проверить новый module shape после удаления `images/backend` /
    `templates/backend`;
- `integration_architect`
  - проверить, что storage/auth/build/runtime contracts после вырезания
    `MLflow` не распадаются на ad-hoc wiring;
- `api_designer`
  - проверить, что reset не тащит backend/runtime internals в public model API;
- `backend_integrator`
  - проверить, какие backend-specific surfaces реально ещё живые и что должно
    уйти без остатка.

## 3. Slices

### Slice 1. Freeze the new baseline and remove narrative ambiguity

Цель:

- зафиксировать, что repo больше не строится вокруг `MLflow` backend;
- обновить phase docs, чтобы дальнейшие code slices не противоречили repo-local
  правилам и README narrative.

Файлы/каталоги:

- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/development/TZ.ru.md`
- `docs/development/PHASES.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- `rg -n "MLflow|mlflow" README.md README.ru.md docs docs/development`

Артефакт результата:

- docs baseline, который больше не утверждает, что `MLflow` — live center of
  the module.

### Slice 2. Define ai-models-owned `ModelPack` publisher contract

Цель:

- заменить brand-specific `KitOps` contract на ai-models-owned publisher
  boundary.

Нужно зафиксировать:

- first native cutover shape:
  worker-local checkpoint directory -> single controller-owned OCI artifact;
- OCI artifact layout and digest rules;
- manifest/config/blob ownership;
- publication inputs from source mirror / upload staging;
- what remains internal and what is observable in status.

Файлы/каталоги:

- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/modelpack/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/ports/modelpack ./internal/adapters/modelpack/...`

Артефакт результата:

- explicit native publisher design plus bounded implementation target:
  single tar weight layer under `model/`, live-materializer-compatible
  manifest/config shape, binary-free registry push/remove contract.

### Slice 3. Land native publisher and cut over publication path

Цель:

- сделать ai-models-owned publisher live;
- в том же slice удалить `KitOps` live usage, а не оставлять fallback.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/internal/publicationartifact/*`
- `images/controller/werf.inc.yaml`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/... ./internal/dataplane/publishworker/... ./cmd/ai-models-artifact-runtime`
- `cd images/controller && go test ./internal/adapters/modelpack/oci`

Артефакт результата:

- publication path, который больше не вызывает `KitOps`, а runtime image больше
  не тащит `kitops.lock` / `install-kitops.sh` / external publisher binary.

### Slice 4. Delete `MLflow` backend runtime shell

Цель:

- убрать `images/backend/*`, `templates/backend/*` and related scripts/tests as
  live repo surface.

Already landed in this slice:

- удалены `templates/backend/*`, `templates/auth/dex-client.yaml` и
  `templates/module/backend-*`;
- убран backend-only `openapi` contract (`auth`, `artifacts.pathPrefix`,
  internal `backend`/`auth`);
- вычищены legacy render checks и второй managed-postgres auth DB.

Current landing for the same slice:

- удалить `images/backend/*`, backend build/smoke targets в `Makefile` и legacy
  import/cleanup tools;
- синхронизировать docs/module narrative с тем, что historical backend shell
  уже реально удалён из live repo.

Файлы/каталоги:

- `images/backend/*`
- `templates/backend/*`
- `tools/*`
- `tools/helm-tests/*`
- CI / build files

Проверки:

- `rg -n "images/backend|run_hf_import_job|upload_hf_model|run_model_cleanup_job|libai_models_job|backend-build|backend-shell-check|backend-oidc-auth" . -g '!plans/archive/**'`
- `rg -n "MLflow|mlflow" README.md README.ru.md docs docs/development Makefile module.yaml -g '!plans/archive/**'`
- `make helm-template`
- `make kubeconform`

Артефакт результата:

- repo without live backend runtime shell or legacy import/build helpers.

### Slice 5. Delete retired PostgreSQL metadata shell

Цель:

- убрать из live repo оставшийся metadata-database shell, который был нужен
  только historical backend baseline.

Файлы/каталоги:

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/database/*`
- `templates/_helpers.tpl`
- `fixtures/module-values.yaml`
- `fixtures/render/*`
- `tools/helm-tests/*`
- `tools/kubeconform/*`
- `docs/CONFIGURATION*`
- `DEVELOPMENT.md`
- `docs/development/REPO_LAYOUT.ru.md`

Проверки:

- `python3 -m py_compile tools/helm-tests/validate-renders.py`
- `make helm-template`
- `make kubeconform`
- `rg -n "postgres|Postgres|postgresql|managed-postgres|postgresClass" README.md README.ru.md DEVELOPMENT.md docs openapi templates fixtures tools .github/workflows module.yaml -g '!plans/archive/**'`
- `make verify`

Артефакт результата:

- repo without live PostgreSQL config/template/render shell.

### Slice 6. Finish repo contraction and proof

Цель:

- добить остаточные historical references и убедиться, что repo shape уже
  согласован после удаления backend shell и retired PostgreSQL shell.

Файлы/каталоги:

- all touched live surfaces

Проверки:

- `make lint-docs`
- `make helm-template`
- `make kubeconform`
- `make verify`

Артефакт результата:

- coherent repo baseline with no live backend shell and no stale build/docs
  claims about it.

### Slice 7. Stream the native publisher layer upload

Цель:

- убрать последнюю full-size локальную tar-копию из native publisher path;
- сохранить уже landed OCI manifest/config/materializer contract unchanged.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/... ./cmd/ai-models-artifact-runtime`
- `make verify`

Артефакт результата:

- layer bytes stream directly from `checkpointDir` tar writer into registry blob
  upload protocol;
- worst-case local full-size copies shrink from `checkpointDir + temp tar` to
  just `checkpointDir` on the bounded worker volume/PVC.

### Slice 8. Release HF/upload source artifacts before publish

Цель:

- убрать из pre-publish byte path временные source artifacts, которые больше не
  нужны после materialization/validation;
- зафиксировать более честный runtime state к моменту старта native publisher:
  локально остаётся только `checkpointDir`.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- `HF` path удаляет `.hf-snapshot` до старта publish step, сохраняя валидный
  `checkpointDir`;
- `upload staging` path удаляет локально скачанный source object до старта
  publish step, сохраняя валидный `checkpointDir`;
- public API и OCI artifact contract не меняются, а true source-to-registry
  streaming остаётся отдельным следующим workstream.

### Slice 9. Pull selected format/profile paths out of mandatory checkpointDir

Цель:

- сделать первый реальный cut против обязательного `checkpointDir`, не ломая
  остальные source/format paths;
- покрыть только те paths, где runtime facts уже достаточны:
  - `HF Safetensors` profile resolution from remote summary;
  - direct uploaded `GGUF` file publish without checkpoint materialization.

Файлы/каталоги:

- `images/controller/internal/adapters/modelprofile/*`
- `images/controller/internal/adapters/modelformat/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelprofile/... ./internal/adapters/modelformat ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- `HF Safetensors` path больше не требует local `checkpointDir` именно для
  profile resolution: summary считается из remote `config.json` plus
  `.safetensors` byte sizes;
- direct `GGUF` upload path больше не создаёт `checkpointDir`, а публикует
  исходный file path напрямую;
- remaining archive/safetensors upload paths всё ещё materialized и остаются
  отдельным следующим slice.

### Slice 10. Collapse Hugging Face publish to a single local model root

Цель:

- убрать из live `HuggingFace` publish path второй локальный tree entirely;
- перестать гонять bytes через `.hf-snapshot -> checkpointDir` handoff, когда
  selected files уже можно validated/published in place;
- сохранить failure semantics простыми: один локальный model root, optional
  remote summary for `Safetensors`, same native OCI publisher.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- `FetchRemoteModel(HuggingFace)` downloads/materializes selected files directly
  into one final local model root and validates that root in place;
- `publish-worker` publishes from that same model root without separate
  `checkpointDir` tree;
- for current `HuggingFace` path worst-case local full-size copies shrink to
  one model root on the bounded worker volume/PVC plus the durable source
  mirror in object storage when enabled.

### Slice 11. Make native OCI push deterministic and resumable

Цель:

- довести native publisher до reliable interruption semantics without temp
  files;
- перейти от one-shot pipe `PATCH` к deterministic digest-first blob contract:
  precompute digest/size, `HEAD`-based de-duplication, chunked `PATCH` upload,
  `GET <location>` status recovery and `PUT ?digest=...` finalize;
- зафиксировать honest compatibility line относительно upstream `ModelPack`:
  core artifact/config/weight-layer contract aligned, but full mediatype matrix
  remains outside current live runtime scope.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`
- `make verify`

Артефакт результата:

- native publisher computes layer digest/size deterministically before upload;
- existing blobs are skipped via `HEAD /v2/<name>/blobs/<digest>`;
- layer upload uses chunked `PATCH` with `Content-Range`, recovers current
  offset via `GET <location>` after transient failures, and finalizes with
  `PUT ?digest=<digest>`;
- local byte path stays zero-temp-file: one local model root, no tar staging
  copy, but now with bounded retry/resume semantics;
- repo-local notes explicitly capture that current ai-models runtime matches
  upstream `ModelPack` core contract (`artifactType`, config media type, weight
  layer media type, `org.cncf.model.filepath`) after removing `KitOps`, while
  not claiming the broader upstream layer/media-type matrix as implemented.

### Slice 12. Land the full upstream ModelPack layer matrix in the OCI adapter

Цель:

- убрать remaining compatibility gap между нашим native `oci` adapter и full
  upstream `ModelPack` layer contract;
- поддержать весь upstream family layer base types:
  - `weight`
  - `weight.config`
  - `dataset`
  - `code`
  - `doc`
- поддержать upstream format/compression matrix:
  - `raw`
  - `tar`
  - `tar+gzip`
  - `tar+zstd`
- сделать это без public API noise:
  live `Model` / `ClusterModel` contract не растёт, а full layer matrix
  остаётся internal publisher/materializer capability.

Файлы/каталоги:

- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/ports/modelpack ./internal/adapters/modelpack/oci`
- `cd images/controller && go test ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/... ./cmd/ai-models-artifact-runtime`
- `make verify`

Артефакт результата:

- native `oci` adapter умеет валидировать, materialize и publish all upstream
  `ModelPack` layer classes and supported compressions without `KitOps`;
- current live worker call sites могут по-прежнему публиковать только model
  layers, но adapter boundary уже не ограничена single-weight-layer semantics;
- `MaterializeResult.ModelPath` остаётся stable consumer contract for model
  workloads, while auxiliary docs/code/dataset layers unpack рядом в той же
  destination tree.

### Slice 13. Stream upload archives into native OCI publish without checkpointDir

Цель:

- сделать следующий реальный streaming cut уже на upload ingest path, а не
  только на registry push path;
- перестать materialize-ить `tar` / `tar.gz` / `tgz` / `zip` upload bundle в local
  `checkpointDir`, когда:
  - format можно определить по archive entry list;
  - `Safetensors` profile можно посчитать по `config.json` plus entry sizes;
  - native publisher уже умеет принять full `ModelPack` layer contract;
- сохранить bounded fallback:
  - unsupported archive layouts не ломают upload flow, а идут через старый
    extracted path.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- `tar` / `tar.gz` / `tgz` / `zip` upload bundle can be inspected without
  extraction:
  normalized archive root, selected model files and safetensors summary are
  resolved directly from the archive stream;
- publish-worker sends that bundle to native publisher as one streamed
  `weight tar` / `weight tar+gzip` layer with benign-extra stripping, tar/zip
  root normalization and selected-file filtering, instead of
  `PrepareModelInput -> checkpointDir`;
- staged upload keeps one local downloaded archive copy during publish and
  removes it after completion; extra extracted `checkpointDir` is gone for
  streamable upload archive paths.

### Slice 14. Stream archive-wrapped GGUF uploads without checkpointDir

Цель:

- добить следующий real upload-materialization tail after safetensors archive
  streaming;
- убрать `checkpointDir` и для `tar` / `tar.gz` / `tgz` / `zip` uploads,
  которые содержат `GGUF` model file and can be profiled from archive
  metadata alone;
- не вводить speculative generic magic-scanning over arbitrary archive bytes:
  streamable `GGUF` archive path должен оставаться bounded to selected
  `.gguf` entry plus archive metadata.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/modelprofile/gguf/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/modelprofile/gguf ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- archive inspection now returns enough `GGUF` summary to resolve profile from
  selected `.gguf` entry name plus uncompressed size without extraction;
- publish-worker sends archive-wrapped `GGUF` uploads through the same
  archive-backed native publisher seam instead of `PrepareModelInput ->
  checkpointDir`;
- remaining upload materialization is now limited to unsupported or genuinely
  non-streamable archive layouts rather than canonical `Safetensors` and
  `GGUF` bundle shapes.

### Slice 15. Treat zstd tar uploads as first-class streamable archives

Цель:

- довести тот же archive-streaming contract до `tar.zst` / `tar.zstd` /
  `tzst`, а не оставлять эти bundle shapes implicit partial support;
- закрыть end-to-end byte path:
  - upload admission;
  - archive inspection;
  - publish-worker streaming upload path;
  - native archive-backed OCI publish/materialize round-trip;
- не вводить отдельную zstd-only boundary:
  zstd tar должен быть ещё одним canonical streamable archive shape внутри
  already existing upload/archive seams.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/domain/ingestadmission/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/domain/ingestadmission ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- `tar.zst` / `tar.zstd` / `tzst` uploads are accepted and validated as real
  archive inputs rather than falling through unclear suffix-only behavior;
- archive inspection and publish-worker streaming path treat zstd tar exactly
  like other canonical streamable archive families, with one local archive
  copy and no extracted `checkpointDir`;
- native `oci` adapter proves round-trip for zstd archive-backed publish
  without bringing back a materialized local tree.

### Slice 16. Publish mirrored Hugging Face sources without local model root

Цель:

- убрать remaining local `workspace/model` copy из `HF` path в том случае,
  когда source mirror already holds the selected files and remote summary is
  enough for profile resolution;
- не тащить object-storage specifics directly into publish-worker or modelpack
  manifest logic:
  нужен отдельный bounded object-source seam inside native publisher;
- сохранить interrupt-safe deterministic publish:
  precompute and upload must read the same immutable mirror objects rather than
  opportunistically walking a mutable temp directory.

Файлы/каталоги:

- `images/controller/internal/ports/uploadstaging/*`
- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/uploadstaging/s3/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- when `HF` source mirror is enabled and remote profile summary already exists,
  `FetchRemoteModel` no longer materializes `workspace/model`;
- publish-worker builds one explicit mirror-backed object-source layer over
  immutable source-mirror objects and hands it to native publisher instead of
  `ModelDir=workspace/model`;
- native `oci` adapter proves round-trip for this object-source layer without
  introducing local temp copies or a second publication source of truth.

### Slice 17. Publish direct Hugging Face `Safetensors` without local model root

Цель:

- закрыть следующий реальный хвост после mirrored `HF` happy path:
  direct `Hugging Face` publication still creates `workspace/model` even
  though resolved revision and remote profile summary are already enough to
  treat selected files as immutable object-source inputs;
- не тащить `huggingface.co` HTTP/auth/path resolution в publish-worker:
  direct remote object reader and file metadata must stay inside
  `sourcefetch/*`, with worker only mapping source-owned object source into
  the native `ModelPack` layer contract;
- сохранить bounded and stable fallback:
  if direct remote object-source planning fails, fetch path must still fall
  back to the existing one-local-copy model-root materialization instead of
  breaking publication.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- when direct `Hugging Face` `Safetensors` source already has resolved
  revision and remote profile summary, `FetchRemoteModel` no longer downloads
  `workspace/model`;
- `sourcefetch` exposes immutable remote object-source metadata
  (`resolvedURL`, `size`, `etag`) and hides `HF` auth/URL resolution from the
  worker;
- publish-worker maps that direct remote object-source into the same native
  object-source publish seam already used by mirrored `HF` and keeps the old
  one-local-copy path only as a bounded fallback on planning failure.

### Slice 18. Extend direct Hugging Face zero-local-copy to `GGUF`

Цель:

- убрать оставшийся format gap внутри direct `HF` happy path:
  после Slice 17 direct remote object-source publication still works only for
  `Safetensors`, because remote profile summary exists only there;
- не вводить второй special-case publish path for `GGUF`:
  current remote-summary/object-source seam should simply widen to one more
  canonical format;
- сохранить bounded fallback semantics:
  if `GGUF` remote summary cannot be planned from immutable selected file
  metadata, fetch path must still fall back to one-local-copy materialization.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- direct `HF GGUF` source now resolves remote profile from immutable selected
  `.gguf` file metadata (`name` + `size`) without local model root;
- `resolveRemoteProfile()` widens the existing remote-summary seam instead of
  bringing back format-specific local materialization;
- direct `HF GGUF -> OCI` joins `HF Safetensors` in the zero-local-root happy
  path, while unsupported remote summary cases still stay on bounded fallback.

### Slice 19. Add ranged object reads for interrupt-safe object-source upload

Цель:

- after direct and mirrored `HF` zero-local-root landing, current resumable
  OCI upload still replays object-source tar bytes from the beginning on
  interrupted uploads because object-source readers only expose full reads;
- harden object-source publication without changing product boundary:
  inference/runtime image stays external (`Docker Hub`) and this slice only
  improves model artifact byte-path resilience;
- keep the optional-reader pattern:
  ranged reads must stay an additive capability, not a mandatory expansion of
  `uploadstaging.Client` or every source adapter.

Файлы/каталоги:

- `images/controller/internal/ports/uploadstaging/*`
- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/uploadstaging/s3/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- object-source readers may optionally serve ranged reads without widening the
  mandatory client contract;
- native `oci` adapter uses range-aware tar streaming for uncompressed
  object-source layers, so interrupted upload resumes no longer have to
  re-download previously committed `HF`/mirror object bytes from source offset
  `0`;
- publish proof demonstrates that transient `PATCH` failure on object-source
  publish actually triggers ranged source reads during resume.

### Slice 20. Stream direct staged `GGUF` upload from object storage

Цель:

- current upload-stage happy path still downloads a staged direct `GGUF` file
  to local worker storage even though the source object already lives in
  object storage and native publisher already knows how to read object-source
  layers;
- remove that redundant copy without widening product scope:
  ai-models still owns only controller / artifact runtime images, while the
  consumer serving image stays external (`Docker Hub`);
- keep the change bounded and additive:
  only stage-backed direct `GGUF` should move to zero-local-copy in this
  slice, while other upload shapes stay on existing paths.

Файлы/каталоги:

- `README*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- stage-backed direct `GGUF` upload now validates lightweight `GGUF` magic via
  staging reads and publishes as one object-source layer directly from the
  staged object;
- this path no longer creates a local downloaded copy before publish;
- top-level repo narrative now explicitly says ai-models owns controller /
  artifact runtime images, while consumer serving images stay external.

### Slice 21. Stream staged tar-family archives from object storage

Цель:

- after Slice 20, staged direct `GGUF` is zero-local-copy, but staged
  tar-family archives still get downloaded to worker storage before archive
  inspection and native publish;
- extend the existing archive-backed native publisher seam instead of
  inventing a second upload-specific path:
  tar/tar.gz/tgz/tar.zst/tar.zstd/tzst staged archives should reuse archive
  inspection + archive-source publication directly from object storage;
- keep the slice bounded:
  staged `zip` remains on the old local-download fallback until a proper ranged
  `ReaderAt` seam exists.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- staged tar-family archives are now inspected via reopenable object-storage
  reads, not by local download;
- native `oci` archive-source layers now accept an optional archive reader, so
  staged tar-family archives publish directly from object storage without
  inventing a second publish contract;
- staged `zip` stays on explicit local-download fallback rather than pretending
  to be zero-copy without a real ranged `ReaderAt` seam.

### Slice 22. Stream staged `zip` archives via ranged `ReaderAt`

Цель:

- after Slice 21, staged tar-family archives already stream from object
  storage, but staged `zip` still falls back to local download only because
  zip inspection/publication requires random access;
- land a real ranged `ReaderAt` seam instead of pretending `zip` is
  streamable:
  inspect and publish staged `zip` directly from object storage through
  bounded range reads;
- keep the slice additive and explicit:
  no new product surface, only internal zip inspection/publish hardening over
  the existing staging/object-source contracts.

Файлы/каталоги:

- `images/controller/internal/ports/modelpack/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/adapters/modelpack/oci ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- staged `zip` archive inspection now uses a real ranged `ReaderAt` over
  object storage instead of local download;
- native `oci` archive-source publish now supports remote `zip` readers with
  explicit archive size, so staged `zip` publishes directly from object
  storage too;
- the old staged-archive local-download tail is removed for canonical archive
  formats, not just for tar-family bundles.

### Slice 23. Fail fast on invalid uploads before workspace preparation

Цель:

- after Slice 22, canonical upload happy paths already avoid redundant copies,
  but invalid uploads can still fall through to workspace/download shell and
  only fail later during local materialization or validation;
- reuse existing domain upload-probe semantics instead of inventing another
  publishworker-local classifier:
  invalid direct uploads must fail before workspace creation and, for staged
  uploads with object reads, before any local download;
- keep the slice bounded:
  valid publish paths stay unchanged, only pointless local-copy/error path is
  removed.

Файлы/каталоги:

- `images/controller/internal/domain/ingestadmission/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/domain/ingestadmission ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- publish-worker now performs the same cheap upload probe before workspace
  preparation and rejects invalid direct uploads early;
- staged invalid direct uploads no longer download the staged object locally
  just to fail on the same file later;
- the remaining local-copy tail stays limited to valid layouts that still
  require materialization, not to already-invalid inputs.

### Slice 24. Skip workspace creation for local zero-copy upload paths

Цель:

- after Slice 23, invalid uploads already fail before workspace/download, but
  valid local zero-copy happy paths still create `ensureWorkspace(...)`
  eagerly even when they publish directly from the original file or archive;
- keep the slice bounded:
  only local upload fast paths move ahead of workspace creation, while staged
  paths and materialized fallback stay unchanged.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- local direct `GGUF` publish and local streamable archive publish no longer
  create an empty workspace under `SnapshotDir` before taking their zero-copy
  happy path;
- workspace creation now remains only on paths that actually need local
  preparation/materialization.

### Slice 25. Allocate Hugging Face workspace only when fallback really needs it

Цель:

- after Slice 24, local upload zero-copy paths no longer create eager
  workspaces, but `HuggingFace` publish still allocates `ensureWorkspace(...)`
  upfront even when remote/object-source streaming path succeeds with no local
  model root;
- keep the slice bounded:
  make workspace lazy for `HF`, while preserving the existing bounded fallback
  to local materialization when remote planning cannot stay streaming.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- direct and mirrored `HF` streaming happy paths no longer allocate
  `SnapshotDir`/workspace before fetch;
- `sourcefetch` now returns an explicit `workspace required` signal only when
  local model-root materialization is actually needed;
- publish-worker retries with workspace only for that bounded fallback path.

### Slice 26. Remove local upload materialized fallback entirely

Цель:

- after Slice 24, local zero-copy upload happy paths no longer create eager
  workspaces, but the generic upload shell still kept a final branch into
  `publishMaterializedUpload(...)` for local inputs;
- tighten the contract:
  once fail-fast validation has accepted a local upload, it must be handled by
  direct/streaming zero-copy fast paths, otherwise this is publishworker drift,
  not a reason to revive local materialization.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- local upload path no longer has any materialized fallback branch after
  fail-fast validation;
- only staged uploads can still fall into `publishMaterializedUpload(...)`,
  because they may need a bounded local preparation path when streaming/object
  reads are unavailable or not enough.

### Slice 27. Remove staged upload materialized fallback entirely

Цель:

- after Slice 26, the only remaining upload branch that could still fall into
  `publishMaterializedUpload(...)` was staged upload after local download;
- tighten the live contract to match upload admission and already-landed
  zero-copy paths:
  once a staged upload has either object-read fast paths or a downloaded local
  file, successful publish must still go through direct `GGUF` or streamable
  archive paths, not through resurrected checkpoint materialization.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- upload path no longer contains `publishMaterializedUpload(...)` as a live
  successful branch at all;
- staged download-only clients still publish through local zero-copy fast
  paths:
  direct `GGUF` or streamable archive;
- any upload path escaping those fast paths after validation is now treated as
  publishworker drift, not as justification to keep dead materialization code.

### Slice 28. Remove upload workspace shell from staged download-only publish

Цель:

- after Slice 27, staged `Download`-only uploads still needed one local temp
  source copy, but the shell around that copy still created a dedicated upload
  workspace directory first;
- tighten the remaining local-copy path too:
  keep the single downloaded source file, but remove the extra workspace shell
  and its empty `SnapshotDir` churn from successful staged publish.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- staged `Download`-only publish now downloads into one temporary source path,
  not into a dedicated upload workspace;
- the remaining byte path stays bounded:
  one local downloaded file, then direct `GGUF` or archive streaming publish;
- successful staged download-only publish no longer leaves `SnapshotDir`
  created just to host that temporary file.

### Slice 29. Remove staged download-only publish fallback entirely

Цель:

- after Slice 28, staged `Download`-only publish was already reduced to one
  local temp file, but this path still existed only to support staging clients
  without object-read contracts;
- live publication wiring already passes the S3 adapter, which exposes
  `Reader`/`RangeReader`, so keeping `Download`-only publish in the runtime
  shell became unnecessary fallback complexity;
- tighten the runtime contract:
  staged upload publication now requires object reads and fails fast when the
  staging client cannot expose them.

Файлы/каталоги:

- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- upload publish shell no longer contains any staged local-download fallback;
- staged uploads now have one live contract only:
  object-read streaming publish;
- download-only staged clients fail before download, before workspace, and
  before publisher invocation.

### Slice 30. Remove temp-download shell from source-mirror state store

Цель:

- even after Slice 29, source-mirror state/manifest loading still pulled JSON
  through `Download` into a temporary local file before decoding;
- this did not change model byte path, but it kept an avoidable temp-file
  shell and preserved an unnecessary `Downloader` dependency inside the mirror
  state store;
- tighten that boundary:
  source-mirror state/manifest loading should use `OpenRead` streaming decode
  directly from object storage.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcemirror/objectstore/*`
- `images/controller/internal/dataplane/publishworker/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcemirror/objectstore ./internal/dataplane/publishworker/...`
- `make verify`

Артефакт результата:

- mirror manifest/state JSON no longer downloads into a temp local file before
  decode;
- objectstore adapter now depends on `Reader`, not `Downloader`, for this
  state path;
- test proof no longer gives the fake mirror store a `Download(...)`
  implementation just to satisfy the round-trip contract.

### Slice 31. Remove the final `HF` local fallback and tighten streaming contracts

Цель:

- after Slice 30, `HF` publish still kept one remaining structural escape hatch:
  remote summary/object-source planning could degrade to bounded local
  materialization through `workspace/model`;
- finish that workstream decisively:
  direct and mirrored `HF` publication must now be streaming/object-source
  only, with explicit error on planning failure instead of hidden local retry;
- tighten surrounding contracts to match the already landed runtime semantics:
  publish-worker staging client requires streaming-capable reads, source-mirror
  transport no longer depends on broad downloader shell, and publish-worker CLI
  no longer advertises `--snapshot-dir` as a live contract.

Файлы/каталоги:

- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/ports/uploadstaging/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `plans/active/phase-reset-own-modelpack-and-remove-mlflow/*`
- `images/controller/TEST_EVIDENCE.ru.md`

Проверки:

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker/... ./internal/adapters/k8s/sourceworker`
- `make verify`

Артефакт результата:

- direct and mirrored `HF` publish no longer allocate or retry a local
  workspace at all;
- remote summary/object-source planning failures now fail explicitly instead of
  falling back into `workspace/model`;
- publish-worker staging contract is narrowed to streaming-capable object
  reads, uploads, deletes and prefix deletes;
- `publish-worker` no longer exposes `--snapshot-dir`, and source-worker no
  longer wires it as a runtime arg.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: новый narrative уже зафиксирован,
но runtime still unchanged.

После Slice 2 можно безопасно остановиться: native publisher contract defined,
но old runtime still present.

После Slice 3 rollback уже становится дорогим, поэтому Slice 3 is the first
true cutover point: it must land with working publication and without fallback.

## 5. Final validation

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
