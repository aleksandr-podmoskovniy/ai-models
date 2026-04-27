# Codebase slimming and virtualization-style boundary pass

## 1. Заголовок

Планомерно сокращать live codebase и выравнивать границы под DKP /
virtualization-style module architecture.

## 2. Контекст

Live Go code всё ещё крупный и неравномерный. Самые тяжёлые области:

- `images/controller/internal/adapters/modelpack/oci` — около 8.8k LOC;
- `images/dmcr/internal/garbagecollection` — около 6k LOC;
- `images/controller/internal/adapters/sourcefetch` — около 4.4k LOC;
- `images/controller/internal/adapters/k8s/modeldelivery` — около 4.1k LOC;
- `images/controller/internal/dataplane/publishworker` — около 3.8k LOC;
- `images/controller/internal/controllers/workloaddelivery` — около 3.5k LOC.

Предыдущие reset/refactor slices уже убрали часть legacy backend narrative, но
в live tree остаются совместимые ветки, transitional delivery paths, heavy test
matrices и повторяющиеся helper contracts. Сокращение до 25k LOC нельзя делать
одним механическим удалением: нужен последовательный проход по владельцам
runtime-поведения.

## 3. Постановка задачи

Начать серию code slimming slices, где каждый slice:

- удаляет или схлопывает доказуемо лишнюю live ветку;
- не меняет публичное поведение без отдельного API/RBAC решения;
- улучшает package ownership и separation of concerns;
- проходит узкие проверки и не ухудшает `make verify`.

## 4. Scope

- Снять метрики live Go code без архивов/cache/render artifacts.
- Через read-only subagents проверить первые кандидаты на безопасное
  сокращение.
- Реализовать первые безопасные slices с bounded write-sets.
- Зафиксировать LOC/architecture evidence и остаточные кандидаты.

## 5. Non-goals

- Не пытаться одномоментно довести код до 25k LOC ценой поломки текущего
  metadata/publication/runtime baseline.
- Не трогать staged/unstaged изменения других workstreams, кроме файлов
  текущего slice.
- Не менять `Model` / `ClusterModel` public API, RBAC или Helm templates без
  отдельного доказуемого slice.
- Не удалять compatibility path, если нет теста или явного архитектурного
  решения, что он больше не live.

## 6. Затрагиваемые области

Первичный аудит:

- `images/controller/internal/adapters/modelpack/oci`
- `images/dmcr/internal/garbagecollection`
- `images/controller/internal/adapters/sourcefetch`
- `images/controller/internal/adapters/k8s/modeldelivery`
- `images/controller/internal/dataplane/publishworker`
- `images/controller/internal/controllers/workloaddelivery`
- `images/controller/internal/nodecache`
- `api/core/v1alpha1` and `crds`, only for subagent-approved public schema
  slimming that removes already-denied surface.

Фактический write-set уточняется после read-only reviews.

## 7. Критерии приёмки

- Есть compact active bundle с выбранным первым slice.
- Read-only reviews зафиксировали top candidates и риски.
- Реализованы bounded refactor/cut slices без изменения публичного поведения
  или с явно доказанным public schema slimming.
- Нет новых файлов выше 350 LOC и нет нового монолита.
- Удалён или упрощён минимум один legacy/transitional path либо доказуемо
  схлопнут один duplicated helper surface.
- Узкие проверки по затронутым пакетам проходят.
- `git diff --check` проходит.
- Если затронуты templates/API/RBAC/runtime entrypoints, дополнительно
  проходят API/codegen checks, `make helm-template`, `make kubeconform` и RBAC
  evidence where relevant.

## 8. Риски

- Большой LOC target может провоцировать небезопасное удаление tests или
  compatibility behavior вместо настоящего упрощения.
- Transitional workload delivery и node-cache paths могут всё ещё быть live в
  кластере; их нельзя удалять без явного runtime evidence.
- DMCR GC и direct-upload paths stateful; сокращение без replay/idempotency
  тестов может сломать recovery.
