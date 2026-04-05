## 1. Заголовок
Объяснить serving contract ai-models/MLflow и проверить STS support в Ceph RGW.

## 2. Контекст
Пользователь разбирается, как `ai-models` должен жить поверх MLflow backend, как выдавать model artifact URI в serving системы и насколько Ceph RGW 19.2.2 подходит для short-lived machine credentials. Параллельно нужно объяснить физический смысл `Experiment` / `Run` и production promotion в MLflow.

## 3. Постановка задачи
Проверить по официальным источникам поддержку STS в Ceph RGW 19.2.2, объяснить откуда брать artifact URI для inference, и разложить MLflow object model и production-promotion semantics в терминах реального MLOps workflow.

## 4. Scope
- Проверить Ceph RGW STS по официальной документации.
- Сопоставить это с текущим ai-models serving contract.
- Объяснить, что такое `Experiment` / `Run` / `Logged Model` / `Registry`.
- Объяснить, как MLflow обычно использует evaluation, benchmarks и promotion.

## 5. Non-goals
- Не менять сейчас code/runtime модуля.
- Не проектировать детально phase-2 API.
- Не обещать интеграции, которых в текущем модуле ещё нет.

## 6. Затрагиваемые области
- `plans/active/explain-mlflow-serving-and-ceph-sts/`
- live knowledge from repo
- official MLflow / Ceph docs

## 7. Критерии приёмки
- Есть точный ответ про STS в Ceph RGW 19.2.2.
- Есть понятный ответ, откуда брать serving URI.
- Есть простое объяснение production promotion и роли Experiments / Runs.

## 8. Риски
- Часть MLflow guidance зависит от текущей версии upstream docs; нужно опираться на официальные источники и явно отделять наш phase-1 contract от общих возможностей MLflow.
