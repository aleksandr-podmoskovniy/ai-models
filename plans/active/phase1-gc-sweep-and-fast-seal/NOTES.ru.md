## 1. Итог текущего continuation slice

В этом bundle закрыты оба заявленных phase-1 долга:

1. Productized stale sweep для `DMCR`:
   - появился public module contract `dmcr.gc.schedule` с default daily
     schedule `0 2 * * *`;
   - `dmcr-cleaner` теперь умеет `gc check` и `gc auto-cleanup`;
   - stale discovery сравнивает live `Model` / `ClusterModel` cleanup handles с
     фактическими repository/source-mirror prefix в storage;
   - maintenance `gc run` теперь перед `registry garbage-collect` делает тот
     же stale sweep, а schedule enqueue встроен в существующий secret-driven
     maintenance choreography без нового параллельного GC path;
   - для stale discovery и operator-facing commands добавлен cluster-scope RBAC
     на list/get `models` и `clustermodels`.
2. No-copy sealing для controller-owned publication path:
   - `DMCR direct-upload` больше не делает второй полный storage-side copy
     после `CompleteMultipartUpload(...)`;
   - continuation `plans/active/dmcr-zero-trust-ingest` заменил доверие к
     controller-provided digest на `DMCR`-owned verification read;
   - актуальная целевая картина: нет второй полной записи объекта, но есть
     один проверочный проход чтения по physical upload object.

## 2. Основные затронутые области

- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/cmd/dmcr-cleaner/cmd/gc.go`
- `images/dmcr/internal/directupload/service.go`
- `templates/dmcr/deployment.yaml`
- `templates/dmcr/rbac.yaml`
- `templates/_helpers.tpl`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `images/dmcr/README.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

## 3. Проверки

Узкие проверки:

- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/... ./internal/directupload/...`

Repo-level:

- `make verify`

Обе проверки прошли успешно локально.

## 4. Residual risk

- Закрыто continuation slice 2026-04-23: `dmcr-cleaner gc run` получил
  внутренний lease-based executor ownership на
  `coordination.k8s.io/Lease/dmcr-gc-executor`.
- Lease holder выполняет scheduled enqueue, arm/delete request secrets и
  active `auto-cleanup`; non-holder replica остаётся standby и не мутирует GC
  state.
- Lease tuning остаётся internal runtime detail и не выводится в public
  module settings.

## 5. Continuation checks 2026-04-23

Узкая проверка:

- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`

Repo-level:

- `make fmt`
- `make verify`

Результат:

- прошла успешно локально.
- `make verify` прошёл успешно локально; cluster rollout/validation не
  выполнялись.

## 6. Continuation slice 2026-04-23: orphan direct-upload GC

Read-only review inputs, зафиксированные до реализации:

- `backend_integrator`:
  - orphan direct-upload cleanup должен жить только внутри `dmcr-cleaner` как
    третья internal GC категория;
  - published physical blobs остаются под authority registry GC через
    `sealeds3.Delete()`;
  - authoritative live signal для physical object — только `.dmcr-sealed`
    metadata, не controller state.
- `integration_architect`:
  - storage inventory boundary нужно расширить до small-metadata reads и
    object timestamps, иначе безопасный orphan sweep невозможен;
  - public/operator semantics остаются на `gc check` / `gc auto-cleanup`
    без нового public knob;
  - orphan sweep должен быть fail-closed и не превращаться в generic bucket
    janitor.

- Productized stale sweep расширен на третью internal DMCR-owned категорию:
  orphan direct-upload prefixes под
  `_ai_models/direct-upload/objects/<session-id>`.
- `dmcr-cleaner gc check` и `gc auto-cleanup` теперь:
  - отдельно считают stored/referenced direct-upload prefixes;
  - находят orphan только если prefix не защищён ни одной валидной
    `.dmcr-sealed` metadata reference;
  - удаляют только такие orphan prefixes, если они старше bounded internal
    stale-age window `DMCR_DIRECT_UPLOAD_SESSION_TTL + activation delay`;
  - не трогают canonical blob paths и repository links.
- Published heavy blobs по-прежнему удаляются только через registry
  `garbage-collect` и `sealeds3.Delete()`, а не новым prefix sweeper.
- Metadata inventory сделан fail-closed:
  - если `.dmcr-sealed` metadata не читается или не парсится, весь orphan
    direct-upload report/cleanup slice падает ошибкой и не пытается делать
    age-only deletion.

Основные затронутые области:

- `images/dmcr/internal/garbagecollection/storage_s3.go`
- `images/dmcr/internal/garbagecollection/directupload_inventory.go`
- `images/dmcr/internal/garbagecollection/directupload_inventory_test.go`
- `images/dmcr/internal/garbagecollection/cleanup.go`
- `images/dmcr/internal/garbagecollection/report.go`
- `images/dmcr/internal/garbagecollection/runner.go`
- `images/dmcr/README.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `openapi/config-values.yaml`

Проверки:

- `cd images/dmcr && go test ./internal/garbagecollection`
- `cd images/dmcr && go test ./internal/garbagecollection ./cmd/dmcr-cleaner/...`
- `git diff --check`
- `make verify`

Результат:

- все проверки прошли локально;
- cluster rollout/validation не выполнялись.
