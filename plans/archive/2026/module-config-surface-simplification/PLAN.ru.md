# План

## Current phase

Этап 2: Distribution and runtime topology. Задача режет публичную поверхность
configuration, не меняя сам runtime byte-path.

## Orchestration

`solo`.

По правилам репозитория values/OpenAPI/storage обычно требуют delegation, но
в текущем turn пользователь не дал явного разрешения на subagents; системное
ограничение запрещает spawn без явного запроса. Риск компенсируется узким
slice и `review-gate`.

## Active bundle disposition

- `capacity-cache-admission-hardening` — keep: отдельный workstream capacity /
  cache admission.
- `live-e2e-ha-validation` — keep: отдельный live E2E workstream.
- `observability-signal-hardening` — keep: отдельный observability workstream.
- `pre-rollout-defect-closure` — keep: отдельный rollout defect workstream.
- `ray-a30-ai-models-registry-cutover` — keep: отдельный RayService cutover
  workstream.
- `source-capability-taxonomy-ollama` — keep: отдельный source metadata /
  Ollama workstream, сейчас имеет локальные незакоммиченные изменения.
- `module-config-surface-simplification` — current: этот slice.

## Slices

### 1. Схема и defaults

- Файлы: `openapi/config-values.yaml`, `openapi/values.yaml`.
- Действие: удалить public `dmcr`, удалить public
  `artifacts.sourceFetchMode`, заменить verbose `nodeCache` на
  `enabled`, `size`, `nodeSelector`, `blockDeviceSelector`.
- Проверки: `git diff --check`, render.

### 2. Helm runtime defaults

- Файлы: `templates/_helpers.tpl`.
- Действие: internal DMCR schedule оставить фиксированным default;
  `sourceFetchMode` сделать module-owned `direct`; node-cache storage names
  оставить module-owned constants; shared cache volume size = public
  `nodeCache.size`; default selectors = `ai.deckhouse.io/model-cache=true`.
- Проверки: `make helm-template`.

### 3. Docs

- Файлы: `docs/CONFIGURATION.ru.md`, `docs/CONFIGURATION.md`,
  точечные README/evidence refs.
- Действие: зафиксировать новый компактный contract и убрать объяснение
  удалённых knobs.
- Проверки: `git diff --check`.

## Rollback point

Откатить изменения этого bundle в `openapi/`, `templates/_helpers.tpl` и docs:
runtime code не меняется, поэтому rollback не требует миграции данных.

## Final validation

- `make helm-template`;
- `make kubeconform`;
- `git diff --check`;
- `review-gate`.

## Выполнено

- `openapi/config-values.yaml` больше не содержит public `dmcr` и
  `artifacts.sourceFetchMode`.
- `nodeCache` сокращён до `enabled`, `size`, `nodeSelector`,
  `blockDeviceSelector`; default label contract:
  `ai.deckhouse.io/model-cache=true`.
- Helm helpers оставляют DMCR schedule, source policy и storage object names
  internal defaults.
- `make helm-template` прошёл.
- `make kubeconform` прошёл.
- `make verify` прошёл.
- `git diff --check` прошёл.
