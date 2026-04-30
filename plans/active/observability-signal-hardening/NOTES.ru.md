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
## 2026-04-29 follow-up closure

- Workload-delivery blocked diagnostics no longer live in PodTemplate
  annotations. The scheduling gate remains the blocking mechanism, while
  `ai.deckhouse.io/model-delivery-blocked-*` annotations now live on top-level
  workload metadata to avoid avoidable template hash churn.
- Repeated blocked logs/events are suppressed against current persisted
  workload state, including stale reconcile replay.
- DMCR upstream registry access logs now downgrade only expected manifest
  `DELETE` `404` misses from `error` to `info` with
  `reason=expected_registry_delete_miss`; read misses, blob misses, write
  failures and 5xx responses remain errors.
