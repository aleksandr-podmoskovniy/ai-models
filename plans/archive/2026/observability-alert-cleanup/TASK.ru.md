# Observability alert cleanup

## 1. Заголовок

Убрать ложные `ai-models` алерты в `k8s.apiac.ru` и зафиксировать шаблонный
фикс, чтобы они не возвращались после следующего render.

## 2. Контекст

В live-кластере firing:

- `D8AIModelsBackendPodIsNotRunning`;
- `D8AIModelsBackendTargetAbsent`;
- `D8AIModelsControllerTargetAbsent`;
- `D8AIModelsDMCRTargetAbsent`.

`backend` — legacy alert от удалённого backend-first runtime. Controller/DMCR
targets отсутствуют из-за неправильного ServiceMonitor wiring: Prometheus
выбирает только ServiceMonitor с `prometheus=main`, а `dmcr` ServiceMonitor
смотрит на labels, которых нет на Service.

## 3. Scope

- Удалить legacy backend PrometheusRule source из repo.
- Исправить controller/DMCR ServiceMonitor и Service labels.
- Обновить render validation, чтобы этот класс ошибок не вернулся.
- В live-кластере удалить уже созданные alert objects после фикса источника
  или явно зафиксировать, что live managed resources нельзя patch'ить напрямую.

## 4. Non-goals

- Не отключать весь мониторинг `ai-models`.
- Не удалять полезные controller/DMCR readiness/running/down alerts.
- Не менять Prometheus/Alertmanager global configuration.

## 5. Acceptance Criteria

- В render нет `D8AIModelsBackend*`.
- Controller и DMCR ServiceMonitor имеют `prometheus=main`.
- ServiceMonitor selector совпадает с labels целевого Service.
- `make helm-template`, `make kubeconform`, `git diff --check` проходят.
- В `k8s.apiac.ru` после deploy/current cleanup нет active `ai-models`
  ClusterObservabilityAlert noise.

## 6. RBAC/Exposure

RBAC и user-facing API не меняются. Изменение только в observability wiring.
