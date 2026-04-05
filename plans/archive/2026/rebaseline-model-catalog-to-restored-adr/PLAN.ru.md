# PLAN

## 1. Current phase

Задача относится к phase 2. Внутренний backend уже есть как отдельный baseline,
а сейчас нужно выровнять phase-2 catalog track с restored ADR и не потерять
практические user expectations.

Orchestration mode: `full`.

Причина:

- меняются API/CRD/controller boundaries;
- затрагиваются public contract, status/conditions и runtime/module shell;
- нужно сверить design docs, code и target rollout;
- есть риск снова уехать в неявный redesign.

Read-only subagents:

- `api_designer`
  - проверить drift current public API против restored ADR;
- `integration_architect`
  - проверить module/runtime rollout path контроллера, CRD install, HA/metrics;
- `backend_integrator`
  - проверить, как user target по HF/MLflow/OCI publication можно совместить с
    restored ADR без протаскивания raw backend в public contract.

## 2. Slices

### Slice 1. Audit and rebaseline matrix

Цель:

- зафиксировать противоречия между restored ADR, current implementation и user
  target.

Файлы:

- `plans/active/rebaseline-model-catalog-to-restored-adr/*`
- `api/core/v1alpha1/*`
- `images/controller/internal/*`
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`

Проверки:

- audit matrix заполнен фактами с file refs;
- выводы subagents зафиксированы в bundle notes;
- для следующего implementation slice выбран bounded scope.

Артефакт:

- `NOTES.ru.md` с drift matrix и rebaseline decisions.

### Slice 2. ADR-neutral controller runtime rollout

Цель:

- довести controller до module runtime baseline, не закрепляя спорный public API
  drift.

Файлы:

- `images/controller/*`
- `images/hooks/*`
- `templates/*`
- `.werf/stages/*`
- `openapi/*`
- `docs/CONFIGURATION*.md`

Проверки:

- `go test ./...` in `images/controller`
- `go test ./...` in `images/hooks`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`

Артефакт:

- controller image;
- CRD install path;
- Deployment/ServiceAccount/RBAC/Service/ServiceMonitor/PDB;
- leader election, health, metrics.

### Slice 3. Public API rebaseline against restored ADR

Цель:

- начать выравнивание API shape и status model к restored ADR.

Файлы:

- `api/core/v1alpha1/*`
- `api/README.md`
- relevant phase-2 design docs

Проверки:

- `go generate ./...` in `api`
- `bash scripts/verify-crdgen.sh` in `api`
- `go test ./...` in `api`

Артефакт:

- согласованный next-step API direction без source-oriented contract drift.

### Slice 4. Review gate

Цель:

- проверить, что task bundle, код и docs реально вернули проект к restored ADR
  как source of truth.

Файлы:

- `plans/active/rebaseline-model-catalog-to-restored-adr/REVIEW.ru.md`

Проверки:

- `make verify`
- `git diff --check`

Артефакт:

- финальный review с findings и residual risks.

## 3. Rollback point

Безопасная точка остановки: audit matrix и rebaseline decisions оформлены, но
implementation changes beyond docs/plan ещё не внесены.

## 4. Final validation

- `go test ./...` in `api`
- `go test ./...` in `images/controller`
- `go test ./...` in `images/hooks`
- `bash scripts/verify-crdgen.sh` in `api`
- `make fmt`
- `make test`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
