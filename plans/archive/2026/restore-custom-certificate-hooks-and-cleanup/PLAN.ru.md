# План: вернуть CustomCertificate через hooks/batch и дочистить dead hooks tree

## Current phase
Этап 1. Managed backend inside the module.

## Режим orchestration
`solo`.
Причина: задача архитектурно важная, но основной риск уже локализован и подтверждён по reference-проектам; для этого среза достаточно прямой локальной реализации без delegation.

## Slice 1. Зафиксировать канонический hooks path
Цель: вернуть правильную структуру hooks на уровне layout и build shell без
oversized hook artifact.

Файлы/каталоги:
- `hooks/*`
- `.werf/stages/bundle.yaml`
- `werf.yaml`
- `images/hooks/*` только для удаления остатков

Проверки:
- чтение reference flow в `n8n-d8` и Deckhouse `shell_lib`;
- локальная sanity-проверка путей bundle/render.

Артефакт результата:
- shell hook лежит в top-level `hooks/`, мёртвый `images/hooks` path убран, а
  oversized binary path не участвует в bundle.

## Slice 2. Вернуть рабочий CustomCertificate flow
Цель: снять временный запрет на custom certificate и подключить common hook
обратно через shell wrapper.

Файлы/каталоги:
- `hooks/*`
- `templates/_helpers.tpl`
- `openapi/values.yaml` при необходимости

Проверки:
- `make test`
- `make helm-template`

Артефакт результата:
- `CustomCertificate` снова поддерживается через DKP shell hook, validate не
  режет сценарий.

## Slice 3. Выровнять docs и hygiene
Цель: убрать ложные описания и зафиксировать новый канонический path.

Файлы/каталоги:
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `DEVELOPMENT.md`
- `plans/active/restore-custom-certificate-hooks-and-cleanup/*`

Проверки:
- `make lint`
- `make helm-template`

Артефакт результата:
- docs и repo rules соответствуют реальной структуре и runtime contract.

## Rollback point
Если common hook не удаётся корректно вернуть без нового packaging blocker, откатиться к состоянию после Slice 1: оставить cleanup и batch shell scaffold, но не снимать validate guard до подтверждённого рабочего wiring.

## Final validation
- `make lint`
- `make test`
- `make helm-template`
