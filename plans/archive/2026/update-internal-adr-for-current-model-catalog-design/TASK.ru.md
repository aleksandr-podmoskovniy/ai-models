# Update Internal ADR For Current Model Catalog Design

## 1. Контекст

Во внешнем репозитории `internal-docs` есть ADR:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

Сверка показала, что он уже расходится с текущим phase-2 design bundle и
реальным состоянием public contract в `ai-models`:

- ADR всё ещё описывает OCI-only `spec.artifact` first iteration;
- ADR опирается на inference-oriented поля `modelType`, `usagePolicy`,
  `launchPolicy`, `status.resolved.*`;
- текущий design bundle уже зафиксировал:
  - `spec.source={HuggingFace|Upload|OCIArtifact}`;
  - publication-oriented lifecycle;
  - backend-neutral `status.artifact`;
  - controller-owned delete cleanup semantics;
  - always-local runtime delivery как internal contract.

Пользователь просит обновить ADR под текущую цель. При этом upload semantics
должны мыслиться по аналогии с virtualization/DVCR: controller-owned upload
contract, staging, временные креды/доступ, verify/promote/cleanup. Но эти
детали не нужно буквально протаскивать в ADR как virtualization-specific
implementation.

## 2. Постановка задачи

Обновить ADR так, чтобы он:

- отражал текущий design bundle как source of truth;
- описывал `Model` / `ClusterModel` через source-oriented publication contract;
- фиксировал backend-neutral public status;
- отделял public API от internal managed backend;
- описывал canonical runtime delivery как controller-owned local materialization,
  не превращая это в inference API;
- описывал controlled delete cleanup semantics;
- не раскрывал implementation details upload path глубже, чем нужно для
  архитектурного решения.

## 3. Scope

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/*`
- reference only:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/docs/internal/dvcr_auth.md`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/core/v1alpha2/virtual_image.go`

## 4. Non-goals

- Не менять код `ai-models` в этом slice.
- Не переписывать ADR под DVCR terminology.
- Не детализировать pod-level materializer implementation, sidecars или конкретные
  PVC objects как final product contract.
- Не пытаться в этом же slice разрешить все будущие inference policy questions.

## 5. Критерии приёмки

- ADR больше не конфликтует с текущим design bundle по форме public API.
- ADR явно отделяет:
  - public contract;
  - internal managed backend;
  - runtime delivery internals.
- Upload описан как controller-owned workflow с staging/promote semantics, но без
  лишней backend- или virtualization-specific detail leakage.
- Delete cleanup и local materialization отражены как архитектурные решения.
- Результат review зафиксирован в task bundle.

## 6. Риски

- Если переписать ADR слишком implementation-heavy, он быстро устареет снова.
- Если оставить старые inference-oriented поля рядом с новым contract, документ
  станет внутренне противоречивым.
- Если в ADR прямо протащить DVCR-specific механику, это создаст ложную связку
  между `ai-models` и virtualization.
