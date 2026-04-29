# TASK: Universal workload delivery contract without controller-specific mutation

## Контекст

Текущий `workloaddelivery` начал поддерживать KubeRay через знание `RayService` / `RayCluster`, ownerReferences и generated pod templates. Это даёт быстрый результат для Ray, но протекает абстракциями: каждый новый сторонний controller потребует новый adapter внутри ai-models.

Новый целевой запрос: убрать controller-specific delivery support, не добавлять node labels / node selector для CSI case, оставить универсальный контракт на уровне workload/pod template annotations и явных volume driver parameters. Если пользователь не задал CSI driver parameters или на выбранной ноде нет local storage, временно допустим PVC/materialize delivery path, но он должен быть явно отделён как временный compatibility path, а не скрытый fallback.

## Scope

- `images/controller/internal/controllers/workloaddelivery/`
- `images/controller/internal/adapters/k8s/modeldelivery/`
- `templates/controller/*` webhook/controller scope, если нужно
- runtime delivery docs/tests/e2e plan updates
- RBAC only if required by PVC/materialize compatibility path

## Non-goals

- Не добавлять поддержку новых сторонних CRD/controllers по именам.
- Не внедрять ai-inference runtime planner в этот slice.
- Не менять publication / DMCR / ModelPack byte path.
- Не делать live e2e до отдельной команды пользователя.
- Не возвращать Secret projection в workload namespaces без отдельного auth design.

## Acceptance criteria

- `workloaddelivery` не содержит KubeRay-specific controller ownership path: нет watches/mappers/reconcile по `RayService`/`RayCluster` как first-class сущностям.
- CSI delivery не навешивает node selectors / labels на workload; планирование остаётся responsibility пользователя/внешнего scheduler/driver.
- CSI path опирается на явно заданный workload-facing volume/driver contract, а не на распознавание конкретного higher-level controller.
- PVC/materialize compatibility path, если включён, явно помечен как temporary and bounded, покрыт тестами и не требует cluster-wide Secret writes.
- Общий contract применим к любому controller, который создаёт Pod template или Pod с ai-models annotations.
- Документация и e2e plan не обещают KubeRay-specific mutation.

## Architecture acceptance criteria

- Controller package остаётся owner-level orchestration, без provider-specific generated object shaping.
- `modeldelivery` остаётся reusable PodTemplate/Pod mutation adapter.
- Runtime auth не протекает в workload namespaces через projected Secrets.
- Если нужен PVC compatibility path, его byte path, auth path, cleanup и limits описаны явно.
- Файлы остаются в рамках LOC < 350, thin reconciler rule сохраняется.

## RBAC coverage

Задача может менять controller/service-account RBAC. Human-facing RBAC не должен расширяться. Если PVC/materialize path требует Secret/PVC writes, они должны быть namespaced/module-owned или иметь отдельное обоснование; cluster-wide workload namespace Secret create/update/patch остаётся запрещённым.

## Orchestration

Mode: full.

Required read-only subagents before implementation:

- `repo_architect`: проверить removal KubeRay-specific boundary и package layout.
- `integration_architect`: проверить CSI/PVC/materialize/auth/storage/HA implications.
- `api_designer`: проверить annotation/driver-params contract, status/reasons, RBAC/API convention.

## Rollback point

До merge: revert this bundle and code changes to previous SharedDirect-only delivery implementation.

После merge: feature-disable PVC compatibility path and keep explicit CSI-only path if fallback proves unsafe in e2e.
