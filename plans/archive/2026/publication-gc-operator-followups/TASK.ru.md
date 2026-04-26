# TASK: Publication / GC / RBAC operator follow-ups

## Контекст

После E2E проверки обновленного `ai-models` остались operator-facing хвосты:

- transient `DMCR 503 UNAVAILABLE` во время GC read-only окна переводит publication в terminal `Failed`;
- успешные короткоживущие publish worker pods удаляются быстрее, чем оператор успевает снять `kubectl logs`;
- delete/GC выглядит как непонятный хвост `dmcr-gc-*`, хотя фактически есть queued окно около 10 минут, затем armed/GC/done;
- `rbacv2/use` агрегирует `ClusterModel`, но namespaced `RoleBinding` не дает доступа к cluster-scoped объектам.

## Цель

Сделать поведение безопаснее и понятнее без расширения пользовательского API:

- publication retries transient DMCR maintenance/read-only responses;
- успешный publish pod остается доступным для ручного снятия логов после завершения;
- GC request явно маркирует queued/armed lifecycle;
- `rbacv2/use` остается namespaced persona, а `ClusterModel` остается только в manage/cluster-oriented access path.

## Non-goals

- Не переделывать source worker в Kubernetes Job.
- Не менять публичный `Model` / `ClusterModel` spec/status contract.
- Не добавлять новые user-facing RBAC уровни.
- Не менять DMCR storage/GC алгоритм удаления blobs.

## Acceptance criteria

- `503 Service Unavailable` от DMCR direct-upload API ретраится с backoff и не становится terminal failure сразу.
- Completed successful publish pod не удаляется cleanup handle, при этом projected auth/registry secrets удаляются.
- `dmcr-gc-*` Secret создается с queued marker и при arming получает armed marker.
- `templates/rbacv2/use/*` не обещают `ClusterModel` через namespaced `RoleBinding`.
- Узкие тесты и render checks проходят.

## RBAC coverage evidence

- `rbacv2/use`: namespaced `Model` read/use only; `ClusterModel` intentionally denied because RoleBinding cannot grant cluster-scoped resources.
- `rbacv2/manage`: keeps `Model` and `ClusterModel` management path.
- No module-local access is added to `Secret`, `pods/log`, `pods/exec`, `pods/attach`, `pods/portforward`, `status`, `finalizers` or internal runtime objects.
