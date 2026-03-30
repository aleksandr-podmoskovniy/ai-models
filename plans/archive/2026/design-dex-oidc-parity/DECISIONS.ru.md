# DECISIONS: Dex/OIDC parity with n8n

## Вывод

`ai-models` сейчас не имеет app-native OIDC parity с `n8n-d8`, и этот parity
нельзя получить "автоматом" только через `DexAuthenticator`.

## Рекомендуемый путь

1. Считать текущий ingress-level Dex SSO в `ai-models` рабочим baseline phase-1.
2. Если нужен именно parity с `n8n`, делать это как отдельную substantial task:
   - спроектировать app-native auth layer для внутреннего backend;
   - решить, идём ли через upstream `basic-auth` app + controlled patching,
     либо через отдельный auth proxy/bridge с явным ownership model;
   - отдельно закрыть user identity, provisioning и permission mapping.

## Что не считать корректным решением

- Называть ingress-level Dex auth "полным OIDC parity".
- Включать внешний OIDC provider в `user-authn` и считать, что этого достаточно
  для внутреннего backend parity с `n8n`.
- Делать ad-hoc hacks внутри templates без явного решения по ownership модели
  auth внутри самого backend.
