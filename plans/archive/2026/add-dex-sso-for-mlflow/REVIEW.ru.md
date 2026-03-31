# REVIEW

## Итог

Критичных замечаний по repo-level реализации нет.

## Что проверено

- Выбранный путь соответствует официальному MLflow SSO contract: browser login
  идёт через OIDC app, а не через ingress-only gate.
- Для DKP использован канонический `DexClient` паттерн с upgrade-safe `lookup`
  поведением по аналогии с `n8n-d8`.
- Direct-to-S3 import path сохранён.
- Machine-oriented access не потерян: `ServiceMonitor` и import Jobs остаются на
  внутреннем MLflow user, который bootstrap'ится init container'ом.
- Values/OpenAPI/templates/docs/fixtures/validation согласованы.
- `make verify` зелёный.

## Остаточные риски

- Первый live rollout для `auth.mode=DeckhouseSSO` нужно отдельно подтвердить на
  кластере: Dex callback, auto-created `DexClient` secret и bootstrap внутреннего
  machine user ещё не проверены end-to-end в runtime.
- Групповая модель пока intentionally conservative: по умолчанию внутрь
  допускается только Deckhouse `admins`. Более богатую user/group/workspace
  provisioning story нужно делать отдельным slice.
