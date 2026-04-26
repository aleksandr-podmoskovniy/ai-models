# Выровнять DMCR TLS по virtualization DVCR pattern

## Контекст

После перевода controller webhook TLS на virtualization-style hook остаётся
такой же drift в DMCR: `templates/dmcr/secret.yaml` генерирует CA/TLS прямо в
Helm render path через `lookup`, `genCA` и `genSignedCert`, рендерит приватный
`ca.key` и держит checksum-логику вокруг generated template state.

В `virtualization` DVCR TLS заведён через `module-sdk/common-hooks/tls-certificate`:
hook пишет cert state в internal values, а template только рендерит Secret из
values.

## Постановка задачи

Перенести DMCR TLS на hook-owned internal values и убрать Helm-time certificate
generation из DMCR template без изменения публичного API и DMCR auth contract.

## Scope

- Добавить internal TLS hook для DMCR.
- Добавить `aiModels.internal.dmcr.cert.{ca,crt,key}` в values schema и
  fixtures.
- Перевести `templates/dmcr/secret.yaml` на `kubernetes.io/tls` data из values.
- Оставить отдельный `ai-models-dmcr-ca` Secret только с `ca.crt` для клиентов.
- Пересчитать DMCR TLS restart checksum из values-backed cert/key, а не через
  generated/lookup state.
- Расширить render validation на запрет DMCR Helm-time TLS generation и
  приватного `ca.key`.

## Non-goals

- Не менять DMCR auth/password generation в этом slice.
- Не менять registry protocol, ports или storage wiring.
- Не менять public API `Model` / `ClusterModel`.
- Не вводить module common rootCA для всех endpoints; это отдельный slice,
  если понадобится единая CA и rootCA Secret.

## Критерии приёмки

- `templates/dmcr/secret.yaml` не содержит `genCA` / `genSignedCert`.
- DMCR TLS Secret рендерится как `kubernetes.io/tls` и не содержит `ca.key`.
- DMCR CA Secret содержит только `ca.crt`.
- `Deployment/dmcr checksum/secret` продолжает совпадать с runtime Secret
  restart annotations.
- `make helm-template`, `make kubeconform`, render validation и `make verify`
  проходят.
