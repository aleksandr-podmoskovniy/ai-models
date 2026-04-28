# Notes

## Сравнение с virtualization

- В `virtualization-artifact/pkg/monitoring/metrics/*` collectors отделяют
  iterator/scraper/reporting, а объектные labels держат только на объектных
  метриках. В ai-models сохраняем этот принцип: health metrics имеют только
  `collector`, а `name/namespace/uid` остаются только в catalog/runtime object
  metrics.
- В `virtualization-artifact/pkg/logger/attrs.go` закреплены стабильные
  structured attrs (`collector`, `controller`, `name`, `namespace`, `err`).
  В ai-models первый slice не вводит новый logging facade, но закрепляет
  `collector` как обязательный атрибут collector logs и как label health
  metrics.

## Что исправлено этим slice

- Ошибка чтения `Model`, `ClusterModel`, runtime resources, GC requests или
  storage ledger больше не выглядит как молчаливое исчезновение части рядов:
  соответствующий collector отдаёт `d8_ai_models_collector_up{collector}=0`.
- Успешный scrape обновляет
  `d8_ai_models_collector_last_success_timestamp_seconds`.
- Длительность scrape видна через
  `d8_ai_models_collector_scrape_duration_seconds`.
- Controller/runtime и DMCR structured logs нормализуют error attribute в
  `err`, не требуя ручного изменения всех `slog.Any("error", err)` call sites.

## Следующие observability slices

- Выровнять controller/dataplane log field dictionary дальше:
  `duration_ms` vs `duration_seconds`, digest/artifact/source field names.
- Проверить alert rules на новые collector health metrics после live rollout.
- Добавить e2e evidence: при временном RBAC/API отказе collector health падает,
  а metrics endpoint остаётся живым.
- Разобрать DMCR logs отдельно: garbage-collection, maintenance gate,
  direct-upload session and registry errors должны иметь одинаковые request /
  repository / phase поля без leakage секретов.
