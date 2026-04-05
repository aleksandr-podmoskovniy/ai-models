# Review CRD Against Internal ADR

## 1. Контекст

В репозитории уже есть текущий phase-2 public contract для `Model` /
`ClusterModel` и несколько implementation slices вокруг:

- artifact status;
- upload lifecycle;
- managed backend contract;
- runtime delivery intent;
- delete cleanup semantics.

Пользователь просит сверить текущий CRD contract c ADR из соседнего
репозитория:

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

Нужно проверить, что текущая форма CRD и связанные controller semantics не
разошлись с ADR, и явно перечислить совпадения, расхождения и открытые вопросы.

## 2. Постановка задачи

Сделать read-only review, который:

- извлекает из ADR канонические архитектурные требования к `Model` /
  `ClusterModel`;
- сравнивает их с текущим CRD contract и ближайшими controller-side semantics;
- фиксирует конкретные расхождения без размытого summary;
- отделяет already-implemented behavior от planned/not-yet-wired slices.

## 3. Scope

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
- `api/core/v1alpha1/*`
- `images/controller/internal/{publication,runtimedelivery,cleanuphandle,cleanupjob,modelcleanup,managedbackend}/*`
- `plans/active/implement-backend-specific-artifact-location-and-delete-cleanup/*`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `plans/active/review-crd-against-internal-adr/*`

## 4. Non-goals

- Не менять API types или controller code в этом review slice.
- Не переписывать ADR.
- Не открывать новый implementation slice по каждому найденному расхождению в
  этой же задаче.

## 5. Критерии приёмки

- Ясно перечислены совпадения ADR и текущего CRD contract.
- Ясно перечислены расхождения ADR и текущего состояния репозитория.
- Для каждого существенного расхождения указано, это:
  - intentional shift;
  - implementation gap;
  - ambiguous/open question.
- Результат сохранён в review bundle и пригоден как вход в следующий
  implementation slice.

## 6. Риски

- Если сверять только `api/*`, можно пропустить drift между public contract и
  фактическими controller semantics.
- Если смешать planned bundle statements и реально работающий code path,
  получится ложная оценка готовности.
