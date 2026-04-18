## 1. Current phase

Это узкий repair slice внутри уже landed phase-2 controller/runtime baseline.
Новый runtime behavior не вводится; чинится live build shell.

## 2. Orchestration

`solo`

Задача узкая и механически локализована:

- проблема уже сведена к одному `werf.inc.yaml`;
- architectural ambiguity нет;
- read-only delegation здесь не добавит сигнала.

## 3. Slices

### Slice 1. Restore the missing YAML document boundary

Цель:

- вернуть корректный YAML document separator между
  `controller-build-artifact` и final image definitions.

Файлы/каталоги:

- `images/controller/werf.inc.yaml`

Проверки:

- `werf config render >/tmp/ai-models-werf-render.yaml`

Артефакт результата:

- `werf` больше не склеивает build artifact и final image в один YAML map.

### Slice 2. Validate the repaired live build shell

Цель:

- подтвердить, что regression действительно закрыта и repo-level guards не
  нашли дополнительный drift.

Файлы/каталоги:

- touched files from slice 1

Проверки:

- `make verify`
- `git diff --check`

Артефакт результата:

- CI render failure больше не воспроизводится локально;
- финальный diff не содержит syntax/whitespace drift.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: правка ограничена document
separator и не меняет runtime semantics image stages.

## 5. Final validation

- `werf config render >/tmp/ai-models-werf-render.yaml`
- `make verify`
- `git diff --check`
