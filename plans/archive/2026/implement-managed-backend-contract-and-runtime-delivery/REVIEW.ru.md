# REVIEW

## Findings

Критичных блокеров по текущему slice не найдено.

Final reviewer pass поднял два medium-level замечания, оба закрыты в этой же
итерации:

- MLflow-specific fields вынесены из generic `MirrorPlan.Tags` /
  `MirrorPlan.Metadata` в отдельный backend-scoped `BackendData`, чтобы generic
  managed-backend contract не цементировал MLflow-shaped payload.
- Runtime delivery contract смягчён от PVC-specific выбора до
  shared-filesystem contract, чтобы следующий runtime/injection slice не был
  заранее прибит к конкретному storage object kind.

Повторный review-gate подтвердил:

- phase-2 граница сохранена:
  - public `Model` / `ClusterModel` API не менялся;
  - `MLflow` остался internal managed backend adapter, а не public contract;
  - canonical publish plane по-прежнему остаётся OCI registry;
- `internal/managedbackend` строится вокруг generic `PublicationSnapshot` /
  `MirrorRequest`, а не вокруг raw MLflow сущностей;
- `internal/runtimedelivery` остался внутренним controller/runtime contract и не
  протащил credentials, secret refs или pod mutation details в public status;
- bootstrap wiring в `cmd/` осталось thin shell и не превратилось в patchwork
  вокруг конкретного backend implementation.

## Scope check

- В controller module добавлены bounded internal packages:
  - `internal/publication`
  - `internal/managedbackend`
  - `internal/runtimedelivery`
- `MLflow` подключён как first concrete managed-backend adapter через internal
  planner factory и bootstrap config selection.
- Runtime delivery зафиксирован как internal plan:
  - `KServe` сохраняет native OCI path;
  - `KubeRay`, `vLLM` и generic runtimes по умолчанию идут через
    local-materialized path.
- Slice не протёк в:
  - live backend API calls;
  - actual import/publish workers;
  - pod mutation, sidecars, init-container materialization;
  - public status fields для backend/runtime internals.

## Checks

Пройдены:

- `go test ./...` в `images/controller/`
- `make fmt`
- `make test`
- `make verify`
- `git diff --check`

Дополнительно проверено новыми tests:

- backend factory selection для `mlflow` и `internal`
- MLflow mapping из generic publication snapshot в mirror plan
- end-to-end contract scenario: published artifact -> MLflow mirror ->
  runtime-delivery plan
- runtime-delivery defaults/overrides для `KServe`, `KubeRay`, `vLLM`
- publication snapshot validation
- reuse того же publication snapshot для placeholder `internal` backend
- bootstrap fail-fast при некорректной managed-backend config

## Residual risks

- `managed-backend` selection уже провязан в bootstrap shell, но deployment
  templates этого controller runtime ещё не существуют. Следующий runtime slice
  должен аккуратно замкнуть env/config wiring на module manifests.
- `internal/runtimedelivery` сейчас фиксирует controller-side intent и default
  local shared-filesystem path, но не materialize'ит фактические PodSpec/PVC
  objects. Это
  сознательно оставлено на следующий runtime/injection slice.
- MLflow adapter пока only-plan layer: нет live sync, idempotency handling,
  retry policy и reconciliation against backend drift.
- Publication snapshot и runtime delivery plan пока живут как library contract.
  Следующий шаг должен решить, какой orchestrator/worker boundary будет
  владельцем actual source download, metadata inspection и promotion report.

## Next step

Следующий нормальный slice:

- worker/runtime boundary для actual source import or upload promotion,
  metadata inspection и live backend mirror execution поверх уже зафиксированных
  `managedbackend` и `runtimedelivery` contracts.
