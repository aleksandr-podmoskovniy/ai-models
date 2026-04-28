# Plan: honest model metadata contract

## 1. Current phase

Задача относится к Phase 1/2 boundary:

- Phase 1 уже требует корректного `Model` / `ClusterModel` status baseline;
- будущий Phase 2 и `ai-inference` будут потреблять этот status как input для
  runtime planning.

Сейчас нужен design-first slice без публичной schema migration.

## 2. Orchestration

Режим: `full`.

Причина: текущий slice меняет DKP-facing `Model` / `ClusterModel` API/status
projection и internal profile boundaries.

Read-only subagents перед implementation:

- `api_designer` — подтвердил, что `minimumLaunch`, `compatible*`,
  `framework` и weakly typed planner-like fields нельзя закреплять как
  публичный planner contract.
- `repo_architect` — подтвердил, что API package должен остаться schema
  boundary, а profilers не должны считать runtime policy.
- `integration_architect` — подтвердил, что публичное расширение status сразу
  расширяет RBAC exposure; текущий slice должен добавлять internal evidence,
  но не новые public `footprint/evidence` поля.

## 3. Active bundle disposition

- `publication-runtime-chaos-resilience` — archived to
  `plans/archive/2026/publication-runtime-chaos-resilience`; this metadata
  bundle no longer treats it as active work.
- `codebase-slimming-pass` — archived to
  `plans/archive/2026/codebase-slimming-pass`; metadata changes must not absorb
  follow-up slimming slices.
- `dmcr-gc-s3-consolidation` — archived to
  `plans/archive/2026/dmcr-gc-s3-consolidation`.
- `controller-backend-prefix-consolidation` — archived to
  `plans/archive/2026/controller-backend-prefix-consolidation`.
- `model-metadata-contract` — current. Новый workstream про metadata contract и
  будущий inference consumer semantics.

## 4. Slices

### Slice 1. Зафиксировать target metadata contract

Цель:

- описать честную модель metadata provenance;
- разделить public summary и evidence/provenance;
- добавить boundary для model-derived planning facts: модель описывает
  serving capabilities и resource factors, а не готовый runtime plan и не
  `acceleratorCount`;
- описать целевую структуру `application/profilecalc` как чистый use-case
  calculator между profilers, immutable snapshot и public status projection;
- записать consumer semantics для будущего `ai-inference`.

Файлы:

- `docs/development/MODEL_METADATA_CONTRACT.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-04-26-ai-models-metadata-profile.md`

Проверки:

- `git diff --check`

Артефакт:

- durable design, пригодный для реализации без повторного обсуждения базовых
  терминов.

### Slice 2. Public status shrink and internal confidence model

Цель:

- добавить internal `ProfileConfidence` / per-field signal model;
- научить profilers возвращать confidence alongside summary;
- сжать public `status.resolved` до стабильных model-derived facts;
- убрать из projection `minimumLaunch`, `compatibleRuntimes`,
  `compatibleAcceleratorVendors`, `compatiblePrecisions`, `framework`;
- не добавлять новые public `footprint/evidence/launchProfiles` поля.

Файлы:

- `api/core/v1alpha1/`
- `images/controller/internal/publishedsnapshot/`
- `images/controller/internal/adapters/modelprofile/`
- `images/controller/internal/dataplane/publishworker/`
- `images/controller/internal/domain/publishstate/`
- `images/controller/internal/monitoring/catalogmetrics/`
- `crds/`

Проверки:

- `cd api && go test ./...`
- targeted `go test` по profiler, publishstate и catalogmetrics packages.
- `api/scripts/update-codegen.sh`
- `api/scripts/verify-crdgen.sh`
- `make verify`

Артефакт:

- controller runtime отличает exact/derived/estimated/hint значения до status
  projection;
- public CRD не содержит planner-like launch fields;
- public status не выглядит как scheduler input.
- public status projection фильтрует unknown endpoint values до записи в
  enum-backed CRD status.

Validation evidence:

