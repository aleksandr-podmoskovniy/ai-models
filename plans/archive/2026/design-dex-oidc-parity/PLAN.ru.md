# PLAN: определить путь к Dex/OIDC parity с n8n

## Current phase

Этап 1. Внутренний managed backend. Задача относится к auth/SSO boundary
внутреннего backend и определяет, остаётся ли phase-1 на ingress-level Dex SSO
или требует app-native OIDC integration.

## Slices

### Slice 1. Зафиксировать текущий auth path ai-models
- Цель: отделить то, что уже даёт модуль, от того, чего в нём нет.
- Области:
  - `templates/auth/*`
  - `templates/backend/ingress.yaml`
  - `templates/_helpers.tpl`
  - cluster diagnostics по `user-authn`, `DexAuthenticator`, `DexProvider`
- Проверки:
  - file inspection
  - cluster object inspection
- Артефакт:
  - подтверждённое описание текущего SSO path для `ai-models`.

### Slice 2. Сравнить с n8n-d8 и upstream MLflow
- Цель: понять, можно ли получить parity без неаккуратной самодеятельности.
- Области:
  - `n8n-d8` SSO/OIDC bootstrap path
  - upstream `MLflow` auth/OIDC/security sources
- Проверки:
  - primary-source inspection
- Артефакт:
  - матрица различий: ingress SSO vs app-native OIDC client.

### Slice 3. Зафиксировать рекомендуемый implementation path
- Цель: дать следующий шаг без архитектурной двусмысленности.
- Области:
  - `plans/active/design-dex-oidc-parity/*`
  - при необходимости docs
- Проверки:
  - связность выводов с текущим phase-1 scope
- Артефакт:
  - решение, как делать parity корректно и что для этого потребуется.

## Rollback point

После Slice 1, без repo-side изменений, только с диагностикой и сравнением.

## Orchestration mode

solo

## Final validation

- узкие проверки на чтение/сравнение источников;
- `make verify`, только если по итогам анализа изменяются файлы репозитория.
