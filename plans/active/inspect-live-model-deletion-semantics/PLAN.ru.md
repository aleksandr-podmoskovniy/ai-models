## 1. Current phase
Этап 1. Managed backend inside the module.

## 2. Slices

### Slice 1. Зафиксировать live state удаления
- Цель: проверить, что удалено в registry, logged model layer и run layer.
- Файлы/каталоги: live cluster, `plans/active/inspect-live-model-deletion-semantics/`.
- Проверки:
  - `kubectl get deploy,pods,jobs -n d8-ai-models`
  - MLflow API calls на registered models, model versions, logged models, runs
- Результат: понятная snapshot-картина текущего состояния.

### Slice 2. Сопоставить с upstream MLflow object model
- Цель: объяснить, почему удаление не является одной операцией.
- Файлы/каталоги: `.cache/backend-upstream/mlflow/*`, `plans/active/inspect-live-model-deletion-semantics/`.
- Проверки:
  - чтение upstream store/client code для delete semantics
- Результат: краткая схема `run -> logged model -> registered model version` и причины отсутствия cascade delete по умолчанию.

### Slice 3. Сформулировать live-gap модуля
- Цель: отделить upstream behavior от нашего runtime gap.
- Файлы/каталоги: live cluster, repo scripts/docs при необходимости.
- Проверки:
  - наличие/отсутствие cleanup entrypoint в live image
- Результат: точное объяснение, что в кластере сейчас не докатилось или не реализовано.

## 3. Rollback point
Задача диагностическая. Безопасная точка остановки — после фиксации live state без изменений cluster resources.

## 4. Final validation
- Проверить, что task bundle создан и отражает фактический scope.
- Дать пользователю итог по live cluster и по upstream delete semantics.
