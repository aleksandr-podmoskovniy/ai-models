## 1. Current phase

Этап 1: publication/runtime baseline. Изменение касается только placement
module-owned runtime Pods и не меняет public `Model` / `ClusterModel` API.

## 2. Orchestration

`solo`

Причина:

- решение механически повторяет Deckhouse/virtualization pattern;
- изменение ограничено templates/render validation;
- в текущем запросе нет разрешения на subagents, а tool policy запрещает
  запускать их без явного текущего запроса.

## 3. Slices

### Slice 1. Сравнить текущий placement с virtualization

Цель:

- подтвердить, какие helpers использует `virtualization-controller`;
- подтвердить, как `dvcr` размещается на system nodes без hard master fallback;
- выбрать соответствующее соответствие для `ai-models`.

Файлы/каталоги:

- read-only: соседний repo `virtualization`;
- `plans/active/runtime-placement-virtualization-style/*`.

Проверки:

- manual source inspection.

### Slice 2. Перевести ai-models templates на helm-lib placement

Цель:

- заменить локальные placement helpers в controller/DMCR deployments;
- удалить больше не используемые локальные helpers;
- при необходимости добавить global discovery defaults в render fixtures.

Файлы/каталоги:

- `templates/_helpers.tpl`;
- `templates/controller/deployment.yaml`;
- `templates/dmcr/deployment.yaml`;
- `fixtures/module-values.yaml`;
- `tools/helm-tests/validate-renders.py`.

Проверки:

- `make helm-template`;
- targeted render inspection for `Deployment/dmcr`.

### Slice 3. Зафиксировать validation evidence

Цель:

- доказать, что render больше не жёстко садит `DMCR` на control-plane при
  отсутствии system nodes.

Файлы/каталоги:

- `plans/active/runtime-placement-virtualization-style/NOTES.ru.md`.

Проверки:

- `git diff --check`;
- `make helm-template`.

## 4. Rollback point

До изменения templates: можно остановиться после source inspection без влияния
на repo state.

## 5. Final validation

- `make helm-template`;
- `git diff --check`;
- review-gate checklist по scope/templates/render.
