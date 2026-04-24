## 1. Current phase

Phase 1 publication/runtime baseline hardening.

RBAC coverage относится к auth/platform integration и публичному API contract,
но не меняет publication byte path.

Status: implementation, repo-level validation and final read-only review
completed.

## 2. Orchestration

`full`

Причина:

- задача меняет auth/RBAC и публичную resource accessibility;
- затрагивает `Model` / `ClusterModel` API semantics;
- нужны read-only reviews до implementation.

Read-only reviews:

- `integration_architect` — Deckhouse RBAC/user-authz/rbacv2 wiring и runtime
  auth boundaries;
- `api_designer` — access matrix для `Model` / `ClusterModel`, status,
  finalizers и Secret references;
- финальный `reviewer` после implementation.

## 3. Slices

### Slice 1. Зафиксировать Deckhouse RBAC reference

Цель:

- записать найденные паттерны Deckhouse, initial conservative matrix and
  final post-hardening matrix for ai-models.

Файлы/каталоги:

- `plans/active/rbac-access-level-coverage/NOTES.ru.md`
- `plans/active/rbac-access-level-coverage/DECISIONS.ru.md`

Проверки:

- manual consistency review against Deckhouse templates

Артефакт результата:

- clear RBAC matrix and deny paths before template edits.

### Slice 2. Закрыть public-surface hardening blockers

Цель:

- сделать user-facing read grants безопасными до добавления RBAC templates.

Файлы/каталоги:

- отдельный implementation bundle по cleanup-handle storage;
- отдельный implementation bundle по `status.upload.tokenSecretRef`;
- отдельный API decision по `ClusterModel.spec.source.authSecretRef.namespace`;
- отдельный API decision по public condition reasons.

Проверки:

- `api_designer` review;
- targeted controller/API tests по выбранному hardening slice;
- `make verify`.

Артефакт результата:

- public `Model` / `ClusterModel` read больше не раскрывает backend cleanup
  handles или active upload token path сверх согласованного API contract;
- `ClusterModel` write policy защищена от cross-namespace Secret consumption.

### Slice 3. Зафиксировать access-review matrix

Цель:

- добавить проверяемые allow/deny cases до RBAC template implementation.

Файлы/каталоги:

- `plans/active/rbac-access-level-coverage/ACCESS_REVIEW.ru.md`
- при реализации — script/test fixture для rendered RBAC matrix или
  cluster-backed `kubectl auth can-i` cases.

Проверки:

- manual matrix review;
- future implementation must run rendered RBAC/access-review check.

Артефакт результата:

- cases для `User`, `PrivilegedUser`, `Editor`, `ClusterEditor`,
  `ClusterAdmin` по:
  `models`, `clustermodels`, `models/status`,
  `clustermodels/status`, `models/finalizers`,
  `clustermodels/finalizers`, Secret paths, `pods/log`,
  exec/attach/port-forward/proxy and internal runtime resources.

### Slice 4. Legacy user-authz fragments

Цель:

- добавить module-local ClusterRoles для старой accessLevel модели.

Файлы/каталоги:

- `templates/user-authz-cluster-roles.yaml`

Проверки:

- `make helm-template`
- `make kubeconform`
- rendered RBAC/access-review matrix check

Артефакт результата:

- `d8:user-authz:ai-models:user` читает `models`/`clustermodels`;
- `d8:user-authz:ai-models:editor` управляет только `models`;
- `d8:user-authz:ai-models:cluster-editor` и
  `d8:user-authz:ai-models:cluster-admin` управляют `clustermodels` после
  hardening `ClusterModel` Secret semantics.

### Slice 5. rbacv2 use permissions

Цель:

- добавить module capability permissions для прикладного использования catalog.

Файлы/каталоги:

- `templates/rbacv2/use/view.yaml`
- `templates/rbacv2/use/edit.yaml`

Проверки:

- `make helm-template`
- `make kubeconform`
- rendered RBAC/access-review matrix check

Артефакт результата:

- `d8:use:capability:module:ai-models:view` aggregate-to-kubernetes-as
  `viewer`;
- `d8:use:capability:module:ai-models:edit` aggregate-to-kubernetes-as
  `manager`;
- edit управляет только namespaced `models`.

### Slice 6. rbacv2 manage permissions

Цель:

- добавить module management permissions для операторов платформы.

Файлы/каталоги:

- `templates/rbacv2/manage/view.yaml`
- `templates/rbacv2/manage/edit.yaml`

Проверки:

- `make helm-template`
- `make kubeconform`
- rendered RBAC/access-review matrix check

Артефакт результата:

- manage view/edit содержит `deckhouse.io/moduleconfigs` с
  `resourceNames: ["ai-models"]`;
- manage edit управляет public `models` и `clustermodels`, без
  status/finalizers и internal runtime resources.

### Slice 7. API hardening follow-ups after RBAC templates

Цель:

- открыть только residual hardening follow-ups, которые не блокируют
  user-facing grants.

Файлы/каталоги:

- отдельные future task bundles

Проверки:

- manual decision record

Артефакт результата:

- отдельные задачи на residual RBAC/API cleanup, если они не blocking.

## 4. Rollback point

После Slice 1 можно остановиться без runtime/template changes.

После Slice 2 можно остановиться с безопасным public-surface hardening, но без
RBAC grants.

После каждого template slice rollback — удалить только добавленный RBAC
fragment; service-account RBAC и controller runtime не меняются.

## 5. Final validation

- `make helm-template` — passed.
- `make kubeconform` — passed.
- rendered RBAC/access-review matrix check — passed.
- `make verify` — passed.
- final `review-gate` — completed.
- final `reviewer` — completed; bundle drift findings fixed.
