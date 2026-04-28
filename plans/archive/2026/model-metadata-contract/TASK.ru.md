# Honest metadata contract for model catalog

## 1. Заголовок

Спроектировать честный metadata contract для `status.resolved` и будущего
`ai-inference`.

## 2. Контекст

`internal-docs/2026-03-18-ai-models-catalog.md` уже описывает, что
`ai-models` должен считать технический профиль модели: task, family,
architecture, format, parameterCount, quantization, endpoint types и
внутренние resource factors без публичных launch-требований.

Текущий код уже заполняет `status.resolved`, но смешивает разные типы сигналов:

- факты из model config;
- выводы из architecture/task mapping;
- оценки по размеру весов;
- подсказки из имени GGUF-файла;
- платформенные projections вроде endpoint types;
- устаревшие launch/runtime hints, которые выглядели как scheduler contract.

Для будущего `ai-inference` это недостаточно честный контракт: сервису надо
понимать, какие поля можно использовать как факт для admission/planning, а какие
являются только UX/capacity hint.

## 3. Постановка задачи

Зафиксировать целевую модель metadata provenance и consumer semantics так, чтобы
следующие code slices могли менять профилировщики, `publishedsnapshot`,
status/conditions и при необходимости публичную schema без догадок.

Текущий implementation slice дополнительно должен убрать из публичного
`Model` / `ClusterModel` status поля, которые выглядят как готовый launch plan
или runtime compatibility proof, но реально считаются эвристиками.

## 4. Scope

- Описать разделение `status.resolved` как краткой consumer summary и
  provenance/evidence как доказательной модели.
- Зафиксировать промежуточную model-derived `ResolvedPlanningProfile` /
  planning facts boundary: модель описывает serving capabilities и resource
  factors, но не выбирает runtime topology и не считает `acceleratorCount`.
- Зафиксировать целевую структуру расчётного модуля `application/profilecalc`
  и его границу с `adapters/modelprofile`, `publishedsnapshot`,
  `domain/publishstate` и будущим `ai-inference`.
- Классифицировать каждое поле metadata: exact/extracted, derived, estimated,
  projected, hint или unknown.
- Зафиксировать, что будущий `ai-inference` может использовать как hard input,
  а что только как мягкую подсказку.
- Описать порядок implementation slices без premature public API expansion.
- Синхронизировать ai-models repo docs с ADR/materials в `internal-docs`.
- Сжать публичный `status.resolved` до стабильных model-derived facts и
  endpoint capabilities.
- Расширить публичную capability summary так, чтобы профиль мог честно
  подсвечивать embedding, rerank, STT, TTS, CV, image generation, multimodal и
  tool-calling модели без runtime launch hints.
- Перенести confidence/provenance в internal snapshot/profile boundary, а не в
  публичную CRD.
- Убрать из live projection `minimumLaunch`, `compatibleRuntimes`,
  `compatibleAcceleratorVendors`, `compatiblePrecisions` и `framework`.
- Не эмитить `Validated=True`, пока нет реальной policy validation.

## 5. Non-goals

- Не добавлять user-authored knobs в `Model.spec`.
- Не проектировать полный `ai-inference` API.
- Не заполнять `compatibleRuntimes` как hard compatibility до появления
  отдельного engine compatibility registry.
- Не добавлять новые публичные `status.resolved.footprint`,
  `status.resolved.evidence` или `status.resolved.launchProfiles` поля без
  отдельного consumer-proof API slice.
- Не проектировать full compatibility matrix для `ai-inference`.
- Не считать filename-derived GGUF значения фактами.
- Не называть модель "MCP-compatible": MCP остаётся capability будущего
  inference host/runtime. Каталог может показать только model/tool-calling
  feature, если есть надёжный сигнал.

## 6. Затрагиваемые области

- `plans/active/model-metadata-contract/`
- `docs/development/MODEL_METADATA_CONTRACT.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/`
- Последующие implementation slices затронут:
  - `api/core/v1alpha1/`
  - `images/controller/internal/publishedsnapshot/`
  - `images/controller/internal/adapters/modelprofile/`
  - `images/controller/internal/domain/publishstate/`
  - tests around profile/status projection.

## 7. Критерии приёмки

- Есть durable repo-local metadata contract, не зависящий от chat context.
- Есть ADR/design note в `internal-docs`, связывающий `ai-models` metadata с
  будущим `ai-inference`.
- Для каждого поля `status.resolved` описан источник, уровень честности и
  consumer semantics.
- Описано, как модель говорит "меня можно запускать так": endpoint
  capabilities, required model/runtime features, footprint и evidence без
  выбора KubeRay/vLLM/MIG/MPS и без `acceleratorCount`.
- Описана конкретная internal package structure будущего calculator без
  публичной CRD migration и без смешения с scheduler/runtime planner.
- Явно записано, что empty/unknown лучше, чем заполнение догадкой.
- Явно записано, что launch/runtime hints и estimated `parameterCount` не
  являются scheduler guarantee.
- Явно записано, что public API expansion возможен только отдельным slice с
  schema/RBAC/status evidence.
- Публичная CRD больше не содержит scheduler/request-like поля под
  `status.resolved`.
- Публичный status не выдаёт low-confidence GGUF filename hints за facts.
- Публичный status показывает endpoint types и cross-cutting features отдельно:
  API-тип вызова не смешивается с input/output modality или tool-calling.
- HuggingFace `pipeline_tag` используется как source-declared metadata signal,
  а не как weak filename-style hint.
- Есть fixture matrix для маленьких моделей: embeddings, rerank, STT, TTS, CV,
  image generation, multimodal, tool-calling.
- Conditions различают полный и частичный metadata profile без runtime-specific
  или high-cardinality reasons.
- `git diff --check` проходит.

## 8. Риски

- Если сразу менять CRD, можно закрепить неудачную публичную форму evidence.
- Если оставить только summary без provenance, `ai-inference` начнёт принимать
  runtime decisions по догадкам.
- Если трактовать `compatibleRuntimes` как deny-list/allow-list уже сейчас,
  можно заблокировать рабочие модели без реального engine compatibility proof.
