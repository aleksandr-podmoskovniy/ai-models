# PLAN

## Current phase

Основной проект находится на этапе 2 (`Model` / `ClusterModel` и controller
publication flow), но этот slice берёт один ограниченный элемент из hardening
для собственного Go controller image, потому что текущий `werf` runtime shell
объективно неверный и мешает нормальному production baseline.

## Orchestration mode

`light`

Read-only subagents до кодовых изменений:
- repo/build-shell check по distroless base conventions
- runtime/deployment check по distroless readiness контроллера

## Architecture acceptance criteria

- не смешивать fix controller runtime shell с backend hardening
- не добавлять новый runtime shell drift или ad-hoc workaround в templates
- сохранить thin module shell: только build/publish/runtime concerns этого image

## Slices

### Slice 1. Зафиксировать bounded task

Цель:
- оформить отдельный bundle на controller distroless runtime shell

Файлы:
- `plans/active/fix-controller-distroless-runtime-shell/TASK.ru.md`
- `plans/active/fix-controller-distroless-runtime-shell/PLAN.ru.md`

Проверки:
- ручная сверка bundle против `AGENTS.md`

Результат:
- есть явный scope, non-goals, acceptance criteria и rollback point

### Slice 2. Сверить repo conventions и runtime requirements

Цель:
- подтвердить корректный distroless base и минимально необходимые runtime
  условия для controller

Файлы:
- read-only inspection `build/base-images/*`, `images/controller/*`,
  `templates/controller/*`
- при необходимости выводы фиксируются в bundle

Проверки:
- read-only subagent findings

Результат:
- понятен target shape final image

### Slice 3. Исправить controller final image

Цель:
- заменить неверный final runtime base и оставить controller runnable

Файлы:
- `images/controller/werf.inc.yaml`
- при необходимости `templates/controller/deployment.yaml`

Проверки:
- `go test ./...` в `images/controller`
- `make helm-template`
- `make kubeconform`

Результат:
- controller final image собран на distroless base и не требует alpine runtime

### Slice 4. Закрыть bundle и зафиксировать следующий code step

Цель:
- зафиксировать, что после этого делаем дальше по коду

Файлы:
- `plans/active/fix-controller-distroless-runtime-shell/PLAN.ru.md`
- `plans/active/fix-controller-distroless-runtime-shell/REVIEW.ru.md`

Проверки:
- `make verify`
- `git diff --check`

Результат:
- bundle закрывает slice и содержит явный next step

## Rollback point

Если distroless runtime требует большего числа template/runtime правок, чем
ожидалось, безопасный rollback point: оставить только bundle и read-only выводы,
не менять `images/controller/werf.inc.yaml`.

## Final validation

- `go test ./...` в `images/controller`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`
