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
- `codebase-slimming-pass` — keep. Отдельный executable workstream для
  package-boundary slimming; metadata changes must not absorb those slices.
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
  public evidence field. Current next executable slice is internal
  `profilecalc` implementation, not public schema growth.

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
