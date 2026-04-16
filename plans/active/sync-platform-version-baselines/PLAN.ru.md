### 1. Current phase
Этап 2. Это не новый runtime/API slice, а dependency hygiene и platform baseline
sync для уже существующих module surfaces.

### 2. Orchestration
`solo`

Задача multi-repo, но решение здесь механическое и основано на already-known
source-of-truth surfaces:
- `deckhouse` для `deckhouse_lib_helm`, `module-sdk` Go dependency и `base_images.yml`;
- existing live repo surfaces для stale pins/doc wording.

Новый архитектурный выбор не проектируется, поэтому отдельные read-only reviews
до реализации не нужны.

### 3. Slices

#### Slice 1. Зафиксировать baseline и синхронизировать ai-models
- Цель:
  - привести `ai-models` build/tooling pins к выбранному baseline;
  - подтянуть `dmt` до актуального upstream release, если локальный pin устарел;
  - синхронизировать `build/base-images/deckhouse_images.yml` от `deckhouse`.
- Файлы/каталоги:
  - `build/components/versions.yml`
  - `Makefile`
  - `.gitlab-ci.yml`
  - `tools/install-module-sdk.sh`
  - `tools/module-sdk-wrapper.sh`
  - `tools/install-golangci-lint.sh`
  - `DEVELOPMENT.md`
  - `build/base-images/deckhouse_images.yml`
- Проверки:
  - `bash build/base-images/sync-from-deckhouse.sh /Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
  - `rg -n "0\\.10\\.3|2\\.6\\.2" -S Makefile .gitlab-ci.yml build/components/versions.yml tools DEVELOPMENT.md`
- Артефакт:
  - ai-models больше не держит stale local pins против platform baseline.

#### Slice 2. Синхронизировать virtualization tool/runtime pins
- Цель:
  - обновить `deckhouse_lib_helm`, `module-sdk` hooks dependency и live linter pins;
  - убрать stale version wording.
- Файлы/каталоги:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/Taskfile.yaml`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/Chart.yaml`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/requirements.lock`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/charts/*`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks/go.mod`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks/go.sum`
  - live Taskfiles with explicit `golangci-lint` bootstrap/version logic
  - stale docs/comments with explicit old versions
- Проверки:
  - `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization && helm dependency update`
  - `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks && go test ./...`
  - `rg -n "1\\.55\\.1|0\\.3\\.3|1\\.64\\.8|1\\.52\\.1" -S /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- Артефакт:
  - virtualization lock/task/hook surfaces согласованы с новым baseline.

#### Slice 3. Финальная consistency sweep
- Цель:
  - убедиться, что остались только осознанные version pins;
  - зафиксировать validations и residual risks.
- Файлы/каталоги:
  - `plans/active/sync-platform-version-baselines/*`
  - touched files from slices 1-2
- Проверки:
  - `git diff --stat -- /Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
  - `git status --short /Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- Артефакт:
  - компактный, defendable diff без случайных unrelated изменений.

### 4. Rollback point
После slice 1. К этому моменту `ai-models` уже согласован сам по себе, а
cross-repo changes в `virtualization` ещё не начаты.

### 5. Final validation
- `bash build/base-images/sync-from-deckhouse.sh /Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
- `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization && helm dependency update`
- `cd /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/hooks && go test ./...`
- `review-gate`
