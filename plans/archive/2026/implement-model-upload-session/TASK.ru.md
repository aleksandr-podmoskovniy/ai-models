# Implement Model Upload Session

## Контекст

После corrective re-architecture phase-2 publication path больше не живёт вокруг
batch `Job`, а `HuggingFace` уже идёт через controller-owned worker `Pod`.
Следующий обязательный шаг по архитектуре и по ожиданиям продукта — `Upload`
как controller-owned session по аналогии с virtualization, а не ещё один
batch-style import.

Текущая проблема:

- public API уже содержит `spec.source.type=Upload`, `status.upload`,
  `WaitForUpload` и `UploadReady`, но live controller path этого ещё не делает;
- `Upload` сейчас фактически не работает и не даёт пользователю upload contract;
- без session/supplements boundary дальше нельзя безопасно строить ни local
  upload UX, ни последующий `KitOps`/OCI path.

## Постановка задачи

Сделать первый live bounded slice для `Upload`:

1. ввести controller-owned upload session supplements;
2. перевести `Model` / `ClusterModel` с `spec.source.type=Upload` в
   `WaitForUpload` вместо прямого controlled failure;
3. выдать пользователю working upload command/session contract через public
   `status.upload`;
4. довести upload session до текущего backend artifact plane для
   `expectedFormat=HuggingFaceDirectory`, не ломая future direction к
   `KitOps`/OCI.

## Scope

- новый task bundle для bounded `Upload session` slice;
- controller-owned upload session resources:
  - worker `Pod`;
  - `Service`;
  - short-lived auth/token Secret;
- internal upload session service boundary по аналогии с virtualization;
- publicationoperation support для `Upload`;
- modelpublish status projection для `WaitForUpload` / `UploadReady`;
- backend upload-session runtime entrypoint;
- live upload -> current object-storage-backed artifact path только для
  `expectedFormat=HuggingFaceDirectory`;
- `ModelKit` upload пока оставить controlled failure с явным сообщением до
  следующего `KitOps/OCI` slice.

## Non-goals

- Не реализовывать в этом slice `KitOps` packaging/push.
- Не делать browser upload path.
- Не делать `HTTP` safe reimplementation в этом bundle.
- Не переделывать весь public API shape вокруг upload URLs вместо текущего
  `status.upload`.
- Не закрывать runtime materializer для `ai-inference`.

## Затрагиваемые области

- `api/core/v1alpha1/*` при необходимости
- `images/controller/internal/modelpublish/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/uploadsession/*`
- `images/controller/internal/app/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/controller/*`
- `images/backend/scripts/*`
- `images/backend/Dockerfile.local`
- `images/backend/werf.inc.yaml`
- `images/controller/README.md`
- `docs/CONFIGURATION*`
- `plans/active/implement-model-upload-session/*`

## Критерии приёмки

- `Upload` больше не уходит сразу в controlled failure до создания session.
- Controller создаёт controller-owned upload session supplements с owner refs на
  publication operation.
- `Model` / `ClusterModel` с `spec.source.type=Upload` получают
  `phase=WaitForUpload`, `status.upload` и condition `UploadReady=True`, когда
  session готова.
- Пользователь получает working upload command через `status.upload.command`
  для local machine flow через `kubectl port-forward`.
- После upload для `expectedFormat=HuggingFaceDirectory` controller доводит
  объект до текущего publication result path.
- `expectedFormat=ModelKit` не притворяется working path на object-storage
  backend и честно фейлится с явным сообщением.
- Узкие controller/backend tests проходят.

## Риски

- Upload session быстро тянет за собой auth, supplements naming, expiry и
  cleanup; без bounded slice задача расползётся.
- `status.upload.command` может стать слишком implementation-specific, если не
  держать его как временный user helper поверх stable phase/conditions.
- Если попытаться в этом же slice сделать ещё и `KitOps`/OCI publication,
  получится новый fat branch без rollback point.
