# Починить доверие к CA Dex для MLflow OIDC SSO

## Контекст

После clean reinstall модуль `ai-models` успешно поднимает:
- managed PostgreSQL;
- отдельную auth database для `mlflow-oidc-auth`;
- `DexClient`;
- backend deployment и ingress.

При этом browser login в `https://ai-models.k8s.apiac.ru/login` падает с
`500 {"detail":"Failed to initiate OIDC login"}`. Live проверка из backend
pod показала, что чтение
`https://dex.k8s.apiac.ru/.well-known/openid-configuration` завершается
`SSL: CERTIFICATE_VERIFY_FAILED`, потому что контейнер не доверяет внутреннему
CA Dex.

## Постановка задачи

Нужно починить доставку и использование CA для Dex OIDC discovery/token/JWKS
без жёсткой завязки на ручной runtime patch. Решение должно быть DKP-совместимым
и воспроизводимым: модуль сам получает нужный CA, рендерит собственный namespaced
secret и использует его в backend runtime так, чтобы SSO работал на fresh install.

## Scope

- hook для discovery CA публичного Dex endpoint;
- internal values/schema для хранения обнаруженного CA;
- namespaced secret в `d8-ai-models` для OIDC CA;
- backend runtime/deployment wiring для trust bundle;
- render validation и docs.

## Non-goals

- не менять user-facing auth contract;
- не проектировать sync `namespace/group -> workspace`;
- не менять import/storage path;
- не тащить app-level authorization beyond текущий SSO slice.

## Затрагиваемые области

- `images/hooks/*`
- `templates/backend/*`
- `templates/module/*`
- `templates/_helpers.tpl`
- `openapi/values.yaml`
- `tools/helm-tests/*`
- `fixtures/render/*`
- `docs/CONFIGURATION*.md`

## Критерии приёмки

- модуль автоматически получает Dex CA без ручных post-install действий;
- rendered backend получает trust bundle для OIDC runtime;
- backend render не делает cross-namespace secret mount;
- `make helm-template` и `make verify` проходят;
- после rollout из backend pod можно успешно читать Dex discovery document по TLS.

## Риски

- в кластере может использоваться не только `CertManager`, но и
  `CustomCertificate`, поэтому выбор secret/CA нельзя захардкодить на один путь;
- нельзя ломать другие outbound HTTPS вызовы backend процесса, подменив trust
  store слишком узко;
- hook должен переживать first run, когда нужный secret ещё не успел появиться.
