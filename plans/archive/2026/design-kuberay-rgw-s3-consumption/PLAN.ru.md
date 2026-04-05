# План: дизайн доступа KubeRay к model artifacts в RGW S3

## Current phase

Этап 1. Managed backend inside the module. Задача не меняет backend shell `ai-models`, а уточняет integration contract между внутренним model catalog backend и внешним serving plane.

## Slices

### Slice 1. Зафиксировать текущий разрыв между `ai-models` и `RayService`

Цель:
- понять, как именно сейчас настроен `RayService` и чего в нём не хватает для S3-backed model source.

Файлы/области:
- `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s.apiac.ru/kube-ray/charts/ray-service/ap-values.yaml`
- `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s.apiac.ru/kube-ray/charts/ray-service/01-ServiceAccount.yaml`

Проверки:
- `sed` / `rg` по values и service account.

Результат:
- список текущих HF-only настроек и отсутствующих S3/RGW параметров.

### Slice 2. Подтвердить каноничный upstream path для `Ray Serve LLM` и `Ceph RGW`

Цель:
- сверить предлагаемую интеграцию с официальной документацией.

Файлы/области:
- официальные docs `docs.ray.io`
- официальные docs `docs.ceph.com`

Проверки:
- поиск и чтение релевантных источников.

Результат:
- подтверждённый контракт для `S3`/object storage и аутентификации.

### Slice 3. Подготовить concrete config для `RayService`

Цель:
- описать, что конкретно надо добавить в values: env, secret refs, CA mount, model source.

Файлы/области:
- `ap-values.yaml`
- при необходимости `01-ServiceAccount.yaml`

Проверки:
- визуальная согласованность head/worker wiring.

Результат:
- готовый patch или псевдо-patch, который можно применить без догадок.

### Slice 4. Зафиксировать рекомендованный prod path

Цель:
- отделить current practical path от future hardening.

Файлы/области:
- `plans/active/design-kuberay-rgw-s3-consumption/*`

Проверки:
- итоговая инженерная сводка.

Результат:
- ясный ответ:
  - что делать сейчас;
  - что делать позже;
  - почему `MLflow SSO` не участвует в serving auth.

## Rollback point

Если upstream `Ray` contract для прямого `s3://` model source окажется другим, остановиться на документированном integration design без изменения внешнего `RayService` values.

## Final validation

- Проверить, что bundle отражает текущую фазу и не тащит в себя phase-2 API.
- Если будет patch values-файла, убедиться, что head и workers получают одинаковые storage credentials и CA wiring.
