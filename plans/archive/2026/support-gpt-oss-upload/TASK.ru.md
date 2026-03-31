# TASK

## Контекст
Пользователь хочет загрузить в phase-1 registry конкретную HF-модель
`openai/gpt-oss-20b`, но не через ноутбук как data plane. Нужен корректный
in-cluster import path, который потом естественно переиспользуется контроллером
из `Model` / `ClusterModel`.

## Постановка задачи
Заменить current helper-path через локальную загрузку модели в память на
reusable import flow:
- backend-owned runtime script внутри image;
- manual Kubernetes Job для phase 1;
- thin local wrapper для тех случаев, когда локальный запуск всё ещё нужен.

## Scope
- runtime import script в `images/backend/scripts`;
- packaging backend image нужными lightweight HF deps;
- operator helper для запуска одноразового import Job в кластере;
- обновление локального helper на тот же import flow;
- краткое описание phase-1 operational UX и связи с будущим CRD/controller UX.

## Non-goals
- не вводить phase-2 `Model` / `ClusterModel`;
- не добавлять постоянный Helm-managed operational Job в module templates;
- не патчить upstream MLflow;
- не строить app-native catalog UX поверх backend UI.

## Затрагиваемые области
- `images/backend/*`
- `tools/*`
- `README*.md`
- `plans/active/support-gpt-oss-upload/*`

## Критерии приёмки
- существует backend-owned import entrypoint, который скачивает HF snapshot,
  логирует local checkpoint в MLflow и регистрирует модель без `pipeline(...)`
  и без загрузки весов в RAM;
- существует phase-1 helper для запуска in-cluster Job с использованием уже
  deployed backend image;
- локальный `upload_hf_model.py` остаётся рабочим, но использует тот же import
  flow, что и cluster Job;
- для `openai/gpt-oss-20b` есть явный рекомендованный путь через Job;
- проверки проходят.

## Риски
- для больших моделей основным ограничением станет ephemeral storage Job-пода,
  а не память; это нужно явно отразить в helper usage;
- расширение backend image дополнительными HF-библиотеками может увеличить его
  размер, но это допустимо для phase 1, если позволяет переиспользовать один и
  тот же runtime entrypoint позже в controller-owned Jobs.
