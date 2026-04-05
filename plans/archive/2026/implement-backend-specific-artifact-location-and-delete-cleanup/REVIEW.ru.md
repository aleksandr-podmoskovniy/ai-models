# REVIEW

## Findings

Критичных блокеров по текущему slice не найдено.

Final reviewer pass подтвердил ещё один важный lifecycle guardrail:

- cleanup finalizer больше не вешается безусловно на каждый `Model` /
  `ClusterModel`, а появляется только когда у объекта уже есть валидный
  internal cleanup handle.

Review-gate по финальному diff подтвердил:

- public status отвязан от OCI-only shape и теперь остаётся backend-neutral:
  `status.artifact` отражает только `kind`, `uri`, optional `digest`,
  `mediaType`, `sizeBytes`;
- internal cleanup details не протекли в public API:
  backend-specific delete execution держится в controller-side
  `cleanup-handle` annotation и в cleanup Job contract;
- minimal live delete path действительно появился:
  `Model` и `ClusterModel` получают finalizer, на delete переходят в
  `phase=Deleting`, а для MLflow handle controller создаёт cleanup Job через
  существующую backend script boundary;
- always-local runtime delivery пока остаётся только internal controller
  contract и не превращён в public status или raw runtime UX.

## Scope check

- `api/core/v1alpha1/*` обновлён под generalized artifact location и stable
  cleanup condition/reasons.
- `images/controller/internal/publication` и
  `images/controller/internal/managedbackend` переведены с OCI-only reference на
  backend-specific artifact locator.
- `images/controller/internal/runtimedelivery` зафиксирован как
  local-materialized delivery plan независимо от consumer kind.
- Добавлены bounded controller packages:
  - `internal/cleanuphandle`
  - `internal/cleanupjob`
  - `internal/modelcleanup`
- Bootstrap wiring в `cmd/` и `internal/app` теперь достаточно для live
  delete-only controller path, но slice не расползся в publish/sync/materializer
  reconciliation.

## Checks

Пройдены:

- `go generate ./...` в `api`
- `bash scripts/verify-crdgen.sh` в `api`
- `go test ./...` в `api`
- `go test ./...` в `images/controller`
- `make fmt`
- `make test`
- `make verify`
- `git diff --check`

## Residual risks

- Cleanup на delete сейчас реально исполним только если объект уже несёт
  internal cleanup handle annotation. Следующий publish/sync slice должен стать
  владельцем её заполнения и обновления.
- Live cleanup path в этой итерации закрыт только для MLflow-shaped handle через
  backend cleanup Job. OCI/Harbor/internal-registry cleanup пока сознательно
  оставлен следующим slice.
- Controller стартует только при валидной cleanup Job config. Это нормально для
  working baseline, но module manifests ещё должны аккуратно замкнуть эти env на
  deployment values.
- `phase=Deleting` и `CleanupCompleted=False` используются как observable delete
  lifecycle, но финальное `CleanupCompleted=True` состояние объект уже не
  публикует, потому что после успешного cleanup finalizer снимается и CR
  удаляется.

## Next step

Следующий нормальный slice:

- publication/sync owner, который записывает cleanup handle на published object,
  обновляет `status.artifact` из live publication result и затем расширяет
  cleanup executors на OCI-backed artifact locations.
