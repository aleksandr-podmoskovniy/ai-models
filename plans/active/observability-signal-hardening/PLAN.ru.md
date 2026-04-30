# Plan

## 1. Current Phase

Этап 1 / publication-runtime baseline hardening. Задача усиливает
эксплуатационный слой без изменения public API, RBAC или runtime entrypoints.

## 2. Orchestration

`full` for the 2026-04-29 follow-up closure.

Причина: follow-up затрагивает workload delivery, DMCR logging и общий
production signal/noise contract. Перед изменениями использованы read-only
reviews `integration_architect` и `backend_integrator`.

## 3. Active Bundle Disposition

- `live-e2e-ha-validation` — keep. Это отдельный executable workstream для
  live e2e/HA; observability live proof будет фиксироваться там.
- `observability-signal-hardening` — current. Компактный executable bundle
  для pending Slice 4: live metrics/alerts и log field dictionary.
- `ray-a30-ai-models-registry-cutover` — archived. KubeRay-specific cutover
  premise superseded by generic PodTemplate CSI delivery contract.

## 4. Slices

### Slice 1. Shared collector health contract

Цель:

- добавить общий low-cardinality scrape-health helper;
- подключить его к catalog/runtime/storage collectors;
- покрыть success/failure unit-тестами.

Файлы:

- `images/controller/internal/monitoring/collectorhealth/*`;
- `images/controller/internal/monitoring/catalogmetrics/*`;
- `images/controller/internal/monitoring/runtimehealth/*`;
- `images/controller/internal/monitoring/storageusage/*`.

Проверки:

- `cd images/controller && go test ./internal/monitoring/...`;
- `git diff --check`.

Артефакт:

- health metrics появляются рядом с доменными метриками, но не заменяют их.

Статус: выполнено.

Проверки:

- `cd images/controller && go test ./internal/monitoring/...` — passed.
- `cd images/controller && go test ./...` — passed.

### Slice 2. Structured error attribute parity

Цель:

- выровнять controller/runtime и DMCR JSON logs с virtualization-style
  `err` attribute;
- не править десятки call sites вручную, а держать нормализацию в общих
  logger handlers.

Файлы:

- `images/controller/internal/cmdsupport/*`;
- `images/dmcr/internal/logging/*`.

Проверки:

- `cd images/controller && go test ./internal/cmdsupport`;
- `cd images/dmcr && go test ./internal/logging`.

Артефакт:

- `slog.Any("error", err)` в runtime code выходит как `err` в structured log.

Статус: выполнено.

Проверки:

- `cd images/controller && go test ./internal/cmdsupport ./internal/monitoring/...` — passed.
- `cd images/dmcr && go test ./internal/logging` — passed.

### Slice 3. Signal review and next hardening backlog

Цель:

- проверить diff против virtualization principles;
- зафиксировать, что не вошло в первый slice.

Файлы:

- `plans/active/observability-signal-hardening/PLAN.ru.md`;
- при необходимости `NOTES.ru.md`.

Проверки:

- `git diff --check`;
- `review-gate`.

Артефакт:

- понятный остаток для следующего observability slice.

Статус: выполнено. Остаток зафиксирован в `NOTES.ru.md`.

### Slice 4. Next executable observability hardening

Цель:

- после rollout проверить live `/metrics` и alert rules на новые collector
  health metrics;
- выровнять оставшийся dataplane field dictionary (`duration_ms` /
  `duration_seconds`, digest/artifact/source fields);
- отдельно пройти DMCR direct-upload / GC logs на request/repository/phase
  consistency.

Файлы:

- `templates/**/prometheusrules*` или `monitoring/prometheus-rules/*`, если
  будут добавляться alerts;
- `images/controller/internal/dataplane/**`;
- `images/dmcr/internal/**`.

Проверки:

- targeted Go tests по затронутым packages;
- `make helm-template` и `make kubeconform`, если меняются alerts/templates;
- live scrape/e2e evidence в `live-e2e-ha-validation`.

Статус: partially completed in current pass.

Выполнено:

- blocked workload diagnostics moved from PodTemplate annotations to top-level
  workload annotations, so reason/message updates do not create extra
  Deployment template hash churn;
- blocked workload log/event dedupe now compares against the current persisted
  workload, including stale reconcile replay;
- DMCR main registry process now downgrades only expected manifest `DELETE`
  `404` misses (`MANIFEST_UNKNOWN`, `NAME_UNKNOWN`) from `error` to `info`
  while keeping read misses, blob misses, writes and 5xx errors at `error`.

Проверки:

- `cd images/controller && go test ./internal/controllers/workloaddelivery`
- `cd images/controller && go test ./...`
- `cd images/dmcr && go test ./internal/logging`
- `cd images/dmcr && go test ./...`
- `make verify`

Остаток для live proof:

- повторить live e2e scrape/log evidence after rollout and confirm no active
  ai-models/DMCR alerts;
- if noisy S3 SDK checksum warnings remain in the live image, split them into a
  dedicated backend logging cleanup slice.

## 5. Rollback Point

Откатить подключение `collectorhealth` и новый package целиком. Существующие
domain metrics и collectors останутся в прежнем состоянии.

## 6. Final Validation

- `cd images/controller && go test ./internal/monitoring/...`;
- `git diff --check`;
- `git diff --cached --check` если изменения уже staged.
