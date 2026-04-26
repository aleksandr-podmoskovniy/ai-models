# PLAN: Publication / GC / RBAC operator follow-ups

## Orchestration

Mode: `solo`.

Причина: текущий slice узкий и вытекает из уже собранного E2E evidence. Делегация не используется в этом turn, потому что пользователь не запросил subagents явно в текущей задаче.

## Slices

### 1. Direct-upload retry

Файлы:

- `images/controller/internal/adapters/modelpack/oci/direct_upload_client.go`
- `images/controller/internal/adapters/modelpack/oci/*_test.go`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci`

Rollback:

- Удалить retry loop и тесты, вернуть одиночный HTTP call.

### 2. Publish worker log retention

Файлы:

- `images/controller/internal/adapters/k8s/sourceworker/service.go`
- `images/controller/internal/adapters/k8s/sourceworker/*_test.go`

Решение:

- Cleanup handle удаляет projected secrets, но сохраняет `Succeeded` pod как log retention artifact.
- Failed/running pods удаляются по старому пути, чтобы не ломать retry/recreate semantics.

Проверки:

- `cd images/controller && go test ./internal/adapters/k8s/sourceworker`

Rollback:

- Вернуть удаление pod независимо от phase.

### 3. Delete/GC UX markers

Файлы:

- `images/controller/internal/controllers/catalogcleanup/*`
- `images/dmcr/internal/garbagecollection/*`

Решение:

- Controller создает GC request с phase marker `Queued`.
- DMCR при arming переводит marker в `Armed`.
- Done остается видимым через исчезновение request и DMCR logs.

Проверки:

- `cd images/controller && go test ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./internal/garbagecollection`

Rollback:

- Удалить lifecycle annotations и вернуть только существующий switch marker.

### 4. RBAC v2 use boundary

Файлы:

- `templates/rbacv2/use/view.yaml`
- `templates/rbacv2/use/edit.yaml`

Решение:

- Убрать `ClusterModel` из namespaced `use` aggregation.
- Оставить `ClusterModel` в `manage` и cluster persona paths.
- Evidence: Deckhouse RBAC experimental docs state `use` roles are for namespaced user resources and explicitly do not grant cluster-wide resources through `RoleBinding`; `manage` roles are the cluster-wide/module configuration path.

Проверки:

- `make helm-template`
- `make kubeconform`

## Final checks

- `git diff --check`
- `make verify` если repo-level verify остается выполнимым после узких проверок.
