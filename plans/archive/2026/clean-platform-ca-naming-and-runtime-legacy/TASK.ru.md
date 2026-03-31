# TASK

## Контекст
После нескольких auth/storage slices в репозитории остались наслоения терминов и runtime-хвосты: секрет `backend-oidc-ca` уже используется как общий platform trust CA для Dex и S3, а не только для OIDC; в runtime дереве остаются `__pycache__`-хвосты; часть старых compatibility shims нужно явно классифицировать как живые или удалить.

## Постановка задачи
Почистить явные naming-collisions и runtime legacy в phase-1 ai-models backend integration без смены текущего phase-1 контракта и без новой архитектурной развилки.

## Scope
- rename общего CA wiring из OIDC-specific naming в platform-trust naming;
- убрать runtime мусор из `images/backend/scripts` и закрепить его невозврат;
- проверить старые compatibility shims и удалить только действительно мёртвые.

## Non-goals
- не менять phase-1 SSO/storage semantics;
- не перепроектировать import flow или auth DB contract;
- не трогать phase-2 API и controller design.

## Затрагиваемые области
- `templates/*`
- `openapi/*` при необходимости только для wording
- `tools/*`
- `images/backend/scripts/*`
- `docs/*`
- `plans/active/clean-platform-ca-naming-and-runtime-legacy/*`

## Критерии приёмки
- общий CA path больше не выглядит как OIDC-only, если он обслуживает и S3;
- runtime tree не содержит лишнего Python cache мусора;
- явные legacy shims либо удалены, либо осмысленно сохранены и не выглядят как мёртвый код;
- `make verify` проходит.

## Риски
- rename внутреннего Secret/volume/env может затронуть rollout semantics, поэтому нужно держать change узким и согласованным между templates, runtime и validate guards;
- удаление compatibility fallback бездумно может сломать upgrade path со старых инсталляций.
