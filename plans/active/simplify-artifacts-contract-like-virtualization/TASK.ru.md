# Упростить contract artifacts в ai-models по паттерну virtualization

## Контекст

После добавления bootstrap-friendly credentials flow в `ai-models` contract
появились user-facing поля `existingSecret`, `accessKeyKey` и `secretKeyKey`.
Для текущего phase-1 модуля это лишнее усложнение: соседний модуль
`virtualization` использует более жёсткий и понятный DKP-style contract, где
user-facing S3 credentials задаются напрямую в ModuleConfig, а сам модуль
рендерит внутренний Secret в своём namespace.

## Постановка задачи

Нужно привести `ai-models.artifacts` к более короткому и однозначному
контракту, как в `virtualization`:
- оставить user-facing `accessKey` / `secretKey`;
- убрать user-facing `existingSecret`, `accessKeyKey`, `secretKeyKey`;
- всегда рендерить внутренний Secret с фиксированными ключами;
- упростить runtime wiring, docs и render fixtures под новую модель.

## Scope

- `plans/active/simplify-artifacts-contract-like-virtualization/`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/module/artifacts-secret.yaml`
- `templates/backend/deployment.yaml`
- `docs/*`
- `fixtures/*`

## Non-goals

- не менять PostgreSQL contract;
- не менять TLS/HA wiring модуля;
- не добавлять отдельную CA-bundle механику для S3;
- не трогать phase-2 API и controller work.

## Затрагиваемые области

- user-facing values/OpenAPI для artifact storage;
- runtime Secret wiring;
- docs и fixtures;
- template render matrix.

## Критерии приёмки

- user-facing contract artifacts не содержит `existingSecret`,
  `accessKeyKey`, `secretKeyKey`;
- модуль рендерит внутренний Secret с фиксированными ключами `accessKey` и
  `secretKey`;
- validation требует полный pair `accessKey` / `secretKey`;
- deployment всегда читает internal Secret;
- docs и fixtures согласованы с новой моделью;
- `make verify` проходит.

## Риски

- изменение ломает предыдущий `existingSecret` path, поэтому docs и fixtures
  нужно синхронизировать без остаточных упоминаний;
- render matrix нужно поправить, чтобы не осталось сценариев старого
  credentials contract.
