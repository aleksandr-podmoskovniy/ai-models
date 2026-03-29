# План работ: этап 1 managed MLflow

## Current phase

Этап 1: внутренний managed MLflow.

## Slice 1. Зафиксировать deployment shape MLflow

### Цель
Понять, какие компоненты, values и секреты реально нужны для рабочего baseline.

### Изменяемые области
- `docs/development/`
- при необходимости `images/mlflow/patches/README.md`

### Проверки
- review of upstream assumptions
- отсутствие противоречий с `TZ.ru.md`

## Slice 2. Описать module values и OpenAPI

### Цель
Ввести минимально достаточные настройки MLflow, PostgreSQL, artifact storage и ingress.

### Изменяемые области
- `openapi/values.yaml`
- `openapi/config-values.yaml`
- `docs/CONFIGURATION*.md`

### Проверки
- `make helm-template`
- `make kubeconform`

## Slice 3. Собрать DKP-style templates для MLflow backend

### Цель
Добавить Deployment/Service/Ingress/Secrets/ConfigMaps в манере DKP и deckhouse lib helm.

### Изменяемые области
- `templates/`
- `module.yaml`
- `Chart.yaml`

### Проверки
- `make helm-template`
- `make kubeconform`

## Slice 4. Подключить observability и базовую auth story

### Цель
Обозначить и внедрить базовую схему доступа, логирования и мониторинга.

### Изменяемые области
- `templates/`
- docs
- при необходимости monitoring manifests

### Проверки
- `make helm-template`
- `make kubeconform`

## Slice 5. Подчистить build/release shell и документацию

### Цель
Сделать так, чтобы модуль можно было последовательно развивать дальше без расползания структуры.

### Изменяемые области
- `werf.yaml`
- `base_images.yml`
- `README*`
- `DEVELOPMENT.md`
- `docs/development/`

### Проверки
- `make lint`
- `make helm-template`
- `make kubeconform`

## Rollback point

После Slice 2. Если deployment shape ещё не реализован, а только values/OpenAPI и docs подготовлены, проект остаётся чистым и без полусобранного runtime.

## Final validation

- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify` если к тому моменту репозиторий уже позволяет его пройти без ложных падений
