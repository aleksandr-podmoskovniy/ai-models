# PLAN

## 1. Current phase

Задача проектирует phase-2 public catalog API поверх уже существующего phase-1
managed backend. Реализация CRD/контроллера не входит в этот slice, но дизайн
должен быть достаточно конкретным, чтобы стать основой для implementation
tasks.

Orchestration mode: `solo`.

Причина:
- задача substantial и multi-area по смыслу;
- но текущая tooling policy не допускает delegation без прямого запроса
  пользователя, поэтому design bundle оформляется локально и с финальным
  `review-gate`.

## 2. Slices

### Slice 1. Нормализовать границы решения

Цель:
- зафиксировать, что является canonical publish/storage path;
- отделить public catalog contract от internal backend и serving planes.

Файлы:
- `TASK.ru.md`
- `TARGET_ARCHITECTURE.ru.md`

Проверки:
- согласованность с `docs/development/TZ.ru.md`
- согласованность с `docs/development/PHASES.ru.md`

Артефакт:
- архитектурное решение с явной границей: `Model/ClusterModel` как public API,
  `payload-registry + ModelPack` как publish plane, internal backend как
  secondary metadata/provenance integration.

### Slice 2. Спроектировать API contract

Цель:
- описать `spec`, `status`, conditions, immutability и ownership model для
  `Model` и `ClusterModel`.

Файлы:
- `API_CONTRACT.ru.md`

Проверки:
- contract не выводит raw backend internals;
- `Model` и `ClusterModel` семантически aligned;
- source виды и status conditions покрывают user journeys.

Артефакт:
- implementable contract для будущих типов под `api/`.

### Slice 3. Спроектировать publish flow и user scenarios

Цель:
- описать source flows, upload path, registry RBAC, consumption и cleanup.

Файлы:
- `USER_FLOWS.ru.md`
- `TARGET_ARCHITECTURE.ru.md`

Проверки:
- flow для HF, local upload, KServe, KubeRay и delete покрыт;
- access model опирается на реальные свойства `payload-registry`.

Артефакт:
- полный набор user journeys и controller-owned действий.

### Slice 4. Финальный review bundle

Цель:
- проверить, что решение не утащило в design лишние implementation details и
  не ушло в phase-3 premature hardening.

Файлы:
- `REVIEW.ru.md`

Проверки:
- `git diff --check`
- review against `docs/development/REVIEW_CHECKLIST.ru.md`

Артефакт:
- краткий review с residual risks.

## 3. Rollback point

Безопасная точка остановки: после создания design bundle в `plans/active/...`
без изменений production code, values или templates. В худшем случае bundle
можно удалить целиком без влияния на runtime.

## 4. Final validation

- `git diff --check`
- согласованность design bundle с:
  - `docs/development/TZ.ru.md`
  - `docs/development/PHASES.ru.md`
  - `docs/development/REVIEW_CHECKLIST.ru.md`
  - уже существующими bundle'ами:
    - `design-backend-isolation-and-storage-strategy`
    - `evaluate-kitops-with-dkp-registry`
    - `design-kuberay-rgw-s3-consumption`
    - `explain-mlflow-serving-and-ceph-sts`
