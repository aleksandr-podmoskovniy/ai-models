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
2. Fast sealing для controller-owned publication path:
   - `DMCR direct-upload` больше не делает второй полный storage-side reread
     после `CompleteMultipartUpload(...)`;
   - trusted digest/size теперь приходят с controller-owned publisher boundary;
   - docs явно фиксируют, что это private internal fast path, а не generic
     untrusted external upload guarantee.

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

- Этот slice не проектировал отдельный executor election для `dmcr-cleaner`
  между несколькими replica `DMCR`; stale sweep встроен в уже существующий
  maintenance flow и не добавляет новый parallel cleanup path, но отдельный
  HA-safe executor ownership остаётся потенциальной следующей hardening
  задачей, если его решат выносить в phase-1.5/phase-3 hardening.
