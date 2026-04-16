### 1. Заголовок
Синхронизировать platform version baselines для ai-models и virtualization

### 2. Контекст
Пока шла разработка `ai-models`, version pins для `module-sdk`, `deckhouse_lib_helm`,
`golangci-lint` и связанных build/task surfaces разъехались между
`ai-models`, `virtualization` и актуальным baseline из локального `deckhouse`.
Отдельно `ai-models/build/base-images/deckhouse_images.yml` должен оставаться
прямой копией `deckhouse/candi/base_images.yml`.

### 3. Постановка задачи
Нужно привести version pins и связанные generated/lock artifacts к
согласованному baseline:
- использовать `deckhouse` как source of truth для `deckhouse_lib_helm`,
  `module-sdk` Go dependency и `base_images.yml`;
- обновить `dmt` до актуального upstream release `deckhouse/dmt`, если локальный
  pin в `ai-models` отстаёт;
- убрать устаревшие/stale pins и wording в `ai-models` и `virtualization`;
- перегенерировать связанные lock artifacts там, где это требуется.

### 4. Scope
- `ai-models`:
  - build/tooling pins (`module-sdk`, `golangci-lint`);
  - docs с явными version references;
  - sync `build/base-images/deckhouse_images.yml` от `deckhouse/candi/base_images.yml`.
- `virtualization`:
  - `deckhouse_lib_helm` version pin в chart/task/lock;
  - `module-sdk` Go dependency в hooks;
  - `golangci-lint` pins в live task/bootstrapping surfaces и явных doc references;
  - stale wording, завязанное на старые version numbers.
- regeneration:
  - Helm lock / vendored chart, если version changed;
  - Go module metadata, если dependency changed.

### 5. Non-goals
- Не обновлять произвольные third-party Go dependencies сверх названных baseline.
- Не делать blanket bump `helm` CLI, Go toolchain, werf, trivy и прочих unrelated tools.
- Не менять `deckhouse` repo как источник baseline.
- Не делать runtime/API refactor в `ai-models` или `virtualization`.

### 6. Затрагиваемые области
- `plans/active/sync-platform-version-baselines/*`
- `build/`, `tools/`, `Makefile`, `DEVELOPMENT.md` в `ai-models`
- `Chart.yaml`, `Chart.lock`, `build/base-images/*` в `ai-models`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/Taskfile.yaml`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/Chart.yaml`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/requirements.lock`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/charts/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks/*`
- live Taskfiles / bootstrap docs in `virtualization`, где явно зашиты stale linter pins

### 7. Критерии приёмки
- `ai-models` tool/build pins больше не расходятся с выбранным platform baseline.
- `ai-models/build/base-images/deckhouse_images.yml` синхронизирован с
  `deckhouse/candi/base_images.yml`.
- `virtualization` использует тот же `deckhouse_lib_helm` baseline, что и `deckhouse`.
- `virtualization/images/hooks` больше не сидит на `module-sdk v0.3.3`.
- В live docs/task surfaces не остаётся stale version references вида
  `module-sdk 0.3.3`, `deckhouse_lib_helm 1.55.1` или старых linter pins.
- Обновлённые lock/module artifacts согласованы с manifest files.
- Узкие проверки проходят.

### 8. Риски
- `module-sdk` bump в `virtualization/images/hooks` может потребовать refresh `go.sum`
  и открыть compile/test regressions.
- `deckhouse_lib_helm` bump в `virtualization` может поменять lock digest и vendored chart.
- `golangci-lint` pinning в нескольких Taskfile легко оставить несогласованным,
  если пропустить bootstrap/doc surface.
