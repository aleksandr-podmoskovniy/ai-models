### 1. Current phase
Репозиторий находится в phase-1/early phase-2 состоянии: controller-owned
publication baseline уже live, а часть runtime/distribution slices
(`nodeCache`, shared PVC bridge, runtimehealth) уже landed и должна быть
отражена в документе без смешения с ещё неlanded shared-direct narrative.

### 2. Orchestration
`solo`

Задача docs-only, но нетривиальная по объёму и требует точной сверки с live
кодом. Делегация не нужна: основной риск здесь — semantic drift между внешним
ADR и несколькими repo surfaces, а не архитектурная неопределённость.

### 3. Slices
#### Slice 1
Цель:
собрать live source of truth по public CRD, publication/runtime modes,
statuses/conditions и current workload delivery.

Файлы/каталоги:
- `api/core/v1alpha1/*`
- `crds/*`
- `docs/README.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `images/dmcr/README.md`

Проверки:
- ручная сверка field-by-field и mode-by-mode против текущего ADR.

Артефакт результата:
- зафиксированный список semantic drifts, которые надо убрать из ADR.

#### Slice 2
Цель:
переписать содержимое внешнего ADR в существующей структуре и стилистике,
удалив legacy assumptions и подставив live contract.

Файлы/каталоги:
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

Проверки:
- ручная проверка структуры документа;
- ручная проверка example YAML и всех описаний полей против `crds/*` и
  `api/core/v1alpha1/*`.

Артефакт результата:
- актуализированный ADR без structural drift и без legacy API claims.

#### Slice 3
Цель:
проверить, что обновлённый ADR не противоречит live repo-docs и не обещает
неlanded behavior.

Файлы/каталоги:
- внешний ADR;
- repo-docs из Slice 1.

Проверки:
- `rg` по обновлённому ADR на legacy field names и заведомо несуществующие
  semantics;
- точечная финальная вычитка publication/runtime sections.

Артефакт результата:
- cleaned final document, пригодный как актуальная internal reference.

### 4. Rollback point
До правки внешнего ADR безопасная точка отката — наличие только task bundle в
`plans/active/internal-docs-catalog-refresh/` без изменений в
`internal-docs/2026-03-18-ai-models-catalog.md`.

### 5. Final validation
- ручная сверка обновлённого ADR с:
  - `api/core/v1alpha1/*`
  - `crds/*`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
- поиск по ADR на legacy public fields и narrative drift.
