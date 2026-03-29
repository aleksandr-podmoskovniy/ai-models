# Убрать MLflow branding из module-facing поверхности ai-models

## Контекст

Сейчас `ai-models` уже позиционируется как самостоятельный DKP-модуль, но в
module-facing поверхности репозитория всё ещё слишком много прямых упоминаний
`MLflow`: в metadata, README/docs, OpenAPI descriptions, operational commands и
части runtime naming.

Пользователь явно требует, чтобы модуль снаружи читался как `ai-models`, а
реальный backend engine оставался только под капотом.

## Постановка задачи

Нужно убрать прямой `MLflow` branding из наружной поверхности модуля и сделать
акцент на `ai-models` как на самостоятельном продукте/модуле.

## Scope

- вычистить `MLflow` из module metadata, README/docs и development guidance;
- выровнять OpenAPI/values descriptions и defaults так, чтобы наружный контракт
  описывал `ai-models`, а не конкретный backend engine;
- перевести runtime-facing naming модуля на `ai-models` / `backend`, где это
  не ломает реализацию;
- перевести repo-facing operational entrypoints с `mlflow-*` на более нейтральный
  `backend-*`.

## Non-goals

- не переписывать весь internal upstream build layer;
- не переименовывать upstream source layout внутри imported 3p component;
- не менять прикладную архитектуру phase-1 backend;
- не убирать технические внутренние переменные, если они нужны для реальной
  работы upstream engine.

## Затрагиваемые области

- `plans/remove-mlflow-from-module-surface/`
- `module.yaml`
- `README*.md`
- `docs/`
- `AGENTS.md`
- `DEVELOPMENT.md`
- `openapi/`
- `templates/`
- `fixtures/module-values.yaml`
- `Makefile`
- `Taskfile.yaml`
- `.agents/` / `.codex/`, если там ещё торчит `MLflow` branding

## Критерии приёмки

- module-facing docs, metadata, values/OpenAPI и runtime naming больше не
  продвигают `MLflow` как идентичность модуля;
- наружный контракт и operational wording модуля делают акцент на `ai-models`
  и internal backend, а не на конкретный upstream продукт;
- runtime templates и build entrypoints остаются рабочими;
- `make verify` проходит.

## Риски

- слишком глубокий rename может затронуть внутренний build shell и upstream
  packaging без пользы;
- часть технических `MLFLOW_*` env vars и внутренних путей неизбежно может
  остаться в implementation layer, если они являются реальным контрактом
  upstream backend.
