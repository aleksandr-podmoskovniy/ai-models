# Plan

## Current phase

Операционное восстановление runtime/deployment baseline.

## Orchestration

`solo`: узкая live recovery задача; главный риск управляется inspection перед
удалением Secret.

## Slices

1. Inspect
   - Проверить kube context.
   - Найти namespace `ai-models`.
   - Проверить Secret metadata/type/data keys.
   - Проверить chart templates for expected type.

2. Recover
   - Если Secret является module-owned TLS Secret и не содержит уникальные
     пользовательские данные, удалить только несовместимые Secret.
   - Дождаться повторного Helm/Deckhouse reconcile.

3. Validate
   - Проверить ModuleRun/Module health.
   - Проверить pods, restarts, events.
   - Зафиксировать permanent finding, если chart менял immutable Secret type.

4. Systemic fix
   - Добавить OnBeforeHelm hook после common TLS hooks.
   - Удалять только `ai-models-controller-webhook-tls` и `ai-models-dmcr-tls`,
     если live type не `kubernetes.io/tls`.
   - Проверить тестами preserve/delete branches.

## Rollback point

До удаления Secret изменений нет. После удаления rollback — дождаться
пересоздания Secret текущим chart или восстановить из pre-delete yaml dump.

## Final validation

- `kubectl get secret` показывает `.type: kubernetes.io/tls`.
- ModuleRun/Helm error исчез.
- Pods в `d8-ai-models` не продолжают rollout-loop из-за той же причины.
- `cd images/hooks && go test ./pkg/hooks/tls_secret_type_migration`.
