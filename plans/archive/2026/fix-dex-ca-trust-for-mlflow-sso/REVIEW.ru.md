# REVIEW: доверие к CA Dex для MLflow OIDC SSO

## Findings

Критичных замечаний по диффу нет.

## Checks

- `go test ./...` в `images/hooks`
- `make verify`

## Residual risk

- Repo-side wiring завершён и verify зелёный, но cluster-side эффект ещё нужно
  подтвердить новым rollout модуля: backend должен начать успешно читать Dex
  discovery document по TLS, а `/login` должен перестать отвечать `500`.
