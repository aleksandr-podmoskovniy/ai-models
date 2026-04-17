# PLAN

## Current phase

Tooling hardening для repo-level CI вокруг уже настроенного ARC runner pool `ai-models-runners`.

## Orchestration

- mode: `solo`
- reason:
  - задача узкая и ограничена workflow/bootstrap surface;
  - cluster-side ARC layout и runtime/API boundaries не меняются;
  - причина сбоя очевидна: runtime `apt` bootstrap на минимальном runner image.

## Slice 1. Add repo-local CI wrapper

Цель:
- вынести `lint`/`test`/`verify` orchestration из `make`-bootstrap в script under `tools/ci/`.

Артефакты:
- `tools/ci/run-suite.sh`

Проверки:
- `bash -n tools/ci/run-suite.sh`
- локальный smoke run на одном subcommand (`lint` или compile-only `test`) если не мешают чужие изменения

## Slice 2. Switch workflow off apt bootstrap

Цель:
- заменить `make lint` / `make verify` в `.github/workflows/build.yaml` на repo-local script calls;
- удалить `apt-get install build-essential` из workflow.

Артефакты:
- `.github/workflows/build.yaml`

Проверки:
- `bash -n tools/ci/run-suite.sh`
- `git diff --check`
- selective smoke run одного или нескольких subcommands

## Rollback point

- вернуть `.github/workflows/build.yaml` к `make lint` / `make verify` и удалить `tools/ci/run-suite.sh`, если wrapper окажется semantic mismatch с реальным verify pipeline.
