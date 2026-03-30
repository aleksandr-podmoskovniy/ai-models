# PLAN

## 1. Current phase

Этап 1: внутренний managed backend внутри модуля.

Задача остаётся в рамках phase 1, но ужесточает backend auth/storage integration до более корректного upstream-aligned baseline.

Режим orchestration: `solo`.

## 2. Slices

### Slice 1. Поднять current-state и upstream contract

Цель:
- подтвердить repo-local точки входа для backend auth/import;
- зафиксировать upstream MLflow auth/workspaces и artifact access contracts.

Файлы/каталоги:
- `templates/backend/`
- `templates/auth/`
- `images/backend/`
- `tools/`
- текущий bundle notes/decisions

Проверки:
- точечный анализ файлов;
- официальная документация MLflow как primary source.

Результат:
- bundle с зафиксированными implementation boundaries.

### Slice 2. Native MLflow auth/workspaces wiring

Цель:
- включить upstream-native backend auth/workspaces path в module runtime и values contract.

Файлы/каталоги:
- `openapi/`
- `templates/backend/`
- `templates/auth/`
- `templates/module/`
- `images/backend/`
- `fixtures/`

Проверки:
- `make helm-template`
- `make kubeconform` или render-level semantic checks, если достаточно для slice

Результат:
- backend runtime и templates согласованно рендерят native auth/workspaces configuration.

### Slice 3. Direct-to-S3 import path

Цель:
- перевести machine-oriented import Jobs на direct artifact access path вместо server proxy.

Файлы/каталоги:
- `templates/backend/`
- `images/backend/`
- `tools/`
- `fixtures/`

Проверки:
- `python3 -m py_compile ...`
- `bash -n tools/run_hf_import_job.sh`
- `make helm-template`

Результат:
- import Job использует internal tracking/backend metadata path, но artifact upload идёт напрямую в S3.

### Slice 4. Docs и quality gates

Цель:
- зафиксировать новый phase-1 contract в docs и verify loop.

Файлы/каталоги:
- `docs/`
- `tools/helm-tests/`
- `fixtures/`

Проверки:
- `make verify`

Результат:
- docs, fixtures и verify соответствуют новому backend/auth/storage baseline.

## 3. Rollback point

Безопасная точка отката — состояние до включения native auth/workspaces и до изменения artifact access mode, когда ingress-level Dex и proxied artifact path ещё работали как раньше.

## 4. Final validation

- `make lint`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
