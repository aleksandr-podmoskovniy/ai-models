# Implement Phase-2 Upload Session Lifecycle

## 1. Контекст

В `ai-models` уже есть:

- public DKP API types `Model` / `ClusterModel`;
- validation/defaulting/immutability;
- controller-side pure libraries для registry path conventions и published access
  planning;
- design bundle, который фиксирует local upload pattern по аналогии с
  virtualization:
  - object переходит в `WaitForUpload`;
  - controller выдаёт staging upload contract;
  - user загружает `ModelKit` в staging repo;
  - controller затем verify/promote/cleanup.

Следующий implementation order по design bundle — upload session lifecycle.

Сейчас в public API уже есть `status.upload.expiresAt` и `status.upload.command`,
но нет staging repository в status, а в controller code ещё нет bounded library
для planning upload session, upload TTL и publisher-facing staging grant intent.

## 2. Постановка задачи

Реализовать следующий bounded slice phase 2:

- довести public `status.upload` до agreed contract shape;
- добавить controller-side pure library для planning upload session lifecycle;
- добавить payload-registry-oriented rendering для temporary staging upload
  grants;
- зафиксировать public/upload command semantics и tests так, чтобы следующий
  runtime slice мог поверх этого добавлять live reconcile/materialization.

## 3. Scope

- `api/core/v1alpha1/*`
- `api/README.md`, `api/scripts/*`, если потребуется verify wiring
- `images/controller/internal/*`
- `images/controller/cmd/*`, только если нужен minimal config plumbing
- `plans/active/implement-model-catalog-upload-session-lifecycle/*`

Внутри slice:

- `status.upload.repository` в public API;
- upload-session planning package under `images/controller/internal/*`;
- command builder для user-facing helper command;
- staging grant planning / rendering для publisher subject и staging repo;
- unit tests на lifecycle semantics, TTL, command, subject rendering и
  object-to-target ownership guards.

## 4. Non-goals

- Не делать live reconciler и watches.
- Не materialize'ить реальные `Role`, `RoleBinding` или `PayloadRepositoryAccess`
  в кластере.
- Не делать detection uploaded artifact в registry.
- Не делать promote worker, metadata inspection или final publication.
- Не реализовывать actual CLI helper binary `d8 ai-models model upload`.
- Не делать browser upload или data-plane proxy через controller.

## 5. Затрагиваемые области

- `api/core/v1alpha1/*`
- `images/controller/*`
- `plans/active/implement-model-catalog-upload-session-lifecycle/*`

Reference only:

- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/docs/internal/dvcr_auth.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/core/v1alpha2/virtual_image.go`

## 6. Критерии приёмки

- Public API status для upload теперь содержит staging repository ref наряду с
  TTL и command.
- Для `source.upload` есть testable controller-side planning library, которая:
  - строит staging target из object identity и UID;
  - рассчитывает expiry;
  - переводит объект в `WaitForUpload`;
  - выдаёт user-facing command;
  - планирует temporary publisher grant только на staging repo.
- Publisher subject model не путается с consumer access policy:
  - upload grant можно привязать к `User`, `Group` или `ServiceAccount`;
  - consumer defaults/explicit access остаются в existing access-planning path.
- Slice остаётся в pure planning / projection boundary без live cluster writes.
- Узкие проверки проходят.

## 7. Риски

- Если смешать publisher upload grant и consumer read access в одной модели,
  дальше controller boundaries быстро размоются.
- Если сейчас выдать неверный public `status.upload` shape, потом придётся
  ломать helper UX или делать compatibility shim.
- Если upload session planner сразу полезет в live RBAC materialization, slice
  перепрыгнет через runtime/reconcile этап и станет трудно reviewable.
