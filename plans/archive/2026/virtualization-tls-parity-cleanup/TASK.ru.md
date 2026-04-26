# Выровнять controller webhook TLS по virtualization

## Контекст

При сравнении с `virtualization` найден следующий существенный drift после
cleanup Jobs: `ai-models` генерирует controller webhook CA/TLS прямо в Helm
template через `genCA` / `genSignedCert` и пытается стабилизировать это через
`lookup` existing Secret.

В `virtualization` internal TLS генерируется Go hook через
`module-sdk/common-hooks/tls-certificate`, сохраняется в internal values и
используется templates как обычный deterministic module state:
`virtualization.internal.controller.cert.{ca,crt,key}`.

## Постановка задачи

Перенести `ai-models` controller webhook TLS на virtualization-style internal
TLS hook и убрать random certificate generation из Helm render path.

## Scope

- Добавить hook для self-signed controller webhook TLS.
- Добавить internal values schema для `aiModels.internal.controller.cert`.
- Перевести `templates/controller/webhook.yaml` на values-backed cert data.
- Оставить Secret и webhook CA bundle deterministic относительно values.
- Убрать `lookup` / `genCA` / `genSignedCert` из controller webhook template.
- Обновить render validation, чтобы этот drift не вернулся.

## Non-goals

- Не менять public API `Model` / `ClusterModel`.
- Не менять user-facing RBAC роли.
- Не менять DMCR TLS в этом slice, кроме фиксации его как отдельного
  следующего кандидата.
- Не менять ingress/public certificate logic.

## Затрагиваемые области

- `images/hooks`
- `openapi/values.yaml`
- `templates/controller/webhook.yaml`
- `templates/controller/deployment.yaml`
- `tools/helm-tests`
- `plans/active/virtualization-tls-parity-cleanup`

## Критерии приёмки

- В controller webhook template нет `lookup`, `genCA`, `genSignedCert`.
- Webhook Secret и `MutatingWebhookConfiguration.clientConfig.caBundle`
  читают `aiModels.internal.controller.cert`.
- Hook registered in `images/hooks/cmd/ai-models-hooks/register.go`.
- `make helm-template`, `make kubeconform`, render validation и `make verify`
  проходят.
- Render validation запрещает Helm-time cert generation for controller webhook.

## Риски

- В fixture/render tests должны быть значения internal cert, иначе offline
  render начнёт падать на empty values.
- Common hook должен быть зарегистрирован до template render в live Deckhouse
  lifecycle.
