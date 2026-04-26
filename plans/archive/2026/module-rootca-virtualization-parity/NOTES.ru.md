# Notes

## Virtualization baseline

- `ca-discovery` hook читает persisted Secret `virtualization-ca` и пишет его
  в `virtualization.internal.rootCA`.
- TLS hooks передают `CommonCAValuesPath:
  fmt.Sprintf("%s.internal.rootCA", settings.ModuleName)`.
- `templates/rootCA-secret.yaml` сохраняет rootCA в `kubernetes.io/tls`
  Secret.
- `templates/rootCA-cm.yaml` публикует CA bundle как ConfigMap для internal
  consumers.

## Drift в ai-models

У `ai-models` controller и DMCR certs уже hook-owned, но без общего module CA:
каждый endpoint генерирует собственную CA. Это хуже для поддержки и расходится
с virtualization TLS surface.

## Implementation notes

- `aiModels.internal.rootCA.crt/key` хранятся в values как base64 strings,
  потому что `CommonCAValuesPath` использует `certificate.Authority` с
  `[]byte` и JSON-представлением.
- Endpoint cert values (`controller.cert`, `dmcr.cert`) остаются raw PEM и
  кодируются в templates через `b64enc`; это другой контракт module-sdk
  `FullValuesPathPrefix`.
- При первом rollout после подключения common CA endpoint certs могут
  перегенерироваться, потому что их прежние отдельные CA перестанут совпадать
  с `aiModels.internal.rootCA`.
