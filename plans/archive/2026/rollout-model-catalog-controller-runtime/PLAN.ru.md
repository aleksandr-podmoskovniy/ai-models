# PLAN

## 1. Current phase

Задача относится к phase 2: ввод operational runtime для `Model` /
`ClusterModel` controller поверх already-working phase-1 backend module.

Orchestration mode: `full`.

Причина:

- меняются module build/layout и runtime manifests;
- появляется CRD rollout boundary;
- появляется controller HA/metrics/leader-election operational shape;
- задача затрагивает несколько областей репозитория и module shell.

Read-only subagents:

- `repo_architect`
  - проверить module shell/layout для controller manifests, image wiring и CRD
    rollout path;
- `integration_architect`
  - проверить runtime shape controller deployment:
    leader election, RBAC, metrics, ServiceMonitor, CRD ensure strategy.

Предварительные assumptions:

- controller остаётся отдельным runtime под `images/controller`;
- CRD лучше ставить через module hook/ensure pattern, а не прямыми template
  manifests;
- first rollout slice должен довести process shape, но не обязан уже запускать
  HF/import reconcile logic;
- module values для controller должны быть runtime-only, как и для backend.

## 2. Slices

### Slice 1. Зафиксировать task bundle и rollout boundaries

Цель:

- оформить phase-2 controller runtime как bounded operational slice.

Файлы:

- `plans/active/rollout-model-catalog-controller-runtime/TASK.ru.md`
- `plans/active/rollout-model-catalog-controller-runtime/PLAN.ru.md`

Проверки:

- scope согласован с `AGENTS.md`
- phase boundary не смешивает rollout runtime и HF/publish worker logic

Артефакт:

- task bundle для controller runtime rollout.

### Slice 2. Подготовить build + CRD rollout path

Цель:

- добавить controller image и module-owned CRD install path.

Файлы:

- `werf.yaml`
- `.werf/stages/*`, если нужно
- `images/controller/*`
- `images/hooks/*`
- `api/scripts/*`, если нужен CRD export path
- новый `crds/`, если понадобится module-owned packaging layer

Проверки:

- `go test ./...` в `images/hooks`
- `go test ./...` в `images/controller`
- `bash scripts/verify-crdgen.sh` в `api`

Артефакт:

- reproducible controller image wiring;
- reproducible CRD install path for module runtime.

### Slice 3. Добавить controller manifests и runtime shape

Цель:

- развернуть controller как полноценный DKP runtime component.

Файлы:

- `templates/controller/*`
- `templates/_helpers.tpl`
- `openapi/*`
- `docs/CONFIGURATION*.md`
- `images/controller/internal/app/*`
- `images/controller/cmd/*`

Проверки:

- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`

Артефакт:

- controller Deployment, ServiceAccount, RBAC, Service, ServiceMonitor, PDB;
- leader election + health + metrics runtime shape.

### Slice 4. Закрыть review gate

Цель:

- подтвердить, что module shell, controller runtime и CRD rollout согласованы.

Файлы:

- `plans/active/rollout-model-catalog-controller-runtime/REVIEW.ru.md`

Проверки:

- `make verify`
- `git diff --check`

Артефакт:

- финальный review bundle с findings и residual risks.

## 3. Rollback point

Безопасная точка остановки: controller image wiring и CRD rollout path готовы, но
module manifests controller runtime ещё не включены.

## 4. Final validation

- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `images/controller`
- `go test ./...` в `images/hooks`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
