# PLAN: разобрать задержку удаления namespace d8-ai-models

## Current phase

Этап 1. Внутренний managed backend. Задача ограничена lifecycle cleanup path
модуля на живом кластере.

## Slices

### Slice 1. Снять live delete state
- Цель: собрать фактическое состояние namespace finalization path.
- Области:
  - `kubectl get/describe/events`
- Проверки:
  - live namespace conditions
  - список оставшихся namespaced ресурсов
- Артефакт:
  - список конкретных объектов/условий, которые держали `d8-ai-models` в
    `Terminating`.

### Slice 2. Интерпретировать причину
- Цель: отделить штатный cleanup Kubernetes/Deckhouse от module-specific blocker'ов.
- Области:
  - cluster events
  - связанные module-owned объекты при необходимости
- Проверки:
  - согласованная картина delete sequence
- Артефакт:
  - вывод, является ли задержка штатной или требует module-side исправления.

### Slice 3. Зафиксировать выводы
- Цель: оставить понятный handoff для пользователя.
- Области:
  - `plans/active/debug-namespace-termination-delay/*`
- Проверки:
  - bundle отражает реальные выводы
- Артефакт:
  - понятное explanation + next step при необходимости.

## Rollback point

После Slice 1, до каких-либо code changes. На этом шаге можно остановиться с
чистым diagnostic output без изменения модуля.

## Orchestration mode

solo

## Final validation

- узкие cluster reads
- repo-level проверки не нужны, если код не менялся

## Findings

- На момент диагностики `Namespace/d8-ai-models` уже дошёл до полного удаления:
  `kubectl get ns d8-ai-models` возвращал `NotFound`.
- Cluster-wide не осталось module-owned объектов `ai-models`, которые могли бы
  продолжать держать namespace:
  - `Postgres.managed-services.deckhouse.io` для `ai-models` отсутствует;
  - `DexClient`, `Certificate`, `Ingress` для `ai-models` отсутствуют;
  - namespaced secrets бывшего namespace уже удалены вместе с namespace.
- В cluster-wide выводе по secret'ам встречаются только нерелевантные
  `ai-models-runners` в `arc-*`, не относящиеся к модулю `d8-ai-models`.
- Из фактического состояния не видно module-specific blocker'а удаления.
  Наиболее вероятная причина наблюдавшейся задержки — финальная стадия штатного
  namespace cleanup Kubernetes/Deckhouse после удаления содержимого и
  связанных generated objects.

## Conclusion

Для текущего эпизода evidence указывает не на застрявший finalizer в самом
модуле, а на обычное дожатие удаления namespace. Module-side fix по этим данным
не требуется; если задержка воспроизведётся снова, нужно снимать `kubectl get
ns -o yaml` и список remaining namespaced objects до того, как namespace успеет
исчезнуть.
