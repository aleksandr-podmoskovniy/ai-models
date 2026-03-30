# Убрать default KEK passphrase из runtime ai-models

## Контекст

В живом кластере backend `ai-models` показывает upstream warning/banner про
`MLFLOW_CRYPTO_KEK_PASSPHRASE not set`. Это означает, что `MLflow` использует
default passphrase для encryption envelope, который допустим только для
локальной разработки.

Даже если phase-1 модуль пока не platformize'ит весь gateway/genai surface,
оставлять upstream default KEK в multi-user cluster нельзя.

## Постановка задачи

Нужно добавить module-owned security baseline для `MLflow` crypto KEK:
- генерировать и хранить passphrase во внутреннем Secret модуля;
- подавать его в backend runtime через env var
  `MLFLOW_CRYPTO_KEK_PASSPHRASE`;
- сохранять passphrase стабильным при обычных upgrade/reconcile.

## Scope

- helper для stable generated passphrase;
- internal Secret template;
- backend Deployment env wiring;
- docs про новый security baseline.

## Non-goals

- не platformize'ить сейчас AI Gateway как supported phase-1 feature;
- не вводить user-facing config для этого passphrase;
- не проектировать rotation workflow поверх module API в этом slice;
- не вырезать целиком genai/gateway surface из backend packaging.

## Затрагиваемые области

- `templates/_helpers.tpl`
- `templates/module/*`
- `templates/backend/deployment.yaml`
- `docs/CONFIGURATION*.md`
- `plans/active/harden-mlflow-crypto-passphrase/*`

## Критерии приёмки

- backend больше не стартует с upstream default KEK passphrase;
- `MLFLOW_CRYPTO_KEK_PASSPHRASE` берётся из module-owned Secret;
- generated passphrase сохраняется при обычном upgrade благодаря `lookup`;
- repo-level проверки проходят.

## Риски

- если в будущем кто-то начнёт активно использовать gateway secrets, rotation
  понадобится делать уже отдельным controlled workflow;
- нельзя случайно сделать passphrase user-facing значением без явной причины.
