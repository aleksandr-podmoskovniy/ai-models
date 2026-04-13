# PLAN

## Current phase

Follow-up hardening of repo-level verify/build tooling around the phase-2 runtime baseline.

## Orchestration

- mode: `solo`
- reason:
  - bounded tooling bugfix with obvious cause;
  - no architecture/API uncertainty.

## Slice 1. Harden kubeconform bootstrap download

Цель:
- убрать flaky one-shot `curl` in `tools/kubeconform/kubeconform.sh`.

Артефакты:
- `tools/kubeconform/kubeconform.sh`
- optional `Makefile`/bundle notes if needed

Проверки:
- `make kubeconform`
- `make verify`
- `werf config render --dev --env dev controller controller-runtime dmcr`
- `werf build --dev --env dev --platform=linux/amd64 controller controller-runtime dmcr`
- `git diff --check`

## Rollback point

- revert `tools/kubeconform/kubeconform.sh` to single-download bootstrap if retry logic proves incorrect.
