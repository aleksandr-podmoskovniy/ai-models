# Honest metadata contract for model catalog

## 1. Заголовок

Спроектировать честный metadata contract для `status.resolved` и будущего
`ai-inference`.

## 2. Контекст

`internal-docs/2026-03-18-ai-models-catalog.md` уже описывает, что
`ai-models` должен считать технический профиль модели: task, framework, family,
architecture, format, parameterCount, quantization, endpoint types и
минимальные launch-требования.

Текущий код уже заполняет `status.resolved`, но смешивает разные типы сигналов:

- факты из model config;
- выводы из architecture/task mapping;
- оценки по размеру весов;
- подсказки из имени GGUF-файла;
- платформенные projections вроде endpoint types и minimumLaunch.

Для будущего `ai-inference` это недостаточно честный контракт: сервису надо
понимать, какие поля можно использовать как факт для admission/planning, а какие
являются только UX/capacity hint.

## 3. Постановка задачи

Зафиксировать целевую модель metadata provenance и consumer semantics так, чтобы
следующие code slices могли менять профилировщики, `publishedsnapshot`,
status/conditions и при необходимости публичную schema без догадок.

## 4. Scope

- Описать разделение `status.resolved` как краткой consumer summary и
  provenance/evidence как доказательной модели.
- Зафиксировать промежуточную model-derived `ResolvedPlanningProfile` /
  launch profiles boundary: модель описывает допустимые способы обслуживания,
  но не выбирает runtime topology.
- Классифицировать каждое поле metadata: exact/extracted, derived, estimated,
  projected, hint или unknown.
- Зафиксировать, что будущий `ai-inference` может использовать как hard input,
  а что только как мягкую подсказку.
- Описать порядок implementation slices без premature public API expansion.
- Синхронизировать ai-models repo docs с ADR/materials в `internal-docs`.

## 5. Non-goals

- Не добавлять user-authored knobs в `Model.spec`.
- Не проектировать полный `ai-inference` API.
- Не заполнять `compatibleRuntimes` как hard compatibility до появления
  отдельного engine compatibility registry.
- Не делать публичную CRD schema migration в этом design slice.
- Не считать filename-derived GGUF значения фактами.

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
  capabilities, required model/runtime features, footprint, accelerator class
  и evidence без выбора KubeRay/vLLM/MIG/MPS.
- Явно записано, что empty/unknown лучше, чем заполнение догадкой.
- Явно записано, что `minimumLaunch` и estimated `parameterCount` не являются
  scheduler guarantee.
- Явно записано, что public API expansion возможен только отдельным slice с
  schema/RBAC/status evidence.
- `git diff --check` проходит.

## 8. Риски

- Если сразу менять CRD, можно закрепить неудачную публичную форму evidence.
- Если оставить только summary без provenance, `ai-inference` начнёт принимать
  runtime decisions по догадкам.
- Если трактовать `compatibleRuntimes` как deny-list/allow-list уже сейчас,
  можно заблокировать рабочие модели без реального engine compatibility proof.
