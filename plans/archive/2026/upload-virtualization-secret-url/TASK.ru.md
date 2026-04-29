# Upload URL по паттерну virtualization

## 1. Заголовок

Сделать upload UX как в virtualization: один секретный URL из `status.upload`,
без ручной сборки `Authorization` header из отдельного Secret.

## 2. Контекст

Сейчас `Model.spec.source.upload` выдаёт `status.upload.externalURL` и
`status.upload.inClusterURL`, но сам bearer token остаётся в module-created
Secret. Пользователь вынужден читать Secret, собирать header и только потом
выполнять `curl -T`. Это расходится с UX virtualization, где status содержит
готовый secret URL для прямой загрузки.

## 3. Постановка задачи

Нужно встроить одноразовый upload token в upload URL, который controller
публикует в `status.upload.externalURL` и `status.upload.inClusterURL`.
Пользовательский путь должен быть:

```bash
UPLOAD_URL=$(kubectl -n <ns> get model <name> -o jsonpath='{.status.upload.externalURL}')
curl -fS --progress-bar -T ./model.gguf "$UPLOAD_URL" | cat
```

Старый `Authorization: Bearer ...` путь можно оставить только как
совместимость для существующих внутренних/технических клиентов.

## 4. Scope

- Controller status builder для upload session.
- Upload gateway route/auth parsing.
- Тесты controller adapter и dataplane upload API.
- User-facing docs/examples/FAQ/API notes.

## 5. Non-goals

- Не делать CLI/SDK.
- Не менять ingress/TLS схему.
- Не вводить namespace quotas или storage accounting в этом slice.
- Не удалять внутренний token Secret, если он ещё нужен для recovery/handoff;
  пользовательский UX больше не должен зависеть от чтения этого Secret.

## 6. Затрагиваемые области

- `images/controller/internal/adapters/k8s/uploadsession/`
- `images/controller/internal/dataplane/uploadsession/`
- `docs/`
- `api/README.md`

## 7. Критерии приёмки

- `status.upload.externalURL` и `status.upload.inClusterURL` содержат полный
  secret URL, пригодный для `curl -T` без дополнительных headers.
- Upload gateway принимает новый URL format и продолжает принимать старый
  bearer-header format для обратной совместимости.
- Query token не поддерживается.
- Токен остаётся ограничен session TTL через существующий `expiresAt`.
- Документация не требует пользователю читать token Secret для обычной
  загрузки.
- Тесты покрывают route parsing, secret URL auth и старую bearer-header
  совместимость.

## 8. DKP user-facing auth/RBAC coverage

- `User`: может читать `Model`/`ClusterModel` status согласно существующим
  RBAC правилам и видеть active upload URL только там, где ему разрешён доступ
  к объекту.
- `Editor` и выше: создаёт upload-backed модели как раньше.
- Sensitive deny paths не меняются: module-local доступ к Secrets не
  расширяется, direct Secret read для upload токена не становится частью UX.
- Security tradeoff intentional: как в virtualization, secret URL в status
  является upload credential. Его нельзя логировать или публиковать наружу.

## 9. Риски

- URL со встроенным токеном может попасть в shell history/logs. Это тот же
  класс риска, что у secret upload URL в virtualization, ограниченный TTL.
- Старые клиенты с multipart API должны продолжить работать через bearer
  header.
