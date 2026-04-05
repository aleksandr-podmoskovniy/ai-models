# Implement Phase-2 Model Catalog API Validation And Immutability

## 1. Контекст

В `api/` уже появился первый phase-2 baseline:

- отдельный public API module;
- `Model` и `ClusterModel` types;
- общий `spec` / `status` shape;
- conditions/reasons и reproducible deepcopy generation.

Следующий implementation slice по design bundle — validation/defaulting и
immutability для этих типов. Сейчас shape уже читается, но ещё не фиксирует:

- one-of semantics для `spec.source.*`;
- requiredness ключевых полей;
- default values для первых phase-2 happy paths;
- immutability границ artifact-producing полей;
- явные правила для `ClusterModel` access contract.

## 2. Постановка задачи

Реализовать schema-level validation/defaulting/immutability для
`Model` / `ClusterModel` в пределах `api/`:

- добавить `kubebuilder` validation/default markers на public types;
- зафиксировать one-of contract для `spec.source`;
- зафиксировать initial defaults там, где они уже определены design bundle;
- зафиксировать schema-level immutability для artifact-producing `spec` fields;
- добавить узкий verify path, который реально прогоняет CRD schema generation,
  даже если CRD файлы ещё не коммитятся в репозиторий.

## 3. Scope

- `api/core/v1alpha1/*`
- `api/scripts/*`
- `api/README.md`, если нужно уточнить generation/verification workflow
- `plans/active/implement-model-catalog-api-validation/*`

## 4. Non-goals

- Не реализовывать controller runtime под `images/controller/`.
- Не коммитить production CRD manifests в module templates.
- Не делать webhook binary или admission server.
- Не реализовывать registry access manager, upload lifecycle или publish jobs.
- Не добавлять сейчас backend sync contract в public status.

## 5. Затрагиваемые области

- `api/core/v1alpha1/*`
- `api/scripts/*`
- `plans/active/implement-model-catalog-api-validation/*`

Reference only:
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/*`

## 6. Критерии приёмки

- `spec.source` имеет явный one-of contract и requiredness ключевых полей.
- Initial defaults зафиксированы marker'ами только там, где design bundle уже
  даёт достаточно сигнала.
- Artifact-producing части `spec` стали immutable на schema level.
- `Model` и `ClusterModel` остаются semantically aligned, но `ClusterModel`
  получает stricter access semantics.
- В репозитории появился reproducible verify path для CRD schema generation.
- Узкие проверки проходят локально.

## 7. Риски

- Слишком агрессивная immutability может заблокировать нормальный reconcile flow
  ещё до появления контроллера.
- Слишком ранние defaults могут закрепить неудачные operational assumptions.
- Неправильные `XValidation` правила легко выглядят правдоподобно, но ломаются
  только на CRD generation или на update semantics.
