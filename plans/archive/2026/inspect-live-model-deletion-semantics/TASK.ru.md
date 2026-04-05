## 1. Заголовок
Проверить live-удаление моделей в MLflow и объяснить delete semantics.

## 2. Контекст
В `ai-models` уже есть phase-1 baseline с внутренним MLflow backend, SSO, workspaces и import в S3. Пользователь видит, что удаление модели в UI работает не так, как ожидается: часть сущностей пропадает, часть остаётся в `Experiments` / `Runs` / `Models`, а часть артефактов может жить в S3. Нужно отделить текущий live-gap от нормальной upstream семантики MLflow.

## 3. Постановка задачи
Проверить текущее состояние удаления модели в живом кластере и дать объяснение, как в MLflow связаны `Run`, `Logged Model` и `Registered Model Version`, почему удаление разложено по нескольким сущностям и что именно сейчас не работает или не доведено до конца в live install.

## 4. Scope
- Снять live state через cluster/API.
- Проверить, что удалено в registry, logged-model layer и run layer.
- Сопоставить observed behavior с upstream MLflow semantics.
- Дать пользователю практичное объяснение, не меняя сейчас platform UX.

## 5. Non-goals
- Не проектировать phase-2 `Model` / `ClusterModel`.
- Не менять сейчас upstream MLflow semantics.
- Не внедрять новые delete APIs в UI в рамках этого диагностического среза.

## 6. Затрагиваемые области
- `plans/active/inspect-live-model-deletion-semantics/`
- live cluster state
- upstream MLflow semantics из `.cache/backend-upstream/`

## 7. Критерии приёмки
- Зафиксировано, что именно удалено и что осталось в live cluster.
- Пользователю объяснено, почему в MLflow есть отдельные слои `Runs`, `Models`, `Model Registry`.
- Ясно сказано, является ли текущая проблема багом модуля, особенностью upstream или комбинацией обоих.

## 8. Риски
- Live state уже мог измениться из-за ручных действий пользователя, и тогда диагностика покажет только текущее состояние, а не исторический момент поломки.
