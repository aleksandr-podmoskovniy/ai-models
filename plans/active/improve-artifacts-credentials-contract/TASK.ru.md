# Улучшить контракт credentials для artifacts в ai-models

> Superseded by `plans/active/simplify-artifacts-contract-like-virtualization/`.

## Контекст

Изначально phase-1 contract для `ai-models.artifacts` требовал только
`existingSecret`, который должен уже существовать в `d8-ai-models`. Для
bootstrap-сценария это оказалось неудобно: администратору приходилось заранее
создавать secret в служебном namespace модуля или включать модуль в несколько шагов.

В соседних DKP-модулях уже используются более удобные patterns:
- `virtualization` может получать object storage credentials из ModuleConfig и
  сам рендерить internal secret;
- `n8n-d8` поддерживает смешанную модель `inline value` или `existingSecret`.

## Постановка задачи

Нужно привести `ai-models` к нормальному DKP-friendly contract для S3
credentials:
- оставить поддержку `existingSecret`;
- добавить bootstrap-friendly inline `accessKey` / `secretKey`;
- если inline credentials заданы, модуль должен сам рендерить internal secret в
  своём namespace;
- runtime wiring и validation должны работать в обоих режимах.

## Scope

- `plans/active/improve-artifacts-credentials-contract/`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/module/*`
- `templates/backend/deployment.yaml`
- `docs/*`
- `fixtures/*`

## Non-goals

- не менять PostgreSQL contract;
- не менять TLS/HA wiring модуля;
- не проектировать ещё отдельную CA-bundle схему для S3 TLS;
- не менять phase-2 API и controller layout.

## Затрагиваемые области

- user-facing values/OpenAPI для artifacts;
- runtime template wiring secret references;
- validation rules;
- docs и fixtures для bootstrap flow.

## Критерии приёмки

- `ai-models.artifacts.existingSecret` продолжает поддерживаться;
- при inline `accessKey` / `secretKey` модуль сам рендерит internal secret в
  `d8-ai-models`;
- validation требует либо `existingSecret`, либо полный inline pair;
- deployment использует единый resolved secret name и не требует ручного
  bootstrap секрета в `d8-ai-models`;
- docs и fixtures отражают новый contract;
- релевантные проверки проходят.

## Риски

- неаккуратный приоритет между `existingSecret` и inline values может сделать
  поведение неочевидным;
- Secret template нельзя сделать обязательным для всех случаев, иначе можно
  поломать reuse внешнего секрета;
- изменение schema может потребовать синхронизации docs и fixtures, чтобы repo
  не начал врать про bootstrap path.
