# Plan: artifact storage quotas and usage visibility

## 1. Current phase

Задача относится к Phase 1/2 boundary: publication/runtime baseline уже хранит
artifacts в общем storage, но tenant quota и usage UX ещё не стали частью
платформенного контракта.

## 2. Orchestration

Режим: `solo` для текущего research/design slice.

Причина: текущий запрос не разрешает delegation явно, а код не меняется.
Перед реализацией CRD/controller/RBAC нужен `full` режим с read-only review от
`api_designer`, `integration_architect` и `repo_architect`.

## 3. Active bundle disposition

- `live-e2e-ha-validation` — keep. Это отдельный executable live-check
  workstream.
- `artifact-storage-quota-design` — keep. Это новый storage/API design
  workstream; его нельзя смешивать с live e2e.

## 4. Slices

### Slice 1. References and constraints

Цель:

- собрать релевантные паттерны Kubernetes, Deckhouse, virtualization,
  registry/object-store quotas;
- зафиксировать, что из них переносимо в `ai-models`.

Файлы:

- `plans/active/artifact-storage-quota-design/DESIGN.ru.md`

Проверки:

- локальная сверка по virtualization/deckhouse;
- внешние ссылки только на официальные docs.

Артефакт:

- секция references/constraints в design.

### Slice 2. Target quota model

Цель:

- описать owner scope, hard/used/reserved/available, cluster vs namespace
  accounting и dedupe policy.

Файлы:

- `plans/active/artifact-storage-quota-design/DESIGN.ru.md`

Проверки:

- self-review на отсутствие user-controlled quota bypass.

Артефакт:

- target model и пример CRD shape.

### Slice 3. Implementation slices

Цель:

- разложить будущую реализацию по hexagonal boundaries и проверкам.

Файлы:

- `plans/active/artifact-storage-quota-design/DESIGN.ru.md`

Проверки:

- `git diff --check`

Артефакт:

- executable next slices для будущей реализации.

## 5. Rollback point

Этот slice добавляет только план и design notes. Rollback: удалить каталог
`plans/active/artifact-storage-quota-design/`.

## 6. Final validation

- `git diff --check`
- ручная проверка, что design не противоречит текущему `Model` /
  `ClusterModel` contract и Deckhouse RBAC personas.
