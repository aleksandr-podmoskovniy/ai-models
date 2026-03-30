# Вернуть hooks ai-models к family-pattern DKP модулей

## Контекст

В `ai-models` для поддержки `CustomCertificate` был введён обходной shell-hook path после неудачной попытки доставить Go hooks через `batchhooks` в корень chart bundle. Пользователь требует выровнять модуль с соседними DKP проектами (`gpu-control-plane`, `virtualization`), где hooks собираются по каноническому паттерну `images/hooks -> go-hooks-artifact -> /hooks/go`.

## Постановка задачи

Нужно убрать shell-hook workaround и перевести `ai-models` на тот же hooks packaging pattern, что используется в соседних DKP модулях, не ломая phase-1 runtime contract и поддержку `CustomCertificate`.

## Scope

- оформить hooks runtime code под `images/hooks`;
- подключить hooks artifact в `werf`/bundle по каноническому пути `/hooks/go`;
- удалить временный shell-hook обход и related patchwork;
- выровнять docs/layout guidance под новый hooks path;
- прогнать repo checks, достаточные для подтверждения нового hooks flow.

## Non-goals

- не менять phase-1 runtime contract backend/PostgreSQL/S3/Dex;
- не проектировать новые hooks кроме уже нужного `copy_custom_certificate`;
- не менять storage/auth/openapi contract вне того, что требуется для hooks cleanup;
- не внедрять controller/API этапа 2.

## Затрагиваемые области

- `images/hooks`
- `werf.yaml`
- `.werf/stages/bundle.yaml`
- `hooks`
- `docs/development/*`
- `DEVELOPMENT.md`
- `plans/active/realign-hooks-with-family-pattern/*`

## Критерии приёмки

- hooks code живёт под `images/hooks`, а не в top-level `hooks/batch` и не в shell-hook workaround;
- `werf config render --dev --env dev` показывает `go-hooks-artifact` import в `/hooks/go`;
- в bundle больше нет `batchhooks` path;
- `CustomCertificate` support остаётся wired через Go hooks;
- `make verify` проходит.

## Риски

- возможна регрессия в werf wiring, если новый artifact не подхватится через `images/*/werf.inc.yaml`;
- можно снова получить oversized bundle path, если hooks будут импортированы не по семейному пути;
- есть риск оставить в repo смешанный layout, если не удалить shell-hook workaround полностью.
