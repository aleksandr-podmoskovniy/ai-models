# Упростить user-facing config-values и опереться на global defaults

## Контекст

Текущий `openapi/config-values.yaml` заранее описывает много локальных крутилок для `https`, `ingress`, `mlflow`, `postgresql`, `artifacts` и `auth`. Для текущего этапа это переусложняет контракт модуля: runtime ещё не реализован полностью, а user-facing schema уже выглядит как большой набор override-настроек.

Пользовательское решение на текущем шаге: убрать лишние локальные override и опереться на platform/global defaults, включая режим доступности, выбор сертификатов и другие общие настройки DKP.

## Постановка задачи

Нужно сделать user-facing contract `ai-models` минимальным и однозначным, но не пустым: оставить короткий stable слой пользовательских настроек, а runtime defaults и module wiring держать в `values`. В этот stable слой должны входить действительно необходимые настройки `PostgreSQL` и `S3-compatible artifacts`. Локальные override для HA, HTTPS, выбора сертификатов и другой общей платформенной обвязки пока не выставлять.

## Scope

- упростить `openapi/config-values.yaml` до короткого stable контракта без локальных runtime override для общей платформенной обвязки, но с нормальными настройками `PostgreSQL` и `S3-compatible artifacts`;
- добавить понятные defaults в `openapi/values.yaml` для internal/runtime wiring;
- обновить user-facing docs и README, чтобы они не обещали настройки, которых больше нет в module config, и объясняли источник defaults.

## Non-goals

- не менять runtime templates или поведение будущего deployment shape;
- не проектировать новый user-facing API вместо удалённых полей;
- не менять `docs/development/*` и phase roadmap;
- не вводить phase-2 API.

## Затрагиваемые области

- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `README.md`
- `README.ru.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

## Критерии приёмки

- `config-values` содержит короткий stable user-facing слой с `logLevel`, `PostgreSQL` и `S3-compatible artifacts`;
- `config-values` не содержит локальных runtime override для `https`, `ingress`, `ha`, cert selection и `auth`;
- `values` содержит явные defaults для internal/runtime sections;
- user-facing docs явно говорят, что модуль пока использует global/platform defaults;
- wording не создаёт ожидания, что эти настройки уже можно переопределять на уровне модуля;
- repo-level проверки проходят.

## Риски

- если схлопнуть контракт слишком резко без пояснения, станет неясно, откуда модуль берёт runtime-параметры;
- если не дать defaults в `values`, схема снова будет выглядеть недонастроенной;
- если сделать `PostgreSQL` и `S3` слишком детальными, contract снова разрастётся до набора implementation-specific override;
- если оставить старые описания в docs, repo снова будет обещать несуществующие крутилки.
