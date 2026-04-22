## 1. Заголовок

Zero-trust ingest в `DMCR` без полной переписи blob-ов

## 2. Контекст

Текущий publication baseline уже умеет:

- читать тяжёлые source bytes один раз на стороне `publish-worker`;
- открывать late-digest `direct-upload v2` сессию в `DMCR`;
- заливать части напрямую в backing storage без registry `PATCH` data path.

Но текущая реализация ingest во внутренний registry всё ещё не дотягивает до
production-ready целевой картины для больших моделей:

- `DMCR` на `complete(...)` принимает итоговый `digest` от клиента и использует
  его для выбора канонического blob key;
- backing storage после завершения multipart-сессии делает полный
  `CopyObject(...)` из временного session object в канонический
  digest-addressed key;
- возобновление опирается на живой `sessionToken` и `listParts()`, но не
  описано как durable resume после падения `publish-worker` или controller;
- operator-facing status/progress по long-running ingest path недостаточен для
  defendable эксплуатации.

Это уже не просто follow-up по runtime delivery. Это отдельный corrective
workstream над самим internal registry ingest contract. Пока он не закрыт,
дальнейшая работа по `DMZ`, node-local cache и runtime topology будет
наслаиваться на непрочный publication backend.

Предшественники:

- `plans/archive/2026/publication-storage-hardening/*`
- `plans/active/phase2-model-distribution-architecture/*`

## 3. Постановка задачи

Нужно довести ingest во внутренний `DMCR` до целевого production-ready
контракта, в котором одновременно выполняются четыре условия:

1. `DMCR` не доверяет клиентскому `digest` как источнику истины для
   канонического blob identity.
2. Путь публикации не требует полной повторной записи тех же `500 GiB` в новый
   object key после завершения загрузки.
3. Возобновление загрузки после прерывания является явной durable частью
   контракта, а не побочным эффектом живой multipart-сессии.
4. Весь workstream ограничен только publication ingest в internal registry:
   без смешивания с workload delivery, `DMZ` и node-local cache.

Текущий continuation внутри этого же workstream:

- поднять bounded publication progress из direct-upload checkpoint не только в
  condition reason/message, но и в top-level `Model.status.progress`;
- убрать зависимость controller/status path от парсинга text-only progress
  message;
- сохранить при этом machine-readable stage reasons (`started`, `uploading`,
  `resumed`, `sealing`, `committed`) и honest percentage semantics.

## 4. Scope

- зафиксировать единственный целевой ingest contract для `DMCR`;
- определить trusted boundary для вычисления и подтверждения итогового digest;
- убрать зависимость от схемы "client says final digest -> DMCR believes it";
- убрать зависимость от схемы "temporary object -> full copy -> canonical blob
  key";
- ввести durable session model для старта, продолжения, завершения и abort;
- определить внутреннюю раскладку registry storage/index так, чтобы published
  contract снаружи оставался digest-based OCI contract;
- определить machine-readable progress/state surface для долгих upload-сессий;
- довести `catalogstatus` / `publishobserve` до отдельного
  machine-readable progress field для sourceworker runtime;
- синхронизировать docs с новым ingest contract и его ограничениями.

## 5. Non-goals

- не менять в этом bundle runtime delivery path в workload;
- не проектировать здесь `DMZ` registry tier;
- не проектировать node-local cache service или mount topology;
- не менять public `Model.spec` ради upload/runtime knobs;
- не возвращать digest-first helper или другие legacy fallback paths как
  допустимый long-term baseline.

## 6. Затрагиваемые области

- `plans/active/dmcr-zero-trust-ingest/*`
- `images/dmcr/internal/directupload/*`
- `images/dmcr/internal/*`, если понадобится storage/index split
- `images/controller/internal/adapters/modelpack/oci/*`
- `images/controller/internal/controllers/catalogstatus/*`, если ingest
  progress/state выводится в `status`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/ports/publishop/*`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`
- `images/dmcr/README.md`
- `images/controller/README.md`
- при необходимости `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- Зафиксирован единый target ingest contract, в котором:
  - итоговый `digest` подтверждается trusted `DMCR`-owned boundary, а не
    принимается на веру от клиента;
  - sealing не требует полной переписи уже загруженного тяжёлого объекта в
    новый канонический object key;
  - multipart/session state можно возобновить после падения worker/process,
    а не только в рамках одного живого процесса;
  - published contract для consumers остаётся immutable OCI artifact by digest.
- Явно описано, где физически живут байты во время загрузки:
  - в незавершённой session/object storage state;
  - после sealing в reusable sealed blob storage unit;
  - без второй полной физической копии тех же байтов.
- Явно описано, где живёт durable session metadata:
  - session id;
  - physical object identity;
  - uploaded parts / byte progress;
  - state machine;
  - expiry / cleanup semantics.
- Явно зафиксировано resume behavior:
  - что происходит при обрыве сети;
  - что происходит при падении `publish-worker`;
  - что происходит при падении `DMCR`;
  - когда возможен resume, а когда требуется abort/restart.
- Явно зафиксировано zero-trust security behavior:
  - что именно проверяет `DMCR`;
  - чему `DMCR` не доверяет;
  - какой атакующий сценарий больше не проходит;
  - какие временные signed URL / session token semantics допустимы.
- В bundle нет повторного размывания scope на runtime delivery и distribution
  topology.
- Есть bounded implementation slices с отдельными проверками для:
  - contract/state machine;
  - storage/index refactor;
  - controller-side session resume;
  - status/progress/docs.
- Для sourceworker-driven publication path:
  - top-level `status.progress` публикуется из explicit runtime progress field,
    а не извлекается из human-readable condition message;
  - progress не показывает ложные `100%`, пока весь planned publish не
    завершён;
  - progress очищается на terminal `Ready` / `Failed` status и не становится
    второй историей поверх conditions.

## 8. Риски

- попытка сохранить текущую digest-addressed физическую раскладку blob-ов
  почти неизбежно вернёт полный `CopyObject(...)` после завершения upload;
- попытка добиться zero-trust без trusted digest verification оставит
  архитектурную дыру, даже если подписывать part URL и session token;
- попытка "добавить resume быстро" без durable session metadata приведёт только
  к ещё одному best-effort helper поверх текущего in-memory flow;
- если смешать этот workstream с workload delivery, задача снова разрастётся и
  не даст defendable production baseline к дедлайну.
