# Исправить distroless runtime shell для controller image

## Контекст

`images/controller/werf.inc.yaml` сейчас собирает бинарь корректно, но final image
остаётся на `builder/alpine`. Для собственного Go controller это неверный runtime
shape: лишний shell/runtime мусор, более широкая поверхность атаки и drift от
ожидаемого production baseline.

При этом задача узкая: нужно исправить именно runtime shell контроллера, не
переходя к широкому hardening backend image и не смешивая это с feature work по
publication/runtime materialization.

## Постановка задачи

Перевести controller final image на корректный distroless base по repo
conventions, убедиться, что entrypoint, non-root execution, probes и metrics
остаются рабочими, и зафиксировать, что является следующим кодовым шагом после
этого slice.

## Scope

- `images/controller/werf.inc.yaml`
- при необходимости связанные controller docs/bundles
- минимальные правки controller deployment/runtime shell, если distroless этого
  требует
- фиксация следующего code slice после distroless

## Non-goals

- не переводить backend image на distroless
- не менять public API / CRD
- не продолжать feature work по `authSecretRef`, materializer agent или
  payload/backend integration в этом slice
- не делать общий hardening всего репозитория

## Затрагиваемые области

- `images/controller/*`
- `templates/controller/*` только если нужен минимальный runtime fix
- `plans/active/fix-controller-distroless-runtime-shell/*`

## Критерии приёмки

- final image controller больше не использует `builder/alpine`
- final image опирается на repo-supported distroless base
- controller по-прежнему стартует как non-root с тем же бинарём и entrypoint
- `helm-template`, `kubeconform` и `make verify` проходят
- bundle фиксирует следующий code step после этого slice

## Риски

- distroless base может требовать более явной работы с CA trust или filesystem
- можно случайно сломать runtime assumptions controller manager
- можно незаметно начать phase-3 hardening шире, чем нужно для текущего slice
