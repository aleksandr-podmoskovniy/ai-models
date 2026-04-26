# Notes

## Virtualization baseline

DVCR TLS в `virtualization` устроен как module internal state:

- hook `tls-certificates-dvcr` регистрирует
  `tlscertificate.RegisterInternalTLSHookEM`;
- cert state пишется в `virtualization.internal.dvcr.cert.{ca,crt,key}`;
- Secret `dvcr-tls` рендерится как `kubernetes.io/tls` из values;
- клиентский CA Secret содержит только публичный `ca.crt`;
- Helm template не генерирует TLS и не рендерит приватный `ca.key`.

## Drift в ai-models

DMCR TLS в `ai-models` всё ещё живёт в Helm template:

- `lookup` существующего `ai-models-dmcr-tls`;
- fallback на `genCA` и `genSignedCert`;
- Secret типа `Opaque` со `stringData`;
- приватный `ca.key` попадает в runtime Secret;
- checksum зависит от existing/rendered state, а не от explicit internal
  values.

Это создаёт random offline render, усложняет rollout reasoning и отличается от
DVCR-паттерна virtualization.
