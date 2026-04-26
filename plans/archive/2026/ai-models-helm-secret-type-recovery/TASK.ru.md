# ai-models Helm Secret type recovery

## Контекст

Deckhouse ModuleRun для `ai-models` падает на Helm upgrade:

`cannot patch "ai-models-controller-webhook-tls" with kind Secret:
Secret ... type: Invalid value: "kubernetes.io/tls": field is immutable`

Та же ошибка есть для `ai-models-dmcr-tls`. Это обычно означает, что в live
кластере остался Secret с другим `.type`, а текущий chart пытается привести его
к `kubernetes.io/tls`, что Kubernetes запрещает patch'ить.

## Постановка задачи

Восстановить rollout `ai-models` в live cluster без потери данных: проверить
текущие Secret, понять, являются ли они module-generated TLS материалом, и
безопасно удалить/пересоздать только несовместимые Secret, чтобы Helm upgrade
прошёл.

## Scope

- Проверить context и namespace.
- Проверить live Secret type, labels/annotations/ownerRefs и содержимое keys.
- Проверить templates в репозитории, чтобы понять expected Secret type.
- Если Secret module-generated и не содержит пользовательских данных, удалить
  только эти Secret и дать Deckhouse/Helm пересоздать их.
- Проверить ModuleRun/ModuleSource/Pod rollout после восстановления.
- Внести narrow code fix, чтобы такие legacy Secret удалялись автоматически
  перед Helm upgrade после сохранения TLS material во values.

## Non-goals

- Не удалять PVC, DMCR registry data, object storage credentials или runtime
  state.
- Не менять public API/RBAC/storage contract.
- Не менять TLS templates: текущий expected type `kubernetes.io/tls` остаётся
  целевым состоянием.

## Критерии приёмки

- Helm upgrade больше не падает на immutable Secret type для
  `ai-models-controller-webhook-tls` и `ai-models-dmcr-tls`.
- `d8-ai-models` pods стабилизировались или оставшиеся ошибки явно отделены от
  Secret type problem.
- Module hook автоматически удаляет только managed TLS Secret с non-TLS type и
  не трогает уже корректные `kubernetes.io/tls` Secret.

## Риски

- Удаление wrong Secret может сломать TLS endpoint до пересоздания.
- Если Secret генерируется не Helm hook'ом, а внешним controller'ом, нужно
  дождаться правильного владельца.
