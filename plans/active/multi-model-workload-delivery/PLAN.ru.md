# План

## Current phase

Этап 2: Distribution and runtime topology.

Задача относится к workload-facing runtime delivery: модель уже опубликована,
а workload должен получить один или несколько стабильных локальных путей.

## Orchestration

Режим: `solo`.

Причина: первый slice ограничен existing controller/adapters boundary и не
меняет CRD/public `Model` API. Если после этого потребуется admission policy
для persona-specific use of `ClusterModel`, это отдельный API/RBAC slice.

## Slices

### Slice 1. Contract and parser

- Добавить `ai.deckhouse.io/model-refs`.
- Поддержать `alias=Model/name` и `alias=ClusterModel/name`.
- Оставить старые annotations как single binding с alias `model`.
- Проверки: parser tests.

### Slice 2. Multi-resolution and reconcile handoff

- Резолвить список references в ordered bindings.
- Pending оставить pending, если хотя бы один binding не Ready.
- Логи/events показывают count и primary digest.
- Проверки: reconcile apply/pending tests.

### Slice 3. Modeldelivery render/apply

- Добавить multi-binding render.
- Bridge: один cache root, несколько init containers, shared-store mode,
  alias symlink `/data/modelcache/models/<alias>`.
- SharedDirect: отдельные CSI volumes mounted to
  `/data/modelcache/models/<alias>`.
- Runtime env: primary compatibility + named model env.
- Cleanup: удалить stale managed env/init/volumes/annotations.
- Проверки: modeldelivery render/apply/cleanup tests.

### Slice 4. Materializer alias link

- Добавить optional env/flag для alias link.
- Ссылка создаётся атомарно и не трогает global current/model link.
- Проверки: materialize unit tests and nodecache layout tests.

### Slice 5. Webhook and docs

- Расширить webhook match condition на `ai.deckhouse.io/model-refs`.
- Обновить docs/README/TEST_EVIDENCE.
- Проверки: helm template targeted.

## Rollback point

До применения в live кластере старый single-model contract остаётся полностью
совместимым. Если multi-model path даёт regressions, можно удалить только новую
annotation `ai.deckhouse.io/model-refs`; старые workloads продолжат работать.

## Final validation

- `go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery ./internal/nodecache`
- `go test ./cmd/ai-models-artifact-runtime`
- `make helm-template` или targeted render для controller webhook
- `make fmt`

## Выполненная проверка

- `go test -count=1 ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery ./internal/adapters/k8s/nodecacheruntime ./internal/nodecache ./cmd/ai-models-artifact-runtime`
- `make verify`

## Follow-up hardening

После первичного review найден append-only drift в apply-path:

- при переходе `model-refs` -> legacy single model могли оставаться старые
  per-alias `AI_MODELS_MODEL_<ALIAS>_*` env;
- при уменьшении списка aliases могли оставаться env для удалённой модели;
- при переходе multi SharedDirect -> single/bridge могли оставаться старые
  managed CSI volumes/mounts;
- при переходе registry-backed materializer -> SharedDirect мог оставаться
  orphan `registry-ca` volume.
- CSI node service проверял готовность `store/<digest>/model`, но bind mount
  делал на `store/<digest>`, из-за чего workload мог видеть служебный root
  вместо каталога модели.
- Public workload env `AI_MODELS_MODELS` не должен повторять internal resolved
  annotation с DMCR artifact URI.
- Multi-model SharedDirect не должен молча добавлять module-managed mount в
  path, уже занятый workload'ом другим volumeMount.

Новый slice: сделать runtime delivery apply exact reconciliation для
module-managed env/volumes/annotations, bind-mount'ить только model directory,
разделить public env manifest и internal node-cache handoff annotation, и
закрыть regression tests на transition/conflict paths.

Проверка follow-up slice:

- `go test -count=1 ./internal/dataplane/nodecachecsi ./internal/adapters/k8s/modeldelivery ./internal/controllers/workloaddelivery ./internal/adapters/k8s/nodecacheruntime ./internal/nodecache ./cmd/ai-models-artifact-runtime`
- `make fmt`
- `make verify`
- `git diff --check`
