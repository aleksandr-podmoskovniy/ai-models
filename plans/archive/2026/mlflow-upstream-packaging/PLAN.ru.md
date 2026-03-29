# План работ: upstream MLflow packaging и phase-1 runtime для ai-models

## Current phase

Этап 1: внутренний managed `MLflow` inside the DKP module.

## Slice 1. Зафиксировать upstream baseline и donor boundaries

### Цель

Превратить внешний `MLflow` repo и Bitnami chart из "источников вдохновения" в формально описанные baselines с явными границами применения, где upstream release process является целевым build contract.

### Изменяемые области

- `plans/mlflow-upstream-packaging/`
- при необходимости `docs/development/`

### Проверки

- соответствие `docs/development/TZ.ru.md`
- соответствие `docs/development/PHASES.ru.md`
- отсутствие противоречий с `AGENTS.md`

### Артефакт

Зафиксированы:
- pinned upstream `MLflow` ref и update policy;
- upstream release contract: UI build, `release` wheel и image semantics `default` / `-full`;
- donor chart version и правило "Bitnami chart используется только как donor по semantics";
- ограничение phase 1: tracking/UI backend first, без phase-2 API.

## Slice 2. Определить repo-local layout для 3p source artifact и patch queue

### Цель

Подготовить layout, в котором pinned upstream metadata, source-artifact stages, patch queue и build scripts будут жить отдельно от DKP templates и docs.

### Изменяемые области

- `images/src-artifact/`
- `images/mlflow/`
- `DEVELOPMENT.md`

### Проверки

- layout совместим с текущим `werf` shell;
- отсутствует смешивание upstream code и module templates;
- patch flow воспроизводим без ad-hoc правок поверх fetched 3p source.

### Артефакт

Определены каталоги и роли:
- `images/src-artifact/werf.inc.yaml` для build-only source artifact base;
- `images/mlflow/upstream.lock` для pinned upstream metadata;
- `images/mlflow/werf.inc.yaml` для source fetch, patch application и image build definition;
- `images/mlflow/scripts/` для fetch/build/smoke helpers;
- `images/mlflow/patches/` для controlled patch series и rebase notes.

## Slice 3. Определить reproducible build flow из upstream source

### Цель

Зафиксировать packaging pipeline, который воспроизводит upstream release/full build `MLflow` из upstream source, включая UI и штатные компоненты.

### Изменяемые области

- `images/mlflow/`
- `.werf/stages/`
- `base_images.yml`
- `build/components/versions.yml`

### Проверки

- UI build учитывает требование Node.js `^22.19.0` и upstream `yarn install --immutable && yarn build`;
- release wheel повторяет upstream `release` semantics и содержит `mlflow/server/js/build/**/*`;
- image smoke покрывает как минимум `python -c 'import mlflow'` и `mlflow server --help`.

### Артефакт

Описан pipeline из отдельных стадий:
- source-artifact fetch из pinned external repository или явного local checkout override;
- frontend build для `mlflow/server/js`;
- release wheel build на базе `pyproject.release.toml` с upstream-equivalent package contents;
- runtime image assembly через `werf`, повторяющий upstream `docker/Dockerfile.full` semantics как baseline;
- отдельное решение для DKP-specific delta layer, если для phase 1 потребуется `auth` или другие additions поверх upstream `-full` image.

## Slice 4. Спроектировать phase-1 runtime contract и values mapping

### Цель

Связать уже существующие module `config-values` / runtime `values` с реальным upstream-like `MLflow` runtime и выбрать donor semantics, которые стоит перенести из Bitnami chart.

### Изменяемые области

- `openapi/`
- `templates/`
- `docs/CONFIGURATION*.md`
- `README*`

### Проверки

- `make helm-template`
- `make kubeconform`
- mapping укладывается в phase 1 и не выводит наружу raw `MLflow` contract;
- runtime wiring не требует урезания upstream release/full build.

### Артефакт

Определены:
- tracking-only runtime shape без Bitnami `run` component;
- правила для `--backend-store-uri`, `--artifacts-destination`, `--serve-artifacts` / `--no-serve-artifacts`, `--app-name=basic-auth` и DB upgrade path;
- mapping для PostgreSQL, S3-compatible artifacts, ingress/https, auth, metrics и image pull secrets;
- правило, как оформляются и валидируются явные DKP-specific overlays поверх upstream `-full` image;
- список значений, которые остаются internal/runtime-only и не становятся user-facing API.

## Slice 5. Зафиксировать CI/verify loop и rollout order

### Цель

Сделать так, чтобы import upstream, packaging и runtime wiring проверялись одинаково локально, в GitHub Actions и в GitLab.

### Изменяемые области

- `Makefile`
- `Taskfile.yaml`
- `.github/workflows/`
- `.gitlab-ci.yml`
- `docs/`

### Проверки

- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify`
- MLflow-specific build/smoke loop, добавленный этой задачей

### Артефакт

Определены:
- порядок внедрения slices без patchwork;
- обязательные smoke-проверки для source import, upstream-like wheel build и image build;
- единый verify path для локальной разработки и CI.

## Rollback point

После Slice 2. На этом шаге source layout и patch discipline уже определены, но runtime templates и module behavior еще не завязаны на новый `MLflow` image, поэтому можно остановиться без partially-wired production path.

## Final validation

- `make ensure-tools`
- `make fmt`
- `make lint`
- `make helm-template`
- `make kubeconform`
- `make verify`
- MLflow-specific smoke loop, введенный этой задачей: upstream-like `release` wheel с UI assets, импорт `mlflow` в runtime image, запуск `mlflow server --help` и проверка выбранного image variant contract
