# Live e2e/HA validation after rollout

## 1. Заголовок

Проверить новую сборку `ai-models` в `k8s.apiac.ru` через HF/Upload e2e,
прерывания runtime-компонентов и delete/GC lifecycle.

## 2. Контекст

Новая версия модуля уже выкачена в `k8s.apiac.ru`. Первичный rollout-check
показал:

- `Module ai-models` в фазе `Ready`;
- `ai-models-controller` и `dmcr` подняты по 2 реплики без restartCount;
- актуальная API-группа `ai.deckhouse.io` установлена;
- пустые legacy CRD группы `ai-models.deckhouse.io` удалены;
- старые DMCR maintenance ack leases очищены.

Пока не проверены реальные publication paths, workload delivery, удаление
модели и восстановление после прерываний.

## 3. Постановка задачи

Провести жёсткий live e2e на `k8s.apiac.ru`: HF publication, Upload
publication, cluster-scoped Gemma 4 `ClusterModel`, rollout workload,
controlled interruptions controller/DMCR/worker, delete/GC, логи и
наблюдаемость. Зафиксировать расхождения и исправить только воспроизводимые
дефекты текущей реализации.

## 4. Scope

- Проверить live состояние `d8-ai-models` перед тестом.
- Создать временные test `Model` ресурсы для HF и Upload paths.
- Создать временный `ClusterModel` для самой маленькой доступной публичной
  Gemma 4 модели и проверить cluster-scoped publication path без
  namespace-local secret shortcuts.
- Проверить publication status, worker/job/pod logs, DMCR behavior и отсутствие
  неожиданных restart loops.
- Проверить workload delivery через тестовый workload.
- Во время публикации контролируемо прервать controller, DMCR и runtime worker
  и проверить replay/idempotency.
- Проверить delete lifecycle, cleanup request, DMCR GC и отсутствие зависших
  runtime/lease хвостов.
- Проверить RBAC-sensitive paths на уровне serviceaccount/controller и
  user-facing API.
- Исправить найденные defects узкими патчами, если причина локализована.

## 5. Non-goals

- Не менять публичный `Model` / `ClusterModel` API без отдельного API task.
- Не включать node-cache topology, если она не включена в текущем
  `ModuleConfig`.
- Не лечить unrelated cluster noise вроде `d8-upmeter` scheduling, если он не
  влияет на `ai-models`.
- Не делать destructive cluster cleanup за пределами временных e2e ресурсов.

## 6. Затрагиваемые области

- Live cluster: `k8s.apiac.ru`, namespace `d8-ai-models` и временный
  namespace `ai-models-e2e`.
- Возможные repo areas при bugfix:
  - `templates/`
  - `images/controller/internal/controllers/*`
  - `images/controller/internal/adapters/*`
  - `images/controller/internal/dataplane/*`
  - `images/dmcr/internal/*`
  - docs/test evidence по необходимости.

## 7. Критерии приёмки

- HF model path публикуется или даёт корректный recoverable failure с полными
  логами и понятным status.
- Upload model path публикуется или даёт корректный recoverable failure с
  полными логами и понятным status.
- Workload rollout получает model delivery mutation и не уходит в
  бесконечный BackOff из-за transient state.
- Controlled restart/delete controller, DMCR и worker не ломают eventual
  publication и не оставляют terminal false failure для transient ошибки.
- Delete модели создаёт понятный cleanup/GC lifecycle и завершает удаление без
  зависших finalizer/runtime secrets/jobs.
- После теста в `d8-ai-models` нет лишних restart loops, stale maintenance ack
  leases или активных test workloads.
- Если найден кодовый дефект, он исправлен узким slice с targeted проверкой.

## 8. Риски

- HF источник может быть недоступен из кластера или rate-limited.
- Upload path зависит от gateway ingress/auth и object storage.
- Прерывания live-компонентов могут временно повлиять на module availability.
- GC имеет deliberate delay, поэтому delete/GC проверка может занять больше
  одного короткого цикла.