- `cd api && go test ./...` — pass.
- `cd images/controller && go test ./internal/domain/modelsource ./internal/ports/publishop ./internal/controllers/catalogstatus ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/uploadsession ./internal/monitoring/catalogmetrics ./internal/adapters/sourcefetch ./internal/dataplane/publishworker ./internal/publishedsnapshot ./internal/publicationartifact ./internal/adapters/modelprofile/... ./internal/domain/publishstate ./internal/application/publishobserve` — pass.
- `bash api/scripts/verify-crdgen.sh` — pass.
- `git diff --check` — pass.
- `make verify` — pass.
- Final `reviewer` pass found remote/archive footprint propagation and public
  format enum doc drift; both fixed, then targeted tests and `make verify`
  passed again.
- Additional hardening pass added enum-safe public format/endpoint projection
  and complete partial-confidence detection, then ran:
  `go test -count=5 ./internal/publishedsnapshot ./internal/domain/publishstate ./internal/adapters/sourcefetch ./internal/dataplane/publishworker ./internal/adapters/modelprofile/...`,
  `go test -race` for the same package set, `git diff --check`,
  `api/scripts/verify-crdgen.sh`, and `make verify` — pass.

### Slice 3. Status projection and conditions follow-up

Цель:

- сделать `MetadataResolved` message/reason честным для partial metadata;
- не писать low-confidence значения в hard-consumer fields без evidence;
- добавить tests на Safetensors exact path, GGUF hint path и unknown path.

Файлы:

- `images/controller/internal/domain/publishstate/`
- `api/core/v1alpha1/conditions.go`, только если нужен новый reason constant.

Проверки:

- targeted `go test ./images/controller/internal/domain/publishstate/...`

Артефакт:

- public status не выглядит как “всё вычислено точно”, когда часть полей
  оценочная или неизвестная.

### Slice 4. Public evidence API decision

Цель:

- только после consumer proof решить, нужен ли публичный
  `status.resolved.evidence` или достаточно internal evidence плюс condition
  message.

Файлы:

- `api/core/v1alpha1/`
- generated CRD/OpenAPI docs
- RBAC evidence, если schema/status contract меняется user-facing способом.

Проверки:

- `make helm-template`
- `make kubeconform`
- API/status focused tests.

Артефакт:

- explicit public API decision, а не accidental schema growth.

### Slice 5. Capability vocabulary and public feature summary

Цель:

- расширить `status.resolved.supportedEndpointTypes` до текущих model tasks:
  embeddings, rerank, STT, TTS, CV, image generation и multimodal;
- добавить отдельный `status.resolved.supportedFeatures` для modality/tool
  признаков, которые не являются endpoint: `VisionInput`, `AudioInput`,
  `AudioOutput`, `ImageOutput`, `MultiModalInput`, `ToolCalling`;
- трактовать HuggingFace `pipeline_tag` как source-declared metadata signal,
  а не weak hint;
- сохранить правило: public projection допускает только exact/derived/source-
  declared values, но не filename/bytes hints.

Файлы:

- `api/core/v1alpha1/`
- `crds/`
- `images/controller/internal/publishedsnapshot/`
- `images/controller/internal/adapters/modelprofile/`
- `images/controller/internal/adapters/sourcefetch/`
- `images/controller/internal/dataplane/publishworker/`
- `images/controller/internal/domain/publishstate/`
- `docs/development/MODEL_METADATA_CONTRACT.ru.md`

Проверки:

- `cd api && go test ./...`
- targeted `go test` for modelprofile/sourcefetch/publishworker/publishstate
- `bash api/scripts/update-codegen.sh`
- `bash api/scripts/verify-crdgen.sh`
- `git diff --check`

RBAC/exposure:

- поле добавляется только в `status.resolved`; существующие роли уже читают
  `Model` / `ClusterModel` status целиком. Нового доступа к Secret, runtime
  objects, logs или subresources нет.

Fixture matrix для последующего live smoke после выката:

- embeddings: `sentence-transformers/all-MiniLM-L6-v2`;
- rerank: `cross-encoder/ms-marco-MiniLM-L6-v2`;
- STT: `openai/whisper-tiny`;
- TTS: `facebook/mms-tts-eng`;
- CV: `google/vit-base-patch16-224`;
- image generation: `hf-internal-testing/tiny-stable-diffusion-pipe`
  expected supported only for Safetensors Diffusers-style layouts with
  `model_index.json` plus selected `.safetensors` weights;
