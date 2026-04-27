# Plan: honest model metadata contract

## 1. Current phase

Задача относится к Phase 1/2 boundary:

- Phase 1 уже требует корректного `Model` / `ClusterModel` status baseline;
- будущий Phase 2 и `ai-inference` будут потреблять этот status как input для
  runtime planning.

Сейчас нужен design-first slice без публичной schema migration.

## 2. Orchestration

Режим: `solo`.

Причина: текущий slice document/design-only и не меняет API/code. Для
следующего code/API slice потребуется отдельный review по `api_designer` /
`integration_architect`, если будет разрешена delegation.

## 3. Active bundle disposition

- `publication-runtime-chaos-resilience` — keep. Это отдельный live/e2e
  workstream, сейчас blocked на rollout image contract mismatch.
- `model-metadata-contract` — current. Новый workstream про metadata contract и
  будущий inference consumer semantics.

## 4. Slices

### Slice 1. Зафиксировать target metadata contract

Цель:

- описать честную модель metadata provenance;
- разделить public summary и evidence/provenance;
- добавить boundary для model-derived launch profiles: модель описывает
  допустимые способы обслуживания и требования, а не готовый runtime plan;
- записать consumer semantics для будущего `ai-inference`.

Файлы:

- `docs/development/MODEL_METADATA_CONTRACT.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-04-26-ai-models-metadata-profile.md`

Проверки:

- `git diff --check`

Артефакт:

- durable design, пригодный для реализации без повторного обсуждения базовых
  терминов.

### Slice 2. Internal evidence model

Цель:

- добавить internal `ResolvedProfileEvidence` / per-field signal model;
- научить profilers возвращать evidence alongside summary;
- не менять public CRD schema.

Файлы:

- `images/controller/internal/publishedsnapshot/`
- `images/controller/internal/adapters/modelprofile/`
- `images/controller/internal/dataplane/publishworker/`

Проверки:

- targeted `go test` по profiler packages и publishworker result shaping.

Артефакт:

- controller runtime может отличать exact/derived/estimated/projected/hint
  значения до status projection.

### Slice 3. Status projection and conditions

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
  launch profiles and planner algorithm.
- Slice 2 — next executable slice: internal evidence model without public API
  migration.

## 8. Validation evidence

- `git diff --check` — passed.
- `git diff --cached --check` — passed.
- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --check` — passed.
- `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs diff --cached --check` — passed.
- `rg -n "[ \t]+$" docs/development/MODEL_METADATA_CONTRACT.ru.md plans/active/model-metadata-contract/TASK.ru.md plans/active/model-metadata-contract/PLAN.ru.md /Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-04-26-ai-models-metadata-profile.md` — passed.
- Launch profile refinement — passed with the same whitespace/diff checks;
  no runtime/code/API schema changed.
- Review gate — no critical findings for this docs/design slice. Public CRD
  remains unchanged; `ResolvedPlanningProfile` is explicitly internal until
  an `ai-inference` consumer proves the public projection.
