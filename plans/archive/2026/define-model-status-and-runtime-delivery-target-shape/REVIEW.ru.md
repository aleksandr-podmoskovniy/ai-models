# REVIEW

## Scope check

Slice остался design-only:

- код `api/`, CRD и controller не менялся;
- bundle оформляет concrete target shape для следующего implementation этапа;
- public contract и internal delivery semantics разведены явно.

## Main outcomes

- Зафиксирован target public `status` с опорой на:
  - `artifact`;
  - `resolved`;
  - `upload`;
  - lifecycle `phase` и public `conditions`.
- Зафиксировано, что:
  - `status.metadata` должен уступить место единому `status.resolved`;
  - public enum `S3` надо заменить на `ObjectStorage`;
  - `Syncing`, `ArtifactStaged`, `AccessConfigured`,
    `BackendSynchronized` не должны оставаться user-facing contract.
- Зафиксирован target internal publication contract:
  - `PublishedArtifact`;
  - `ResolvedProfile`;
  - internal `CleanupHandle`.
- Зафиксирован target runtime delivery contract:
  - separate `AccessPlan`;
  - separate `VerificationPlan`;
  - разный auth path для OCI и ObjectStorage.

## Validations

- `git diff --check -- plans/active/define-model-status-and-runtime-delivery-target-shape/*`

## Residual risks

- Read-only subagent calls были запущены, но не вернули ответ до таймаута; это
  не блокирует bundle, но следующий implementation slice всё равно стоит
  провести с финальным review поверх code diff.
- Bundle сознательно не решает текущий drift в `spec` между ADR и `api/`; он
  описывает только target shape для `status` и delivery boundary.
- `ModelResolvedStatus` уже достаточно конкретен для реализации, но enum/field
  vocabulary ещё надо один раз сверить с текущим ADR перед кодовыми правками.

## Direct follow-ups

1. Rebaseline `api/core/v1alpha1/types.go` под этот target shape.
2. Regenerate `crds/*`.
3. Пересобрать `publication` и `runtimedelivery` internal contracts.
4. Только после этого идти в live reconcile/materializer implementation.
