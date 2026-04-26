## 1. Заголовок

Upload-session без raw bearer в публичном статусе

## 2. Контекст

Изначальный slice убрал `?token=...` из `status.upload.externalURL` и
`status.upload.inClusterURL`, но raw bearer остался в
`status.upload.authorizationHeaderValue`. Это всё ещё делает upload-token
доступным каждому субъекту, который может читать `Model`/`ClusterModel`.

Сами upload-session runtime Secrets уже хранят только hash токена. Нужно
сохранить это свойство и вынести user-facing raw bearer в отдельный
owner-scoped Secret, на который статус будет ссылаться без раскрытия значения.

## 3. Постановка задачи

Нужно довести upload-session контракт до безопасной выдачи:

- `status.upload.externalURL` и `status.upload.inClusterURL` больше не содержат
  токен в query string;
- `status.upload` больше не содержит raw bearer value;
- raw bearer доступен через отдельный Secret и фиксированный key;
- upload-gateway больше не принимает `?token=` как рабочий способ
  аутентификации;
- controller runtime, status projection, cleanup, тесты и документация
  синхронизированы с новым контрактом;
- legacy recovery из token-bearing URL и другие доказанно неиспользуемые
  остатки в upload-session boundary удалены.

## 4. Scope

- обновить `ModelUploadStatus` под Secret-reference выдачу;
- переделать `k8s/uploadsession` status shaping и token recovery;
- переделать upload-session dataplane auth на Bearer-only;
- добавить/поддержать отдельный token handoff Secret;
- удалить legacy query-token recovery внутри upload-session boundary;
- обновить unit/integration-style tests для нового контракта;
- обновить docs/evidence, где описан upload-session UX.

## 5. Non-goals

- не менять сам multipart staging flow;
- не менять `DMCR` direct-upload flow;
- не добавлять новый public spec knob для выбора auth-режима;
- не трогать workload delivery и runtime topology.
- не делать broad rewrite всего controller/DMCR кода без отдельного
  defendable slice и проверки использования;

## 6. Затрагиваемые области

- `plans/active/upload-session-bearer-auth/*`
- `api/core/v1alpha1/*`
- `api/README.md`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
- `images/controller/internal/controllers/catalogcleanup/*`
- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/ports/publishop/*`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- `status.upload.externalURL` и `status.upload.inClusterURL` публикуются без
  query-токена.
- В `status.upload` нет поля с raw bearer value.
- В `status.upload` есть ссылка на Secret и key, где лежит значение
  `Authorization: Bearer ...`.
- Runtime session Secret по-прежнему не хранит raw upload token.
- Token handoff Secret удаляется вместе с upload-session runtime state.
- upload-session runtime принимает Bearer-заголовок и не принимает query-token
  как валидный путь аутентификации.
- Активная upload-session продолжает корректно проецироваться в статус и
  повторный reconcile не ротирует токен без причины.
- Узкие тесты вокруг `uploadsession`, `publishstate`, `publishobserve` и
  `catalogstatus` проходят.
- Документация не обещает URL с токеном или raw bearer в статусе.
- Audit legacy cleanup удаляет только код, который доказанно не участвует в
  текущем контракте.

## 8. Риски

- если изменить только status shaping и не изменить runtime auth, получится
  несогласованный контракт;
- если убрать token recovery без Secret handoff, повторный reconcile начнёт
  каждый раз ротировать токен;
- если забыть обновить status equality tests и projection tests, репозиторий
  останется в drift между API и runtime.
- если попытаться вычистить "всё legacy" одним diff, высокий риск удалить
  compatibility/test evidence или phase-2 подготовку без доказательства.
