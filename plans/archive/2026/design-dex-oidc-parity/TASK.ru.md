# TASK: определить путь к Dex/OIDC parity с n8n

## Контекст

Сейчас `ai-models` закрывает UI через `DexAuthenticator` и ingress-level auth, но не
имеет app-native OIDC/Dex integration уровня `n8n-d8`, где приложение умеет
работать как OIDC client against Deckhouse Dex и выполняет bootstrap/provisioning.

Нужно понять, можно ли получить такой же operational result для phase-1 backend
на базе upstream `MLflow` без неаккуратных костылей и без нарушения границы
между модулем и внутренним backend engine.

## Постановка задачи

Проверить текущий auth path `ai-models`, сравнить его с `n8n-d8` и upstream
`MLflow`, затем определить корректный технический путь к parity:

- либо подтвердить, что parity достижим чистой DKP-обвязкой поверх upstream
  `MLflow`;
- либо зафиксировать, что `MLflow` не даёт app-native OIDC parity без
  осознанного отдельного integration layer/patching.

## Scope

- Анализ текущего `DexAuthenticator`/ingress auth path в `ai-models`.
- Сравнение с `n8n-d8` OIDC/Dex bootstrap path.
- Анализ upstream `MLflow` auth/OIDC/security surface.
- Формулировка рекомендуемого implementation path для `ai-models`.
- Если по ходу анализа становится очевиден небольшой repo-side diagnostic gap,
  допускается закрыть его в этом же slice.

## Non-goals

- Полная реализация app-native OIDC parity в этом же bundle.
- Проектирование phase-2 `Model` / `ClusterModel`.
- Изменение внешнего cluster-wide `user-authn` ownership model.

## Затрагиваемые области

- `plans/active/design-dex-oidc-parity/*`
- при необходимости `docs/CONFIGURATION*.md`
- при необходимости `docs/development/*`

## Критерии приёмки

- Чётко описано, что именно сейчас делает `ai-models` для SSO и чего не делает.
- Чётко описано, чем `n8n-d8` отличается по app-native OIDC integration.
- По upstream `MLflow` есть предметный вывод, поддерживает ли он нужный parity
  без отдельного integration layer.
- Сформулирован рекомендуемый следующий implementation path без двусмысленности.

## Риски

- Можно перепутать ingress-level Dex SSO и app-native OIDC внутри приложения.
- Можно недооценить объём required patching, если upstream `MLflow` не даёт
  нужного extension point.
