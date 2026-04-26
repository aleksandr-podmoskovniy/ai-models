# Publication/runtime resilience chaos test

## Контекст

Happy-path проверка `google/gemma-4-E2B-it` на `k8s.apiac.ru` доказала только
обычный сценарий:

- `Model` публикуется в `DMCR`;
- workload стартует через текущий `MaterializeBridge`;
- delete создаёт `dmcr-gc-*`, GC проходит после окна ожидания;
- `dmcr` и controller не рестартятся без внешних отказов.

Этого недостаточно для production-grade модуля. Нужно доказать, что
publication/runtime path восстанавливается после реальных transient отказов:
рестарты publish worker, controller, `dmcr`, read-only окно GC, потеря pod-local
materialization, delete во время незавершённой загрузки и GC при смене lease
holder.

## Цель

Сделать план жёсткого, безопасно исполнимого chaos/resilience тестирования,
который проверяет, что модуль приходит в норму без ручного ремонта state,
без потери модели, без вечных зависаний workload и без orphan объектов в
Kubernetes / DMCR / object storage.

## Scope

- Зафиксировать production-like failure matrix для publication, workload
  delivery и delete/GC.
- Сопоставить ожидаемое поведение с паттернами `virtualization` / `DVCR`.
- Определить preflight, stop conditions, rollback и evidence для каждого
  сценария.
- Отделить безопасные pod-level инъекции от более рискованных node reboot /
  drain сценариев.
- Выявить ожидаемые архитектурные доработки, если тест покажет терминальные
  ошибки вместо retry/recovery.

## Non-goals

- Не запускать разрушительные сценарии без отдельного подтверждения.
- Не ребутать реальные worker nodes на первом шаге.
- Не тестировать большие модели, пока E2B сценарии не проходят стабильно.
- Не использовать production namespace или пользовательские workload как
  объект хаос-теста.
- Не менять API/RBAC/templates в этом slice.

## Acceptance criteria

- Есть матрица отказов с точкой инъекции, способом инъекции, ожидаемым recovery,
  evidence и stop condition.
- Для каждого сценария явно указано, что считается успешным восстановлением.
- План проверяет не только `Ready`, но и логи, events, conditions, restarts,
  state Secrets, GC requests, registry/object-storage cleanup и workload
  convergence.
- План учитывает паттерны `virtualization` / `DVCR`: deterministic runtime pods,
  on-failure restart, retry/backoff, progress evidence, GC lifecycle/result,
  lease/gate/idempotency.
- План можно исполнять slice-by-slice с остановкой после первого системного
  дефекта.

## Rollback point

До выполнения chaos сценариев состояние кластера фиксируется:

- module readiness и pod restart counters;
- список тестовых `Model`, workload, state Secrets и `dmcr-gc-*`;
- состояние `dmcr` Deployment, Lease, maintenance gate и GC sidecar logs;
- наличие свободного места и health object storage / Ceph / RGW.

Rollback для каждого execution slice:

- удалить тестовые `Model` / workload;
- дождаться удаления finalizers и `dmcr-gc-*`;
- удалить только тестовые orphan Secrets/Pods по timestamped prefix;
- вернуть cordon/drain, если использовался;
- собрать evidence до очистки.

