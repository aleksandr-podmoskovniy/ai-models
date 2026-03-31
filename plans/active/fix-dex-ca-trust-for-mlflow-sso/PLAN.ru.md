# PLAN: починить доверие к CA Dex для MLflow OIDC SSO

## Current phase

Этап 1. Внутренний managed backend. Задача ограничена working SSO path для
внутреннего backend engine.

## Slices

### Slice 1. Зафиксировать источник Dex CA
- Цель: выбрать DKP-совместимый source of truth для CA публичного Dex endpoint.
- Области:
  - `images/hooks/*`
  - `openapi/values.yaml`
- Проверки:
  - hook берёт `ca.crt`/`tls.crt` из `d8-user-authn` ingress secret и пишет во
    внутренние values;
  - first run без secret не валит render.
- Артефакт:
  - `aiModels.internal.discoveredDexCA`.

### Slice 2. Протащить CA в backend runtime
- Цель: отдать backend’у trust bundle без cross-namespace mounts и без ломки
  других outbound TLS путей.
- Области:
  - `templates/module/*`
  - `templates/backend/*`
  - `templates/_helpers.tpl`
- Проверки:
  - рендерится namespaced secret с Dex CA;
  - backend получает mounted CA и merged trust bundle;
  - OIDC runtime использует его через стандартные env vars.
- Артефакт:
  - backend способен валидировать TLS `dex.k8s.apiac.ru`.

### Slice 3. Закрыть verify loop и docs
- Цель: не оставить change only-in-cluster.
- Области:
  - `fixtures/render/*`
  - `tools/helm-tests/*`
  - `docs/CONFIGURATION*.md`
- Проверки:
  - `make helm-template`
  - `make verify`
- Артефакт:
  - render fixtures и docs покрывают новый runtime contract.

## Rollback point

После Slice 1, до изменения backend runtime/deployment wiring. На этом шаге CA
discovery уже описан, но rollout behaviour ещё не затронут.

## Orchestration mode

solo

## Final validation

- `make helm-template`
- `make verify`
