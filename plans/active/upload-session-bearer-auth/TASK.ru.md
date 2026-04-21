## 1. Заголовок

Upload-session без токена в URL

## 2. Контекст

Текущий `status.upload` публикует `externalURL` и `inClusterURL` уже с
`?token=...` внутри. Это упрощает клиентскую сторону, но делает токен частью
URL и создаёт лишний риск утечки через ingress/proxy/access-логи, shell-history
и copy/paste.

Сам upload-gateway уже умеет принимать `Authorization: Bearer ...`, поэтому
нужно убрать токен из URL-поверхности и сделать Bearer-заголовок единственным
нормальным контрактом для user-facing upload-session path.

## 3. Постановка задачи

Нужно перевести upload-session контракт на безопасную выдачу:

- `status.upload.externalURL` и `status.upload.inClusterURL` больше не содержат
  токен в query string;
- `status.upload` явно отдаёт отдельное поле для Bearer-аутентификации;
- upload-gateway больше не принимает `?token=` как рабочий способ
  аутентификации;
- controller runtime, status projection, тесты и документация синхронизированы
  с новым контрактом.

## 4. Scope

- обновить `ModelUploadStatus` под Bearer-only выдачу;
- переделать `k8s/uploadsession` status shaping и token recovery;
- переделать upload-session dataplane auth на Bearer-only;
- обновить unit/integration-style tests для нового контракта;
- обновить docs/evidence, где описан upload-session UX.

## 5. Non-goals

- не менять сам multipart staging flow;
- не проектировать отдельный Secret-based token handoff;
- не менять `DMCR` direct-upload flow;
- не добавлять новый public spec knob для выбора auth-режима;
- не трогать workload delivery и runtime topology.

## 6. Затрагиваемые области

- `plans/active/upload-session-bearer-auth/*`
- `api/core/v1alpha1/*`
- `api/README.md`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
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
- В `status.upload` есть отдельное явное поле для значения
  `Authorization: Bearer ...`.
- upload-session runtime принимает Bearer-заголовок и не принимает query-token
  как валидный путь аутентификации.
- Активная upload-session продолжает корректно проецироваться в статус и
  повторный reconcile не теряет токен.
- Узкие тесты вокруг `uploadsession`, `publishstate`, `publishobserve` и
  `catalogstatus` проходят.
- Документация не обещает URL с токеном.

## 8. Риски

- если изменить только status shaping и не изменить runtime auth, получится
  несогласованный контракт;
- если убрать token recovery без замены, повторный reconcile начнёт каждый раз
  ротировать токен;
- если забыть обновить status equality tests и projection tests, репозиторий
  останется в drift между API и runtime.
