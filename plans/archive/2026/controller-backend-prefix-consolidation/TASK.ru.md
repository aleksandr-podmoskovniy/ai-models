# Controller backend prefix consolidation

## 1. Заголовок

Схлопнуть дублирование расчёта DMCR repository metadata prefix между
controller publish и artifact cleanup paths без изменения cleanup handle schema.

## 2. Контекст

После DMCR GC S3 consolidation следующий безопасный codebase-slimming slice
нашёл одинаковую backend-specific строковую логику в controller dataplane:
publish path записывает `RepositoryMetadataPrefix`, а cleanup path при старых
handles восстанавливает тот же prefix из `Backend.Reference`.

Это не публичный API и не DMCR GC policy. Это controller-local backend artifact
layout helper, который должен остаться внутри `images/controller/internal`.

## 3. Постановка задачи

- Вынести чистое преобразование OCI reference -> backend repository metadata
  prefix в узкий controller-local dataplane package.
- Сохранить publish behavior: новые cleanup handles продолжают получать
  `RepositoryMetadataPrefix`.
- Сохранить cleanup compatibility: если stored prefix есть, он побеждает; если
  его нет, cleanup derives prefix from `Backend.Reference`.
- Не менять cleanup handle schema, DMCR object layout, status/API/RBAC,
  templates или runtime entrypoints.

## 4. Scope

- `images/controller/internal/dataplane/backendprefix/`
- `images/controller/internal/dataplane/publishworker/support.go`
- `images/controller/internal/dataplane/artifactcleanup/backend_prefix.go`
- adjacent package-local tests.
- `plans/active/controller-backend-prefix-consolidation/`

## 5. Non-goals

- Не переносить backend layout helper в `support/cleanuphandle`.
- Не переиспользовать `modelpack/oci` reference parser для cleanup object path.
- Не создавать shared cross-image package между controller и DMCR.
- Не менять persisted cleanup handle fields.
- Не менять public `Model` / `ClusterModel` API, RBAC или Helm templates.

## 6. Критерии приёмки

- Prefix derivation живёт в одном controller-local package.
- Registry-less references fail closed and do not produce S3/DMCR prefix.
- Publish path still writes repository metadata prefix.
- Cleanup path prefers stored prefix and falls back to derived prefix only when
  needed.
- Все изменённые файлы остаются LOC<350.
- Targeted controller dataplane tests pass.
- Race tests for touched packages pass.
- `git diff --check` passes.
- `make verify` passes if repository state allows full verification.

## 7. Риски

- Слишком общий helper может превратиться в новый backend abstraction без
  ownership.
- Ошибка parsing может удалить неправильный DMCR metadata subtree.
- Удаление fallback может сломать cleanup старых handles после upgrade.
