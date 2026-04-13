# PLAN

## Current phase

Этап 2. Cross-cutting hardening of the phase-2 runtime logging contract for
our Go-owned components.

## Orchestration

- mode: `solo`
- reason:
  - bounded cross-cutting slice with clear live scope;
  - subagent delegation в этой сессии не нужна и только замедлит работу.

## Slice 1. Open dedicated logging bundle

Цель:

- не прятать logging hardening inside controller structural cleanup;
- зафиксировать отдельный scope и acceptance criteria.

Артефакты:

- `plans/active/logging-contract-hardening/TASK.ru.md`
- `plans/active/logging-contract-hardening/PLAN.ru.md`

Проверки:

- `find plans/active/logging-contract-hardening -maxdepth 1 -type f | sort`

## Slice 2. Normalize Go runtime logger contract

Цель:

- ввести custom JSON formatter в `cmdsupport`;
- сделать `json` default for controller and artifact-runtime;
- сохранить bridge в `controller-runtime` и `klog`;
- не смешивать formatter contract с backend/`dmcr`.

Артефакты:

- updated `images/controller/internal/cmdsupport/*`
- updated `images/controller/cmd/ai-models-controller/run.go`
- updated `images/controller/cmd/ai-models-artifact-runtime/dispatch.go`

Проверки:

- focused `go test` for `images/controller/internal/cmdsupport`

## Slice 3. Wire LOG_FORMAT through live runtime surfaces

Цель:

- явно закрепить `LOG_FORMAT=json` в deployment surface;
- не потерять logging contract в spawned runtime paths.

Артефакты:

- updated `templates/controller/deployment.yaml`
- updated runtime env wiring in sourceworker/cleanup code where needed

Проверки:

- focused `go test` for touched runtime packages
- `make helm-template`

## Slice 4. Review severity vocabulary and finalize notes

Цель:

- проверить, что наши explicit log sites используют `error` только для real
  failures в этом bounded slice;
- зафиксировать validations и residual risks.

Артефакты:

- `plans/active/logging-contract-hardening/REVIEW.ru.md`

Проверки:

- manual grep over explicit log sites in touched files

## Slice 5. Add structured lifecycle logs to dmcr-cleaner

Цель:

- закрыть live operational gap, где `dmcr-garbage-collection` container не
  пишет вообще ничего;
- выровнять repo-owned helper под тот же JSON envelope, не трогая основной
  upstream `dmcr`.

Артефакты:

- updated `images/dmcr/cmd/dmcr-cleaner/*`
- updated `images/dmcr/internal/garbagecollection/*`
- added `images/dmcr/internal/logging/*`
- updated `templates/dmcr/deployment.yaml`
- updated `images/dmcr/README.md`

Проверки:

- `cd images/dmcr && go test ./internal/logging ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- `make helm-template`

## Rollback point

Если новый logging contract окажется неудачным:

1. удалить bundle `plans/active/logging-contract-hardening/`;
2. вернуть stock `slog` handlers и старые `text` defaults;
3. убрать explicit `LOG_FORMAT` wiring из manifests.
4. удалить `images/dmcr/internal/logging/` и вернуть `dmcr-cleaner` на
   pre-logging behavior.

## Final validation

- `cd images/controller && go test ./internal/cmdsupport ./cmd/ai-models-controller ./cmd/ai-models-artifact-runtime ./internal/adapters/k8s/sourceworker ./internal/controllers/catalogcleanup`
- `cd images/dmcr && go test ./internal/logging ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- `make helm-template`
- `make verify`
- `git diff --check`
