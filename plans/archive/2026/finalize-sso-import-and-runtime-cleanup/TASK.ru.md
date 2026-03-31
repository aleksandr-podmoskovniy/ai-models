# TASK

## Контекст

В репозитории одновременно живут:
- новый target на Dex SSO внутри самого MLflow;
- legacy dual-mode auth contract (`Native` и `DeckhouseSSO`);
- direct-to-S3 import path для больших моделей;
- несколько мелких backend runtime helper scripts, часть из которых уже выглядит как избыточный glue.

Пользовательский приоритет на этот slice:
- довести до финала нормальный SSO path;
- оставить upload/import в бакеты как канонический путь;
- не делать пока sync `workspace <-> namespace`, он откладывается до controller/provisioner slice;
- убрать коллизии, лишнее legacy и переусложнение в runtime scripts.

## Постановка задачи

Привести phase-1 backend к одному каноническому runtime contract:
- browser users идут только через Dex SSO в сам MLflow;
- artifacts/import используют direct-to-S3 path;
- dual auth branch убран;
- runtime helper scripts консолидированы так, чтобы остались только реальные операции и один общий helper layer.

## Scope

- auth/runtime contract в `openapi/*`, `templates/*`, `images/backend/*`;
- runtime script consolidation в `images/backend/scripts/*`;
- render/verify tooling и fixtures;
- docs, которые описывают auth/import baseline.

## Non-goals

- не реализовывать sync `namespace/group -> workspace`;
- не добавлять ещё один controller или CRD API;
- не менять phase-2 boundaries `Model` / `ClusterModel`;
- не изобретать кастомный artifact protocol поверх upstream MLflow.

## Затрагиваемые области

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/backend/*`
- `templates/auth/*`
- `images/backend/scripts/*`
- `images/backend/werf.inc.yaml`
- `images/backend/Dockerfile.local`
- `tools/helm-tests/*`
- `tools/kubeconform/*`
- `fixtures/render/*`
- `docs/CONFIGURATION*.md`
- `DEVELOPMENT.md`

## Критерии приёмки

- user-facing auth contract больше не предлагает competing basic-auth path;
- backend стартует только через Dex/OIDC app-native SSO path для browser users;
- direct-to-S3 artifact upload path остаётся включённым и проверяемым;
- `render-auth-config` legacy удалён;
- мелкие runtime helper scripts консолидированы в один helper layer без дальнейшего расползания;
- `make verify` проходит.

## Риски

- можно случайно сломать machine account path для import jobs и monitoring;
- можно оставить в templates скрытый legacy branch, который уже не соответствует docs;
- можно перетащить слишком много refactor-а в один slice и потерять рабочий rollout.
