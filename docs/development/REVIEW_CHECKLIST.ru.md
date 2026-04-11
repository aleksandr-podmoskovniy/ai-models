# Финальная проверка изменения

## Orchestration
- Для нетривиальной задачи есть актуальный task bundle в `plans/active/<slug>/`.
- Для задачи выбран оправданный orchestration mode: `solo`, `light` или `full`.
- Если задача не была в `solo`, нужные read-only subagents действительно были вызваны до реализации.
- Если использовался `solo`, это не скрывает multi-area или архитектурно рискованную задачу.
- Для substantial task финальный review не ограничился только субъективным summary исполнителя.

## Архитектура
- Изменение укладывается в текущий этап проекта.
- Нет смешения разных concerns в одних и тех же каталогах.
- Не появилось новых обходных путей мимо agreed workflow.

## DKP-модуль
- `module.yaml`, `Chart.yaml`, `openapi/`, templates и docs не разъехались.
- Значения и шаблоны согласованы.
- Не появилось случайных root-level файлов или каталогов.

## Internal Backend Integration
- Внутренний backend остаётся внутренним сервисом модуля, а не публичным API.
- Storage, auth, monitoring и logging подключены через платформенные механизмы.
- Нет временных решений без пояснения и без rollback story.
- Если изменение касается MLflow/DMCR/raw-ingest:
  - ясно, что является source of truth для published artifact;
  - ясно, что хранится только для audit/lineage;
  - нет слепой второй полной копии больших raw blobs без явной причины.

## API и контроллеры
- Если изменялись `Model` / `ClusterModel`, роли и ownership понятны.
- `spec` и `status` не смешаны.
- `conditions` и reasons выглядят стабильно и объяснимо.
- Если менялся controller bootstrap или manager entrypoint, root logger явно
  прокинут в `slog`, `controller-runtime/pkg/log` и `k8s.io/klog/v2`.

## Качество и сопровождение
- Изменение не тянет лишних фич из будущих этапов.
- Выполнены релевантные проверки.
- Документация обновлена вместе с кодом.
- По диффу видно, что проект стал понятнее, а не более лоскутным.
- Для любого data-plane/storage change есть точный ответ:
  - как идёт byte path end-to-end;
  - streaming он или materialized;
  - сколько полных копий может существовать одновременно;
  - где они живут;
  - какие memory/cpu/storage limits это ограничивают.
- Для любого "history/metadata" change есть точный ответ:
  - какие именно поля пишутся;
  - кто их читает;
  - зачем они нужны;
  - не превращаются ли они во второй status/source of truth.
