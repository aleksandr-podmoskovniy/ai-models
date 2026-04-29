# Prod security и API conventions перед rollout

## Контекст

Перед продовым rollout надо ещё раз проверить user-facing upload/API contract,
security surface и Kubernetes/DKP naming conventions. Предыдущий slice убрал
Bearer/query-token, но оставил статусные поля `externalURL` / `inClusterURL`,
которые хуже совпадают с существующим virtualization pattern
`imageUploadURLs.external` / `inCluster`.

## Задача

Сузить upload API до одного понятного и безопасного контракта:
secret URL в status, phase-bound gateway auth, internal raw-token handoff
Secret, no-store HTTP responses и Kubernetes-style статусные field names.

## Scope

- `ModelUploadStatus` API shape.
- Upload session status projection.
- Upload gateway response hardening.
- Tests and docs that describe user upload UX.
- CRD regeneration/verification for changed public status fields.

## Non-goals

- Не менять источник истины publication artifact.
- Не добавлять новые auth modes.
- Не проектировать CLI/SDK в этом slice.
- Не менять RBAC матрицу, кроме проверки, что upload token не вынесен в
  отдельные user-readable Secret references.

## Затрагиваемые области

- `api/core/v1alpha1`
- `crds/`
- `images/controller/internal/adapters/k8s/uploadsession`
- `images/controller/internal/dataplane/uploadsession`
- upload-related tests/docs

## Критерии приёмки

- Public upload status использует `status.upload.external` и
  `status.upload.inCluster`, без `externalURL` / `inClusterURL`.
- Upload gateway принимает только `/v1/upload/<session>/<token>` и
  `/v1/upload/<session>/<token>/<action>`.
- `Authorization`, query token и token Secret reference не являются
  user-facing API.
- Upload responses include no-store / no-sniff headers.
- Token остается hash-only в session Secret и raw only во внутреннем handoff
  Secret `Data["token"]`.
- CRD отражает новый status shape.
- Tests and docs pass.

## Риски

- Это public status rename before prod; старые live тестовые объекты после
  rollout могут временно иметь старые поля до reconcile.
- Docs/e2e manifests должны перейти на новый jsonpath.
