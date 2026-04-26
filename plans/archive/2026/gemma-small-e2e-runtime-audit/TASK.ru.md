# Gemma small E2E runtime audit

## Контекст

Нужно проверить текущий live-path `ai-models` на `k8s.apiac.ru` через
небольшую Gemma-модель: публикация модели, доставка в workload, логи,
secrets/jobs/pods/storage, а также архитектурные риски HA-модуля.

## Постановка задачи

Запустить изолированный E2E smoke test для небольшой модели семейства Gemma,
наблюдать каждый этап controller/runtime flow и зафиксировать технические
замечания по проектированию, логам и эксплуатационной безопасности.

## Scope

- Проверить live состояние `ai-models` controller/DMCR/runtime перед тестом.
- Подобрать маленький доступный Gemma artifact без gated доступа, если это
  возможно в текущем кластере.
- Создать отдельный тестовый namespace и test `Model`/workload.
- Наблюдать status/conditions, jobs/pods, events, secrets, PVC/runtime objects,
  DMCR/controller/worker logs.
- Проверить delivery в workload и удалить тестовые ресурсы после проверки.
- Сформулировать замечания по логам, jobs/secrets/storage/HA и module design.

## Non-goals

- Не менять production workload и существующие модели.
- Не менять RBAC/API/templates в рамках этого live-audit без отдельного slice.
- Не использовать большую gated модель, если для неё нужен новый секрет/token.
- Не удалять DMCR registry data вне тестового объекта.

## Критерии приёмки

- Есть воспроизводимый live trace: resource timeline, status, logs, events.
- Понятно, прошла ли публикация модели и delivery в workload.
- Понятно, где находятся credentials/secrets/runtime pods/jobs/PVC.
- Зафиксированы concrete design findings: что нормально, что опасно, что
  надо переделывать по паттернам Deckhouse/virtualization.
- Тестовые ресурсы либо удалены, либо явно оставлены с причиной.

## Риски

- Gemma может быть gated или недоступна без HF token.
- Download может нагрузить внешнюю сеть/object store/DMCR.
- Runtime cleanup может занять время из-за GC/retention.
