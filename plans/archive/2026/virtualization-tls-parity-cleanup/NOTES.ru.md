# Notes

## Virtualization baseline

`virtualization` не генерирует controller webhook TLS прямо Helm template:

- hook `tls-certificates-controller` регистрирует
  `tlscertificate.RegisterInternalTLSHookEM`;
- cert state пишется в
  `virtualization.internal.controller.cert.{ca,crt,key}`;
- Secret `virtualization-controller-tls` и admission webhooks читают эти values;
- render path не зависит от `lookup`, `genCA` или `genSignedCert`.

## Drift в ai-models

`templates/controller/webhook.yaml` сейчас делает:

- `lookup` существующего Secret;
- если Secret не найден, `genCA` и `genSignedCert`;
- кладёт cert/key в `stringData`;
- использует generated CA как webhook `caBundle`;
- Deployment checksum считает весь webhook template, то есть offline render
  получает новый cert на каждый render.

Это хуже virtualization-паттерна: render становится stateful/random, а
сертификат не является явным module internal state.
