## 1. Заголовок

Проверяемая прямая загрузка в `DMCR` без второй полной копии

## 2. Контекст

Этап 1 уже перевёл публикацию больших слоёв на прямую загрузку:

- `publish-worker` открывает внутреннюю `DMCR direct-upload v2` сессию;
- части слоя отправляются напрямую в S3-совместимое хранилище по временным
  URL;
- после завершения сессии `DMCR` пишет repository link и `.dmcr-sealed`
  sidecar, а тяжёлые байты остаются в физическом upload object.

В текущей реализации есть правильная экономия по копированию: после multipart
completion больше нет полного `CopyObject` в digest-addressed key. Но есть
неприемлемая слабая точка: `DMCR` верит `digest`, который прислал контроллер.
Это не подходит для целевой zero-trust картины, потому что владелец
публикации должен доказать соответствие digest фактическим байтам в
хранилище.

Итоговое решение для переносимого S3-compatible пути:

- толстый поток байтов всё равно не проходит через основной registry API;
- после multipart completion `DMCR` один раз читает уже собранный физический
  объект из хранилища, считает `sha256` и фактический размер;
- этот `DMCR`-вычисленный digest становится единственным источником истины для
  repository link, `.dmcr-sealed` и OCI descriptor;
- второй полной записи тех же байтов в другой object key нет;
- если хранилище в будущем сможет доказать полный `sha256` само, это можно
  заменить на проверенный быстрый путь, но текущий переносимый baseline обязан
  быть безопасным без такого допущения.

## 3. Постановка задачи

Нужно заменить доверие к клиентскому digest на `DMCR`-owned verification step,
не возвращая полный copy path и не превращая основной `DMCR` registry data path
в сетевое узкое место.

Для модели на `500 GiB` целевой путь байтов должен быть таким:

1. Во время загрузки байты лежат в одном физическом upload object:
   `_ai_models/direct-upload/objects/<session-id>/data`.
2. На `complete` объектное хранилище собирает multipart object.
3. `DMCR` читает этот же объект один раз, считает итоговый `sha256` и размер.
4. `DMCR` пишет маленький `.dmcr-sealed` sidecar и repository link под
   вычисленный digest.
5. Полной второй копии `500 GiB` внутри `DMCR` storage не появляется.

## 4. Scope

- обновить active bundle под новую целевую схему;
- изменить внутренний `DMCR direct-upload complete` contract:
  - `digest` в request становится optional expected digest;
  - response возвращает `digest` и `sizeBytes`, вычисленные `DMCR`;
  - `DMCR` не публикует объект, если expected digest не совпал с фактическим;
- изменить controller-side прямую загрузку:
  - raw/late-digest path использует digest из ответа `DMCR`;
  - described path сверяет ответ `DMCR` с заранее известным descriptor;
  - checkpoint/manifest не опираются на digest как на “клиент сказал”;
- обновить тесты `DMCR` и controller-side fake direct-upload helper;
- синхронизировать эксплуатационную документацию и README;
- не обращаться к рабочему кластеру.

## 5. Non-goals

- не проектировать `DMZ` и межкластерное распространение;
- не менять workload delivery и node-local cache;
- не добавлять public knobs в `Model.spec`;
- не реализовывать storage-native checksum fast path, пока нет подтверждённого
  переносимого контракта конкретного S3-провайдера;
- не переносить весь byte stream обратно через обычный registry `PATCH` path.

## 6. Затрагиваемые области

- `plans/active/dmcr-zero-trust-ingest/*`
- `images/dmcr/internal/directupload/*`
- `images/controller/internal/adapters/modelpack/oci/*`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`
- `images/dmcr/README.md`
- связанные тесты этих областей

## 7. Критерии приёмки

- `DMCR` больше не использует клиентский digest как источник истины.
- `DMCR` сам считает `sha256` и фактический размер собранного физического
  объекта после multipart completion.
- При mismatch между expected digest и фактическим digest публикация
  отклоняется, физический upload object удаляется.
- Успешный complete возвращает `digest` и `sizeBytes`, вычисленные `DMCR`.
- Если verification read временно падает после успешной multipart-сборки,
  физический объект не удаляется, а повторный complete может продолжить
  проверку уже собранного объекта.
- Controller-side raw path строит OCI descriptor из ответа `DMCR`.
- Controller-side described path сверяет ответ `DMCR` с заранее известным
  descriptor и не коммитит слой при расхождении.
- Published OCI contract остаётся digest-based.
- Для больших объектов нет второй полной записи тех же байтов в другой
  object key; есть один проверочный read pass.
- Документация прямо описывает, где лежат байты и где возникает единственный
  дополнительный проход по объекту.
- Пройдены узкие тесты по `DMCR` direct-upload и controller OCI direct-upload.
- Перед сдачей выполнен `make verify`.

## 8. Риски

- Один проверочный read pass увеличивает время финализации больших моделей, но
  это безопаснее, чем доверять клиентскому digest.
- Если `DMCR` direct-upload helper и основной registry process живут вместе,
  verification read может конкурировать за CPU/IO; это надо отслеживать
  метриками и при необходимости выносить в отдельный worker.
- Если документы оставить в состоянии “trusted digest from controller”, код и
  эксплуатационная картина снова разойдутся.