- multimodal: `hf-internal-testing/tiny-random-LlavaForConditionalGeneration`;
- tool-calling: `Qwen/Qwen2.5-0.5B-Instruct` profile must expose
  text/chat endpoints now; `ToolCalling` requires tokenizer/chat-template
  evidence in a later slice, not architecture guessing.

## 5. Rollback point

Slice 1 можно безопасно откатить удалением нового task bundle, repo-local design
doc и ADR в `internal-docs`. Runtime/code/API не меняются.

## 6. Final validation

Для текущего design slice:

- `git diff --check`
- `rg -n "[ \t]+$" <changed markdown files>`

Для следующих implementation slices:

- targeted `go test`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`, когда schema/templates/runtime будут затронуты.

## 7. Slice status

- Slice 1 — done: repo-local metadata contract и ADR в `internal-docs`
  добавлены. Дополнено уточнение про `ResolvedPlanningProfile` /
  planning facts, planner algorithm and target `profilecalc` package
  structure.
- Slice 2 — done: public status shrink plus internal evidence model.
- Slice 3 — done: public projection omits low-confidence metadata and filters
  unknown enum-backed values before CRD status writes.
- Slice 4 — deferred until `ai-inference` has a concrete consumer proof for a
  public evidence field.
- Slice 5 — done: capability vocabulary, public `supportedFeatures`,
  source-declared task projection and tokenizer-template `ToolCalling`
  detection implemented.

## 8. Validation evidence

- `git diff --check` — passed.
- `git diff --cached --check` — passed.
- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check` — passed.
- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --cached --check` — passed.
- `rg -n "[ \t]+$" docs/development/MODEL_METADATA_CONTRACT.ru.md plans/active/model-metadata-contract/TASK.ru.md plans/active/model-metadata-contract/PLAN.ru.md /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-04-26-ai-models-metadata-profile.md /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-inference-service.md` — passed.
- Launch profile refinement — passed with the same whitespace/diff checks;
  no runtime/code/API schema changed.
- Review gate — no critical findings for this docs/design slice. Public CRD
  remains unchanged; `ResolvedPlanningProfile` is explicitly internal until
  an `ai-inference` consumer proves the public projection.
- Profile calculator structure refinement — passed. `application/profilecalc`
  зафиксирован как pure use-case boundary; runtime topology, Kubernetes API,
  compatibility matrix и public status writes остаются снаружи.
- Accelerator count correction — passed conceptually: `acceleratorCount` is
  removed from the target planning facts model and documented as a legacy hint,
  not a model-derived hard field.
- CRD consequence correction — passed conceptually: future public projection
  must expose footprint/serving-capability facts, not request-like launch
  profile fields.
- `ai-inference` ADR alignment — passed conceptually: planner input is now
  artifact/capabilities/footprint plus compatibility matrix and inventory;
  `acceleratorCount` is documented only as actual `InferenceService` launch
  result, not a model field.
- API/repo/integration subagent review — passed. Shared conclusion: do not add
  public `footprint/evidence/launchProfiles` now; first remove planner-like
  public fields and keep confidence internal.
- Capability vocabulary slice — passed:
  - `cd api && go test ./...`
  - targeted `go test` for `publishedsnapshot`, `modelprofile`, `sourcefetch`,
    `publishworker`, `publishstate`
  - `bash api/scripts/update-codegen.sh`
  - `bash api/scripts/verify-crdgen.sh`
  - `cd images/controller && go test ./...`
  - `cd images/dmcr && go test ./...`
  - `git diff --check`
  - `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check`
  - `make deadcode`
  - `make verify`
- Diffusers-style Safetensors layout follow-up — passed:
  `model_index.json` now satisfies the Safetensors profile config contract,
  HuggingFace direct object-source selection keeps nested Diffusers configs and
  weights, and source-declared `text-to-image` projects `ImageGeneration` /
  `ImageOutput` without adding runtime launch compatibility claims.
  Targeted checks: `go test -count=1 ./internal/adapters/modelformat
  ./internal/adapters/sourcefetch ./internal/dataplane/publishworker` from
  `images/controller`, plus `git diff --check` and repo-level `make verify`.
- Review gate — no critical findings. Residual risks are explicit:
  non-Safetensors Diffusers exports are still unsupported, and `ToolCalling` is
  a model/template feature, not proof that an inference runtime can wire MCP
  tools.
