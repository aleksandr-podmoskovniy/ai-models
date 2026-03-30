# NOTES: Dex/OIDC parity with n8n

## Текущий path в ai-models

- `ai-models` сейчас использует `DexAuthenticator` + ingress auth annotations.
- Это даёт SSO gate на входе в UI, но не делает внутренний backend OIDC client.
- В кластере `DexAuthenticator` для `ai-models` уже создаётся, но `DexProvider`
  пока не настроен, поэтому внешний OIDC provider ещё не подключён вообще.

## Что делает n8n-d8

- `n8n-d8` не ограничивается ingress-level Dex auth.
- У него есть отдельный app-native OIDC bootstrap against Deckhouse Dex:
  - `sso.dex` values;
  - `DexClient`;
  - bootstrap job, который конфигурирует OIDC внутри самого приложения;
  - provisioning по `groups` claims.
- Это видно в `n8n-d8/images/n8n-helm/patches/README.md`.

## Что реально даёт upstream MLflow

- `MLflow server` имеет security middleware (`allowed-hosts`, CORS, X-Frame-Options),
  но это не app-native SSO.
- У upstream есть отдельное приложение `mlflow server --app-name basic-auth`.
- У `basic-auth` app есть только basic-auth oriented auth surface и `authorization_function`
  hook.
- Для FastAPI routes upstream прямо пишет, что custom `authorization_function` не
  поддерживается; поддерживается только default basic-auth function.

## Следствие

- Parity уровня `n8n` не получается одной DKP-обвязкой вокруг vanilla `mlflow server`.
- Для такого parity нужен отдельный integration layer поверх upstream auth app
  или controlled upstream patching.
