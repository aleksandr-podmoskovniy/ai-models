# Упрощение публичного ModuleConfig

## Контекст

`ai-models` уже имеет controller-owned publication/runtime baseline, но текущий
`ModuleConfig` утёк внутренними runtime-деталями: `dmcr.gc.schedule`,
`artifacts.sourceFetchMode`, имена `LocalStorageClass` /
`LVMVolumeGroupSet` / VG / thin pool и два размера node-cache. Это создаёт
операторский шум и закрепляет implementation details как пользовательский
контракт.

В DKP-стиле и по аналогии с virtualization наружу должны выходить только
стабильные решения администратора кластера, а не internal wiring.

## Задача

Сократить user-facing `ModuleConfig`:

- убрать публичный `dmcr`;
- убрать публичный выбор `sourceFetchMode`;
- оставить для `nodeCache` только включение, один размер и понятный
  label-contract для выбора нод и дисков;
- оставить DMCR GC, source-fetch policy и storage object names внутренними
  defaults;
- обновить документацию и render checks.

## Scope

- `openapi/config-values.yaml`;
- `openapi/values.yaml`;
- `templates/_helpers.tpl`;
- user-facing configuration docs;
- тестовые/архитектурные заметки только там, где они прямо ссылаются на
  удаляемые публичные knobs.

## Non-goals

- не менять `Model` / `ClusterModel` CRD;
- не менять RBAC;
- не менять фактический publication byte-path;
- не удалять внутренний DMCR GC runtime;
- не реализовывать новый cache placement алгоритм.

## Критерии приёмки

- `dmcr` отсутствует из user-facing config schema;
- `artifacts.sourceFetchMode` отсутствует из user-facing config schema;
- `nodeCache` содержит только стабильные настройки администратора;
- templates продолжают передавать runtime полный internal config за счёт
  module-owned defaults;
- node-cache по умолчанию использует один label contract
  `ai.deckhouse.io/model-cache=true` для нод и `BlockDevice`;
- source fetch остаётся module-owned policy, default `Direct`;
- render/kube validation проходит.

## Риски

- существующий кластер с ручными `nodeCache.*` overrides потребует
  пересборки `ModuleConfig` под новый компактный контракт;
- слишком агрессивное удаление internal defaults может сломать DMCR GC или
  node-cache runtime templates.
