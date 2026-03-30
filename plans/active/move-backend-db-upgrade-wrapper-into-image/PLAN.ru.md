# PLAN: убрать backend DB upgrade wrapper из ConfigMap в image-owned runtime script

## Current phase

Этап 1. Внутренний managed backend. Задача ограничена runtime shell backend
image и startup init path.

## Slices

### Slice 1. Вынести runtime wrapper в image
- Цель: хранить executable DB init/upgrade logic под `images/backend`.
- Области:
  - `images/backend/scripts/*`
  - `images/backend/werf.inc.yaml`
  - `images/backend/Dockerfile.local`
- Проверки:
  - узкий render/runtime sanity
- Артефакт:
  - image-owned script `ai-models-backend-db-upgrade`.

### Slice 2. Упростить module wiring
- Цель: оставить в ConfigMap только thin launcher.
- Области:
  - `templates/backend/configmap.yaml`
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `make helm-template`
  - `make verify`
- Артефакт:
  - backend ConfigMap без inline Python runtime logic.

## Rollback point

После Slice 1, до перепривязки ConfigMap. На этом шаге runtime wrapper уже
добавлен в image build path, но templates ещё не используют его.

## Orchestration mode

solo

## Final validation

- `make helm-template`
- `make verify`
