# Перепроверить контракт секретов и storage settings

## Контекст
Сейчас `ai-models` принимает storage settings через `ModuleConfig`, включая bucket и S3 credentials. Есть риск, что текущий контракт не соответствует устоявшимся DKP/module patterns: чувствительные данные могут лежать не там, а часть storage metadata может быть user-facing без достаточного основания.

## Постановка задачи
Проверить, как в Deckhouse, `virtualization` и `n8n-d8` обычно хранят и передают секреты и storage settings, и определить, какой контракт для `ai-models` является корректным для phase 1.

## Scope
- изучить reference-паттерны хранения и передачи secrets/storage settings в смежных модулях;
- сравнить их с текущим `ai-models` contract;
- сформулировать, какие поля должны оставаться в user-facing `ModuleConfig`, какие должны уходить во внутренние `Secret`, а какие вообще не должны быть user-facing;
- при наличии ясного и узкого решения подготовить implementation plan или внести минимальную правку.

## Non-goals
- не переделывать весь storage/auth contract без ясного reference baseline;
- не тащить сюда phase-2 API или controller concerns;
- не менять runtime deployment shape, если проблема только в config ownership.

## Затрагиваемые области
- `openapi/*`
- `templates/*`
- `docs/*`
- reference-only чтение в `virtualization`, `n8n-d8`, `deckhouse`
- `plans/active/reassess-secrets-and-storage-contract/*`

## Критерии приёмки
- найден и зафиксирован reference baseline по secrets/storage contract;
- явно описано, что в `ai-models` сейчас сделано правильно, а что нет;
- если есть правка, она согласована с docs/templates/openapi и проверена релевантными make-командами.

## Риски
- разные модули могут использовать разные паттерны по уважительным причинам;
- можно механически скопировать чужой contract, который не подходит для external module phase 1.
