# Define Model Status And Runtime Delivery Target Shape

## 1. Контекст

Для `Model` / `ClusterModel` уже выровнена базовая идея в ADR:

- пользователь задаёт `spec.source`;
- модуль сохраняет модель в managed backend path или OCI distribution path;
- контроллер публикует stable artifact reference и metadata/profile;
- runtime consumer materializes модель в локальный PVC/shared volume.

Теперь нужно перевести это в конкретный target shape для:

- публичного `status` у `Model` / `ClusterModel`;
- internal controller contracts;
- internal agent/materializer contracts для runtime delivery.

Отдельно важно удержать правильную границу по auth/access:

- OCI / `payload-registry` должны жить в обычной Kubernetes/RBAC-like модели;
- S3-compatible path не должен вытаскивать в public contract долгоживущие creds;
- runtime container должен получать только локальный путь к модели, а не storage
  plumbing.

## 2. Постановка задачи

Сформулировать и зафиксировать concrete target shape для:

- `ModelStatus` / `ClusterModelStatus`;
- artifact status block;
- upload/publication/ready/delete lifecycle;
- runtime delivery plan между controller и materialization agent;
- auth boundary для OCI и S3 delivery classes.

Нужно получить shape, который потом можно прямо переносить в `api/` и
`images/controller/*` implementation slices без повторного угадывания.

## 3. Scope

- текущий `api/` и generated CRD shape;
- `images/controller/internal/*` contracts around publication/runtime delivery;
- актуальный ADR в `internal-docs`;
- bundle under `plans/active/define-model-status-and-runtime-delivery-target-shape/*`.

## 4. Non-goals

- Не менять код `api/` и controller в этом slice.
- Не materialize'ить новый runtime agent implementation.
- Не проектировать полную pod mutation story.
- Не фиксировать финальные security-hardening details вроде signature
  verification.

## 5. Затрагиваемые области

- `api/core/v1alpha1/*`
- `crds/*`
- `images/controller/internal/publication/*`
- `images/controller/internal/runtimedelivery/*`
- `images/controller/internal/managedbackend/*`
- `plans/active/define-model-status-and-runtime-delivery-target-shape/*`

## 6. Критерии приёмки

- Есть конкретный proposed shape для `status` у `Model` / `ClusterModel`.
- Есть конкретный proposed internal contract between controller and runtime
  materializer.
- Для OCI и S3 delivery classes отдельно описаны auth/access rules.
- Shape согласован с virtualization-like `source -> target/status` pattern.
- Shape не тащит raw backend entities в public API.

## 7. Риски

- Перепутать public contract со служебными internal delivery details.
- Оставить status слишком абстрактным и непригодным для прямой реализации.
- Зафиксировать слишком ранно pod wiring детали, которые должны остаться
  implementation choice.
