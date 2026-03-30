# Добить make verify и снять зависание kubeconform

## Контекст
После расширения fixture matrix `make helm-template` стал рендерить несколько
сценариев, но `make verify` всё ещё не доводится до зелёного состояния, потому
что шаг `make kubeconform` локально подвисает и не завершает проход. Это делает
repo-level verify loop непредсказуемым и ломает определение done для текущих
срезов.

## Постановка задачи
Нужно локализовать источник зависания `kubeconform` в текущем verify loop и
починить `make verify` так, чтобы он завершался детерминированно и продолжал
валидировать полезную часть render outputs.

## Scope
- диагностировать зависание в `tools/kubeconform/kubeconform.sh`;
- при необходимости скорректировать schema source, timeout, batching или
  preflight-проверки;
- обновить docs только если меняется контракт local verify behavior;
- прогнать `make verify` до завершения.

## Non-goals
- не переписывать весь render/test pipeline;
- не добавлять e2e/integration tests вне текущего verify loop;
- не менять runtime templates без необходимости, если проблема в tooling.

## Затрагиваемые области
- `tools/kubeconform/*`
- `Makefile` при необходимости
- `DEVELOPMENT.md` при необходимости
- `plans/active/fix-make-verify-kubeconform-hang/*`

## Критерии приёмки
- причина зависания локализована и зафиксирована;
- `make kubeconform` завершается детерминированно;
- `make verify` завершается детерминированно;
- validate coverage для built-in Kubernetes ресурсов не деградировала.

## Риски
- hanging может быть связан с внешним schema fetch и потребовать offline/local
  schema strategy;
- неаккуратный workaround может превратить `kubeconform` в формальную, но
  бесполезную проверку.
