# Выровнять internal TLS rootCA по virtualization

## Контекст

Controller webhook TLS и DMCR TLS уже перенесены на module-sdk TLS hooks, но
каждый endpoint пока получает отдельную CA. В `virtualization` internal TLS
endpoints используют общий module rootCA через `CommonCAValuesPath`, а rootCA
persisted в module Secret и восстанавливается отдельным hook до генерации
endpoint certificates.

## Постановка задачи

Добавить общий `aiModels.internal.rootCA` для internal TLS endpoints и
подписывать им controller/DMCR certs, сохранив deterministic render path.

## Scope

- Добавить rootCA discovery hook.
- Добавить `aiModels.internal.rootCA.{crt,key}` в values schema и fixtures.
- Включить `CommonCAValuesPath` для controller и DMCR TLS hooks.
- Добавить rootCA Secret и ConfigMap templates по virtualization pattern.
- Добавить render validation на rootCA surface.

## Non-goals

- Не менять public API, RBAC, ingress/public certificate logic.
- Не менять DMCR auth/storage/ports.
- Не вводить новые external contracts; rootCA остаётся internal module state.

## Критерии приёмки

- Controller и DMCR TLS hooks используют `CommonCAValuesPath`.
- RootCA Secret и ConfigMap рендерятся из `aiModels.internal.rootCA`.
- `templates/controller/webhook.yaml` и `templates/dmcr/secret.yaml` остаются
  values-backed без Helm-time TLS generation.
- `make helm-template`, `make kubeconform`, hooks tests и `make verify` проходят.
