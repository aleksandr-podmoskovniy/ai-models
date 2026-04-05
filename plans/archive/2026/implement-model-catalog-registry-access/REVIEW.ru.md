# REVIEW

## Findings

Критичных блокеров по текущему slice не найдено.

Внешний reviewer pass дал два medium-level замечания, оба закрыты в этой же
итерации:

- `accessplan` теперь валидирует, что published target действительно совпадает с
  identity конкретного `Model` / `ClusterModel`, а не только с scope/kind.
- `payloadregistry.Render` теперь отклоняет пустой repository pattern и пустой
  subject set; добавлены отрицательные tests на оба случая.

Повторный read-only reviewer pass после фиксов критичных блокеров не оставил.

## Scope check

- `images/controller/` перестал быть documentation-only и получил один
  module-local `go.mod` на корне.
- `cmd/ai-models-controller` остался thin bootstrap shell и не утащил внутрь
  domain logic.
- Path/access semantics вынесены в bounded `internal/*` packages:
  - `internal/registrypath`
  - `internal/accessplan`
  - `internal/payloadregistry`
  - `internal/app`
- `pkg/*`, reconcile loops, watch wiring, upload lifecycle и live registry RBAC
  materialization в этот slice не протекли.

## Checks

Пройдены:

- `go test ./...` в `images/controller/`
- `make fmt`
- `make test`
- `git diff --check`

Дополнительно закрыто reviewer-замечаниями через новые negative tests:

- mismatch object-vs-target identity в `internal/accessplan`
- empty repository / empty subject set в `internal/payloadregistry`

## Residual risks

- `cmd/ai-models-controller` пока intentionally даёт только bootstrap shell без
  controller-runtime manager и reconcile wiring. Это нормально для текущего
  slice, но следующий runtime slice должен поверх этого добавить реальный
  process lifecycle.
- `internal/payloadregistry` останавливается на rendered intent и не
  materialize'ит `Role` / `RoleBinding`. Следующий controller slice должен
  аккуратно замкнуть этот шаг без размывания boundaries между semantics и live
  cluster writes.
- `CapabilityPull` сейчас маппится только в verb `get`. Если в следующих flows
  понадобится tag browsing через Kubernetes API extension `payload-registry`,
  нужно будет отдельно решить, добавлять ли `list`.

## Next step

Следующий нормальный slice по implementation order:

- upload session lifecycle поверх уже зафиксированных registry path conventions
  и access planning primitives.
