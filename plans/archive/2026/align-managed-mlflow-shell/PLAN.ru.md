# План работ: выравнивание phase-1 shell для managed MLflow

## Current phase

Этап 1: внутренний managed MLflow внутри DKP-модуля.

## Slice 1. Зафиксировать task bundle и phase-1 framing

### Цель

Зафиксировать, что изменение относится к выравниванию shell, а не к переносу upstream `MLflow`.

### Изменяемые области

- `plans/align-managed-mlflow-shell/`

### Проверки

- соответствие `AGENTS.md`
- соответствие `docs/development/TZ.ru.md`

### Артефакт

Task bundle с явным scope, non-goals и rollback point.

## Slice 2. Выровнять docs и metadata модуля под stage 1

### Цель

Убрать из пользовательских описаний и module metadata premature semantics этапа 2 и сфокусировать shell на managed `MLflow`.

### Изменяемые области

- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `module.yaml`

### Проверки

- `make lint`

### Артефакт

Согласованные тексты для root/docs/module metadata.

## Slice 3. Ввести phase-1 values/OpenAPI baseline

### Цель

Описать минимально полезный и расширяемый контракт для `MLflow`, storage, ingress/https и auth по образцу разделения из `n8n-d8`.

### Изменяемые области

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `fixtures/module-values.yaml`

### Проверки

- `make lint`
- `make helm-template`

### Артефакт

User-facing schema и runtime values, пригодные для следующего шага с переносом `MLflow` build/runtime.

## Slice 4. Подчистить templates и repo checks

### Цель

Убрать фейковый registry flow и выровнять verify-команды под реальный shell.

### Изменяемые области

- `templates/_helpers.tpl`
- `templates/registry-secret.yaml`
- `Makefile`

### Проверки

- `make lint`
- `make helm-template`
- `make kubeconform`

### Артефакт

Реальный registry secret fallback и согласованный verify loop.

## Rollback point

После Slice 2. На этом шаге docs и metadata уже выровнены, но runtime shell ещё не изменён, поэтому можно безопасно остановиться без partially-wired values/template контракта.

## Final validation

- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify`
