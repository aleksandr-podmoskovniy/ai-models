# Phase 2: align model metadata contract with internal ADR semantics

## Контекст

Текущий public contract в `ai-models` смешивает разные semantic layers:

- `spec.launchPolicy.allowedRuntimes` и `status.resolved.compatibleRuntimes`
  используют значения `KServe` / `KubeRay`, хотя это topology/orchestration
  choices, а не inference runtime implementations;
- `status.resolved.supportedEndpointTypes` публикует transport-facing значения
  вроде `OpenAIChatCompletions`, тогда как platform contract и `ai-inference`
  ADR живут в semantic endpoint types `Chat`, `TextGeneration`, `Embeddings`,
  `Rerank`, `SpeechToText`, `Translation`;
- `spec.runtimeHints.engines` используется как публичный источник
  `compatibleRuntimes`, хотя это не рассчитанная metadata модели, а ручной
  hint/override.
- `status.conditions` и delete-time status сейчас тоже расползлись:
  API содержит `Accepted`, `UploadReady`, `ArtifactPublished`,
  `MetadataReady`, `Validated`, `CleanupCompleted`, `Ready`, хотя ADR задаёт
  только короткий набор базовых platform-facing conditions;
- `CleanupCompleted` живёт отдельно от основного publish lifecycle и делает
  delete-path визуально heavier, чем нужно для public contract;
- `Ready` сейчас местами дублирует failure reason конкретного substage вместо
  того, чтобы оставаться итоговым usability signal.

Внутренние ADR в
`/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
и
`/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-inference-service.md`
задают другую и более устойчивую рамку:

- `ai-models` публикует platform-facing semantic metadata о модели;
- `ai-inference` отдельно выбирает и запускает конкретный inference runtime;
- runtime implementation examples там: `vLLM`, `Triton`, `Ollama`, `SGLang`,
  `Ray/vLLM`;
- `KubeRay` относится к distributed launch topology, а не к типу модели.

Нужно привести текущий repo contract к этой рамке, не вытаскивая внутренние
backend/runtime детали в public API.

## Постановка задачи

Переделать phase-2 metadata/public contract так, чтобы:

- `supportedEndpointTypes` стали platform semantic endpoint types;
- runtime compatibility перестала описывать `KServe` / `KubeRay` и была либо
  inference-runtime facing, либо отсутствовала там, где controller не может
  честно её определить;
- `spec.runtimeHints` перестал нести публичный runtime-engine contract и
  остался только для live publication-time hints, которые реально нужны
  controller'у сейчас.
- `status.conditions` и `status.phase` остались короткими, explainable и
  согласованными с ADR:
  минимальный набор condition types без decorative controller-internal stages.
- platform-wide naming surface вокруг AI API был выровнен под единый prefix
  `ai.deckhouse.io`, а не `ai-models.deckhouse.io`, чтобы `Model`,
  будущие inference resources и higher-level AI services жили под одной
  API/annotation namespace и различались по `kind`, а не по отдельным group
  prefixes.

## Scope

- обновить `Model` / `ClusterModel` API types вокруг runtime/endpoint metadata;
- переименовать public API group, generated CRD names и связанные platform
  prefixes c `ai-models.deckhouse.io` на `ai.deckhouse.io`;
- обновить controller profile resolution, publication snapshot и status
  projection;
- упростить public condition/status contract вокруг publish/delete lifecycle;
- обновить policy validation и current docs/test evidence;
- выровнять repo wording с internal ADR semantics по runtime vs topology и
  unified AI API naming.

## Non-goals

- не проектировать весь `ai-inference` API в этом репозитории;
- не добавлять сейчас новые runtime engines или реальный inference scheduler;
- не переписывать весь taxonomy model types beyond the minimum needed for this
  contract cleanup;
- не смешивать этот workstream с предыдущим DMCR GC slice.

## Затрагиваемые области

- `api/core/v1alpha1/*`
- `api/core/*`
- `api/scripts/*`
- `crds/*`
- `templates/controller/*`
- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/publishedsnapshot/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/modelprofile/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- `images/controller/internal/support/*`
- `api/README.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

## Критерии приёмки

- `status.resolved.supportedEndpointTypes` использует platform semantic values,
  согласованные с `ai-inference` ADR и `spec.usagePolicy.allowedEndpointTypes`;
- `spec.launchPolicy.allowedRuntimes` / `preferredRuntime` больше не используют
  topology terms `KServe` / `KubeRay`;
- controller больше не строит `compatibleRuntimes` из
  `spec.runtimeHints.engines`;
- `spec.runtimeHints` содержит только hints, реально нужные publication path
  сейчас;
- `status.resolved.compatibleRuntimes` не заполняется фиктивными значениями в
  тех случаях, где controller не может defendably вывести совместимость;
- public `status.conditions` сведён к минимальному набору базовых
  platform-facing conditions без `Accepted`, `UploadReady` и
  `CleanupCompleted`;
- `Ready` остаётся итоговым usability signal, а не second copy reason для
  каждого внутреннего шага;
- delete path не требует отдельного cleanup-only public condition и остаётся
  объяснимым через `phase=Deleting` плюс итоговую `Ready` semantics;
- policy validation остаётся согласованной с новым public contract и не ломает
  `Model` / `ClusterModel` lifecycle;
- docs и test evidence объясняют новый split:
  model metadata vs inference runtime vs distributed topology.
- `Model` / `ClusterModel` CRD group, generated CRD names, controller RBAC and
  repo docs используют единый platform prefix `ai.deckhouse.io`;
- controller-owned annotations, labels, finalizers и helper constants больше не
  держат старый `ai-models.deckhouse.io` prefix там, где они являются частью
  platform-facing naming surface;
- generated artifacts (`crds/*`, codegen verify scripts) согласованы с новым
  API group и не требуют ручного patching после regenerate;
- ADR и repo docs больше не расходятся с live code по API group, source
  contract и minimal status model.

### Architecture acceptance criteria

- inference runtime brands и distributed topology не смешаны в одном enum;
- public `spec/status` не зависят от backend-private or orchestration-private
  implementation details;
- controller adapters остаются разделены по use-case / port / adapter, без
  wrapper-on-wrapper refactor;
- changes stay bounded to metadata/public contract and do not spill into DMCR,
  backend, or workload delivery runtime behavior.
- API group rename делается как один platform naming decision, а не
  patchwork-set of local aliases.

## Риски

- можно сломать backward compatibility тестов и status projection;
- можно оставить hybrid cluster/runtime surface, где часть code/docs/rbac/crds
  уже на `ai.deckhouse.io`, а часть всё ещё на `ai-models.deckhouse.io`;
- можно оставить полурабочий hybrid contract, где часть кода живёт в старых
  `KServe/KubeRay`, а часть уже в `VLLM/...`;
- можно переусложнить runtime compatibility logic без реального source of
  truth и снова получить guessed metadata вместо defendable metadata.
