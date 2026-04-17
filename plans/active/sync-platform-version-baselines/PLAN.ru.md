### 1. Current phase
Этап 2. Это не новый runtime/API slice, а dependency hygiene и platform baseline
sync для уже существующих module surfaces.

### 2. Orchestration
`solo`

Задача использует несколько repo как reference, но retained diff остаётся
только в `ai-models`. Решение механическое и основано на already-known
source-of-truth surfaces:
- `deckhouse` для `deckhouse_lib_helm`, `module-sdk` Go dependency и `base_images.yml`;
- `virtualization` только как comparative surface для tool/version drift.

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

#### Slice 2. Сверить reference baselines без retained diff в virtualization
- Цель:
  - проверить расхождения относительно `virtualization` и `deckhouse`;
  - не оставить итоговых code changes в `virtualization`.
- Файлы/каталоги:
  - read-only surfaces in `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
  - read-only surfaces in `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- Проверки:
  - `git -C /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization status --short`
- Артефакт:
  - reference выводы зафиксированы в bundle, а `virtualization` остаётся без
    retained diff от этого slice.

#### Slice 3. Финальная consistency sweep
- Цель:
  - убедиться, что остались только осознанные version pins;
  - зафиксировать validations и residual risks.
- Файлы/каталоги:
  - `plans/active/sync-platform-version-baselines/*`
  - touched files from slice 1
- Проверки:
  - `git diff --stat -- /Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
  - `git status --short /Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models /Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- Артефакт:
  - компактный, defendable diff без случайных unrelated изменений.

### 4. Rollback point
После slice 1. К этому моменту `ai-models` уже согласован сам по себе.

### 5. Final validation
- `bash build/base-images/sync-from-deckhouse.sh /Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`
- `make verify`
- `review-gate`
