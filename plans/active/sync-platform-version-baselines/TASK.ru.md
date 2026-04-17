### 1. Заголовок
Синхронизировать platform version baselines для ai-models по reference repos

### 2. Контекст
Пока шла разработка `ai-models`, version pins для `module-sdk`, `deckhouse_lib_helm`,
`golangci-lint` и связанных build/task surfaces разъехались между
`ai-models` и reference baselines из локальных `deckhouse` / `virtualization`.
Отдельно `ai-models/build/base-images/deckhouse_images.yml` должен оставаться
прямой копией `deckhouse/candi/base_images.yml`.

### 3. Постановка задачи
Нужно привести version pins и связанные generated/lock artifacts к
согласованному baseline:
- использовать `deckhouse` как source of truth для `deckhouse_lib_helm`,
  `module-sdk` Go dependency и `base_images.yml`;
- обновить `dmt` до актуального upstream release `deckhouse/dmt`, если локальный
  pin в `ai-models` отстаёт;
- убрать устаревшие/stale pins и wording в `ai-models`;
- не оставлять retained code changes в `virtualization`: он используется только
  как reference repo для сверки текущих version surfaces.

### 4. Scope
- `ai-models`:
  - build/tooling pins (`module-sdk`, `golangci-lint`);
  - docs с явными version references;
  - sync `build/base-images/deckhouse_images.yml` от `deckhouse/candi/base_images.yml`.
- reference-only verification against `virtualization` and `deckhouse`.

### 5. Non-goals
- Не обновлять произвольные third-party Go dependencies сверх названных baseline.
- Не делать blanket bump `helm` CLI, Go toolchain, werf, trivy и прочих unrelated tools.
- Не менять `deckhouse` repo как источник baseline.
- Не оставлять итоговый diff в `virtualization`.
- Не делать runtime/API refactor в `ai-models`.

### 6. Затрагиваемые области
- `plans/active/sync-platform-version-baselines/*`
- `build/`, `tools/`, `Makefile`, `DEVELOPMENT.md` в `ai-models`
- `Chart.yaml`, `Chart.lock`, `build/base-images/*` в `ai-models`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse/*` и
  `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/*` как
  read-only reference surfaces

### 7. Критерии приёмки
- `ai-models` tool/build pins больше не расходятся с выбранным platform baseline.
- `ai-models/build/base-images/deckhouse_images.yml` синхронизирован с
  `deckhouse/candi/base_images.yml`.
- В `ai-models` не остаётся stale version references на старые локальные pins.
- В `virtualization` не остаётся retained diff от этого workstream.
- Узкие проверки проходят.

### 8. Риски
- `deckhouse` и `virtualization` могут сами быть не идеально синхронизированы
  между собой, поэтому baseline приходится выбирать осознанно по каждой
  dependency/tool surface.
- Локальный `deckhouse` repo может отставать от актуального upstream release по
  отдельным tools, как это оказалось с `dmt`.
