# PLAN

## 1. Current phase

Этап 1: внутренний managed backend внутри модуля.

Задача остаётся в рамках phase 1: мы не вводим phase-2 CRD/controller API, а
доводим backend auth/login path до platform-usable состояния.

Режим orchestration: `solo`.

## 2. Slices

### Slice 1. Уточнить upstream и DKP contract

Цель:
- подтвердить, какой SSO path для MLflow сейчас является корректным;
- сопоставить это с DKP reference-паттернами (`n8n-d8`, DexClient, bootstrap jobs).

Файлы/каталоги:
- текущий bundle
- `templates/backend/`
- `templates/auth/`
- `images/backend/`
- reference repos

Проверки:
- анализ локального кода и official docs как primary source.

Результат:
- зафиксированный implementation path без архитектурной гадалки.

### Slice 2. Реализовать SSO wiring

Цель:
- включить вход в MLflow по SSO через Dex, не ломая backend runtime.

Файлы/каталоги:
- `openapi/`
- `templates/backend/`
- `templates/auth/`
- `templates/module/`
- `images/backend/`

Проверки:
- `make helm-template`
- точечные проверки runtime scripts, если появятся новые helper scripts.

Результат:
- browser login идёт по SSO, а runtime wiring остаётся согласованным.

### Slice 3. Совместить SSO с import/observability path

Цель:
- сохранить рабочими in-cluster import Jobs, probes и monitoring.

Файлы/каталоги:
- `templates/backend/`
- `tools/`
- `images/backend/`

Проверки:
- `bash -n ...` / `python3 -m py_compile ...`
- `make helm-template`

Результат:
- machine-oriented flows не деградируют после включения SSO.

### Slice 4. Docs, fixtures и quality gates

Цель:
- выровнять docs/fixtures/validation под новый login/auth contract.

Файлы/каталоги:
- `docs/`
- `fixtures/`
- `tools/helm-tests/`
- `tools/kubeconform/`

Проверки:
- `make verify`

Результат:
- новый contract описан и проверяется repo-level loop.

## 3. Rollback point

Безопасная точка отката — текущий native MLflow basic-auth/workspaces baseline
без SSO, когда вход идёт по bootstrap admin credentials.

## 4. Final validation

- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
