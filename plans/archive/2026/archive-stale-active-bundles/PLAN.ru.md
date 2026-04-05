# PLAN

## Current phase

Поддерживающий hygiene slice поверх phase-2 workstream.

## Orchestration

- mode: `solo`
- причина: mechanical cleanup внутри `plans/`, без архитектурного выбора в
  runtime/code/API

## Slice 1. Identify Safe-To-Archive Bundles

Цель:

- перечислить historical bundles, завершённые в текущем длинном workstream;
- отделить их от неизвестных или потенциально ещё живых bundles.

Файлы/каталоги:

- `plans/active/*`
- `plans/README.md`

Проверки:

- manual audit of `plans/active/`

Вывод аудита:

- safely archived as completed historical slices:
  `analyze-model-artifact-access-patterns`,
  `cleanup-legacy-controller-and-junk`,
  `define-model-status-and-runtime-delivery-target-shape`,
  `design-kuberay-rgw-s3-consumption`,
  `design-model-catalog-controller-and-publish-architecture`,
  `evaluate-kitops-with-dkp-registry`,
  `explain-mlflow-serving-and-ceph-sts`,
  `implement-backend-specific-artifact-location-and-delete-cleanup`,
  `implement-first-backend-publication-path`,
  `implement-hf-live-publication-reconcile`,
  `implement-http-live-publication-reconcile`,
  `implement-kitops-oci-publication-path`,
  `implement-managed-backend-contract-and-runtime-delivery`,
  `implement-model-catalog-api-types`,
  `implement-model-catalog-api-validation`,
  `implement-model-catalog-registry-access`,
  `implement-model-catalog-upload-session-lifecycle`,
  `implement-model-upload-session`,
  `implement-safe-http-source-publication`,
  `rebase-publication-to-pod-session-kitops-oci`,
  `rebaseline-model-catalog-to-restored-adr`,
  `rebaseline-model-status-and-delivery-contracts`,
  `rebaseline-publication-plane-to-backend-artifact-plane`,
  `rebuild-controller-architecture-and-publication-flow`,
  `refine-restored-adr-for-multi-source-publication`,
  `restructure-controller-and-continue-model-catalog-flow`,
  `review-crd-against-internal-adr`,
  `rollout-model-catalog-controller-runtime`,
  `update-internal-adr-for-current-model-catalog-design`.
- intentionally kept in `plans/active/`:
  `add-model-cleanup-workflow`,
  `align-module-config-with-cluster`,
  `debug-live-cluster-startup`,
  `design-backend-isolation-and-storage-strategy`,
  `harden-hf-import-metadata-for-prod-ui`,
  `inspect-live-model-deletion-semantics`.

## Slice 2. Move Completed Bundles To Archive

Цель:

- перенести safe historical bundles в `plans/archive/2026/`.

Файлы/каталоги:

- `plans/active/*`
- `plans/archive/2026/*`

Проверки:

- `find plans -maxdepth 2 -type d | sort`
- `git diff --check`

## Rollback point

До фактического `mv` каталогов между `plans/active/` и `plans/archive/2026/`.
