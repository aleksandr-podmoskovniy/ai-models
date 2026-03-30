# План: перепроверить контракт секретов и storage settings

## Current phase
Этап 1. Managed backend inside the module.

## Режим orchestration
`solo`.
Причина: сначала нужен точный reference-анализ по storage/secrets ownership; без этого implementation будет гаданием. Ограничение среды также не позволяет вызывать subagents без явного запроса пользователя.

## Slice 1. Сбор reference-паттернов
Цель: прочитать `virtualization`, `n8n-d8` и релевантные Deckhouse-модули, чтобы понять типовые модели работы с секретами и storage settings.

Файлы/каталоги:
- reference repos только для чтения
- текущие `openapi/*`, `templates/*`, `docs/*`

Проверки:
- нет; это аналитический slice.

Артефакт результата:
- список reference-паттернов и отклонений текущего `ai-models`.

## Slice 2. Нормализация целевого контракта
Цель: решить, что должно быть user-facing, что должно рендериться во внутренний Secret, а что должно оставаться internal/runtime-only.

Файлы/каталоги:
- `plans/active/reassess-secrets-and-storage-contract/*`
- при необходимости `openapi/*`, `templates/*`, `docs/*`

Проверки:
- `make helm-template`
- `make lint`

Артефакт результата:
- согласованный phase-1 contract для secrets/storage.

## Slice 3. Repo-level проверки
Цель: подтвердить отсутствие regressions, если будет кодовая правка.

Файлы/каталоги:
- затронутые выше

Проверки:
- `make verify`, если реализуемо

Артефакт результата:
- рабочий diff или обоснованный вывод без diff.

## Rollback point
Если reference baseline окажется неоднозначным, остановиться на documented findings и не менять contract без отдельного решения.

## Final validation
- `make lint`
- `make helm-template`
- `make verify` при кодовой правке и работоспособном local loop
