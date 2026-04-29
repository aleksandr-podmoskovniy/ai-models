# Upload только через secret URL

## Контекст

После перевода upload UX на virtualization-style secret URL остался
совместимый gateway path через `Authorization: Bearer ...`. Старых клиентов
нет, поэтому этот path расширяет auth surface без пользы.

## Задача

Оставить единственный пользовательский upload credential: token в path
`status.upload.*URL`. Multipart actions должны идти через тот же secret URL.

## Scope

- Upload gateway auth parsing.
- Upload session tests.
- Внутренний token handoff Secret: хранить raw token под нейтральным key
  `token`, не как готовый Authorization header.
- Документация/architecture notes.

## Non-goals

- Не удалять внутренний token handoff Secret целиком: session state хранит hash,
  а controller должен уметь восстановить raw token между reconcile.
- Не добавлять query token.
- Не менять ingress/TLS.

## Acceptance

- Gateway не принимает `Authorization: Bearer ...` без path token.
- Direct upload и multipart работают через `/v1/upload/<session>/<token>` and
  `/v1/upload/<session>/<token>/<action>`.
- Query token игнорируется.
- User docs не упоминают Bearer header.
- Tests pass.
