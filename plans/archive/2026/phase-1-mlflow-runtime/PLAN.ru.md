# План работ: phase-1 runtime для managed MLflow в ai-models

## Current phase

Этап 1: внутренний managed `MLflow` inside the DKP module.

## Slice 1. Уточнить runtime contract и helper boundaries

### Цель

Зафиксировать внутри репозитория, какие runtime semantics берём из donor chart, а какие вычисляем через DKP library и global values.

### Изменяемые области

- `plans/phase-1-mlflow-runtime/`
- `templates/_helpers.tpl`

### Проверки

- соответствие `docs/development/TZ.ru.md`
- соответствие `docs/development/PHASES.ru.md`

### Артефакт

Появляются helper functions для namespace, names, host, auth wiring, PostgreSQL/S3 mapping и validation points.

## Slice 2. Довести values/OpenAPI до рабочего runtime baseline

### Цель

Сделать user-facing contract для PostgreSQL и S3 коротким, но достаточным для реального phase-1 deploy.

### Изменяемые области

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `fixtures/module-values.yaml`

### Проверки

- `make helm-template`

### Артефакт

Есть согласованные defaults и fixture values для managed/external PostgreSQL и S3-compatible storage.

## Slice 3. Реализовать MLflow runtime templates

### Цель

Перенести tracking/UI runtime semantics из donor chart в DKP-native manifests.

### Изменяемые области

- `templates/`

### Проверки

- `make helm-template`
- `make kubeconform`

### Артефакт

Появляются `Deployment`, `Service`, `Ingress`, `ConfigMap`, `ServiceAccount`, `PDB`, `ServiceMonitor`, `DexAuthenticator`, `Postgres`, `PostgresClass`.

## Slice 4. Подключить hooks и bundle wiring

### Цель

Поддержать global HTTPS modes и доставку hook binary в bundle без ручной сборки вне репозитория.

### Изменяемые области

- `hooks/`
- `.werf/stages/`
- `Makefile`
- `Taskfile.yaml`

### Проверки

- `make test`
- `make verify`

### Артефакт

Есть batch hook для copy custom certificate, а bundle включает hooks в publishable module artifact.

## Slice 5. Обновить эксплуатационные docs

### Цель

Сделать документацию согласованной с реальным phase-1 runtime contract.

### Изменяемые области

- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `DEVELOPMENT.md`

### Проверки

- `make lint`
- `make verify`

### Артефакт

Документация описывает deployable `ai-models` module с MLflow, PostgreSQL, S3 и Dex SSO.

## Rollback point

После Slice 2. На этом шаге contract и defaults уже выровнены, но runtime manifests ещё не заведены, поэтому можно безопасно остановиться без partially-wired runtime.

## Final validation

- `make fmt`
- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
