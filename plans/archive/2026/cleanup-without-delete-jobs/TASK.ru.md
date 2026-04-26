# Удалить per-delete cleanup Jobs из catalog cleanup

## Контекст

После удаления модели `catalogcleanup` создаёт отдельный Kubernetes `Job`
`ai-model-cleanup-*`, который запускает `artifact-cleanup` и только потом
запрашивает DMCR GC. Пользователь указал, что такой подход выглядит
сомнительно и может расходиться с паттерном `virtualization`.

Сравнение с `virtualization` показало: DVCR GC не создаёт отдельный Job на
каждую удаляемую сущность. Там есть controller-owned lifecycle, единый Secret
с состоянием GC и долгоживущий cleanup container внутри DVCR Deployment.
Provisioning/import операции при этом откладываются через состояние GC, а
результат сохраняется обратно в Secret/Deployment conditions.

## Постановка задачи

Заменить per-delete `cleanup Job` на controller-owned cleanup operation:
контроллер должен выполнять удаление publication artifact/staging сам через
внутренний порт, фиксировать completion marker в cleanup Secret и затем
запрашивать уже существующий DMCR GC lifecycle.

## Scope

- Убрать создание и наблюдение Kubernetes `Job` из `catalogcleanup`.
- Сохранить idempotency удаления через marker в controller-owned cleanup Secret.
- Переиспользовать существующую cleanup бизнес-логику `artifactcleanup`.
- Оставить DMCR GC как отдельный долгоживущий lifecycle в DMCR/runtime path.
- Обновить controller wiring/templates, если controller теперь должен иметь
  registry credentials/CA для in-process cleanup.
- Обновить targeted unit/template tests.

## Non-goals

- Не менять публичный API `Model` / `ClusterModel`.
- Не менять user-facing RBAC роли.
- Не перепроектировать DMCR GC container и его 10m delayed arm window в этой
  задаче.
- Не удалять deprecated CLI `artifact-cleanup`, если он ещё нужен как fallback
  или для ручной диагностики.

## Затрагиваемые области

- `images/controller/internal/controllers/catalogcleanup`
- `images/controller/internal/application/deletion`
- `images/controller/internal/adapters/k8s/cleanupstate`
- `images/controller/internal/dataplane/artifactcleanup`
- `images/controller/cmd/ai-models-controller`
- `templates/controller`
- `tools/helm-tests`

## Критерии приёмки

- Удаление модели больше не создаёт `batch/v1 Job ai-model-cleanup-*`.
- Cleanup completion хранится в controller-owned Secret и переживает replay.
- Повторный reconcile после успешного cleanup не повторяет registry/S3 delete.
- Transient cleanup error оставляет finalizer и даёт повторный reconcile.
- Backend artifact после in-process cleanup всё ещё ставит DMCR GC request.
- Успешная DMCR GC request state позволяет снять delete finalizer как раньше.
- В render output нет cleanup-job flags/serviceAccount/image для controller
  deletion path.

## Риски

- Controller должен безопасно получить DMCR write credentials и CA.
- In-process cleanup не должен блокировать reconcile надолго при зависшем
  registry/S3 запросе.
- Старые cleanup Secrets без completion marker должны корректно доехать через
  новый idempotent path.
