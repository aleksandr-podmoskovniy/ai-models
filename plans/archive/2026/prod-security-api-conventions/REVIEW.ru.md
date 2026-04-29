# Security/API review

## Вывод

Текущий upload UX сознательно повторяет virtualization pattern: временный
secret URL находится в status и используется напрямую через `curl -T`.
Это допустимо только потому, что:

- URL time-bounded через `status.upload.expiresAt`;
- token scope ограничен одной upload session;
- gateway принимает token только в path, без header/query fallback;
- session Secret хранит только hash;
- raw token хранится только во внутреннем handoff Secret `Data["token"]`;
- после handoff/terminal phase gateway rejects late mutations;
- upload responses отдают `Cache-Control: no-store` и related hardening headers.

## Исправленные риски

- Убраны публичные поля `externalURL` / `inClusterURL`; status shape теперь
  ближе к virtualization: `status.upload.external` / `status.upload.inCluster`.
- Убран `create` на Secrets у upload-gateway ServiceAccount. Gateway не должен
  создавать module namespace Secrets; controller заранее обеспечивает
  storage-accounting Secret.
- Bearer/query-token compatibility не осталось в live-code.

## Остаточный риск

Любой пользователь с `get` на конкретный `Model` / `ClusterModel` в фазе
`WaitForUpload` видит временный upload URL. Это тот же UX/security tradeoff,
что и в virtualization. Для более строгой модели потребуется отдельный API/CLI
request flow, а не прямой status URL.
