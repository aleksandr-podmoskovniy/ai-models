# TASK

## Контекст
Текущий ai-models managed backend и in-cluster HF import Jobs ходят в S3-compatible storage по HTTPS с `artifacts.insecure: true`, из-за чего runtime работает в режиме без валидации TLS и с `InsecureRequestWarning` в boto3/urllib3.

## Постановка задачи
Добавить корректный CA trust path для outbound S3 access в phase-1 модуле ai-models, чтобы backend и import Jobs могли писать артефакты в S3-compatible endpoint с TLS verification, без перехода на ad-hoc runtime hacks и без вывода сырых backend entity наружу.

## Scope
- values/OpenAPI contract для artifact storage CA trust;
- module templates/secrets/volume mounts/env для backend runtime и import Jobs;
- backend runtime bridge для boto3/MLflow/AWS SDK CA bundle;
- docs и validate/verify guards;
- reuse current import Job helper wiring.

## Non-goals
- не менять public phase-2 Model/ClusterModel API;
- не заменять artifact auth contract;
- не делать автоматическое сетевое discovery CA у внешнего S3 endpoint;
- не вводить cluster-wide shared CA ownership без явного platform contract.

## Затрагиваемые области
- `openapi/*`
- `templates/*`
- `images/backend/scripts/*`
- `tools/*`
- `docs/*`
- `plans/active/fix-s3-ca-trust-for-artifacts/*`

## Критерии приёмки
- модуль умеет принять CA bundle для artifact storage по явному DKP contract;
- backend launcher экспортирует корректный CA bundle для S3 client path;
- in-cluster HF import Job получает тот же trust material автоматически;
- при включённом CA trust можно использовать `artifacts.insecure: false` без insecure warnings;
- `make verify` проходит.

## Риски
- нужен очень аккуратный contract, чтобы не раздуть `artifacts` лишними несовместимыми способами конфигурации;
- import helper не должен начать расходиться с backend runtime по source of truth для S3 TLS.
