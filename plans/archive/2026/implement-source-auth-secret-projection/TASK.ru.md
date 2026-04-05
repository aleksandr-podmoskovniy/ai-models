# Реализовать controller-owned authSecretRef projection для source publication

## Контекст

Current live publication path уже умеет публиковать:

- `HuggingFace -> KitOps/OCI`
- `HTTP archive -> KitOps/OCI`
- `Upload(HuggingFaceDirectory) -> KitOps/OCI`

Но `spec.source.huggingFace.authSecretRef` и `spec.source.http.authSecretRef`
до сих пор intentionally fail-closed. Это уже стало главным функциональным
зазором после corrective refactor: private/gated `HuggingFace` и
authenticated `HTTP` нельзя использовать через `Model` / `ClusterModel`, хотя
API contract уже содержит соответствующие поля и cluster-scoped namespace
guards.

При этом slice нельзя делать грубо:

- auth не должен утечь в public status;
- controller не должен монтировать user Secret напрямую в worker Pod;
- новый код не должен разрушать текущий hexagonal corrective direction и снова
  раздувать reconciler packages.

## Постановка задачи

Сделать controller-owned projection source auth material для publication worker
Pods:

- для `HuggingFace` — поддержать auth secret для private/gated repos;
- для `HTTP` — поддержать auth secret для authenticated downloads;
- controller должен читать Secret из source namespace, копировать только
  минимально нужные ключи в worker namespace, привязывать projection Secret к
  publication operation и удалять его вместе с operation cleanup;
- backend worker должен получать этот auth material через явный internal
  contract, без прямой зависимости на исходный user Secret.

## Scope

- `images/controller/internal/application/publication/*` только если нужен
  bounded source-plan extension
- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/publicationoperation/*` только если нужен runtime
  seam/wiring
- `images/backend/scripts/ai-models-backend-source-publish.py`
- `templates/controller/rbac.yaml` только при необходимости минимального auth
  доступа
- controller docs / active bundle

## Non-goals

- не делать runtime materializer / `kitinit` path
- не менять upload session auth flow
- не вводить new public API fields или менять semantics `Model` /
  `ClusterModel`
- не реализовывать `Upload(ModelKit)`
- не делать generic secret injection framework для всех будущих integrations
- не менять cleanup/publication architecture шире, чем нужно для auth slice

## Затрагиваемые области

- `images/controller/internal/application/publication/*`
- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/publicationoperation/*`
- `images/backend/scripts/*`
- `templates/controller/*`
- `images/controller/README.md`
- `plans/active/implement-source-auth-secret-projection/*`

## Критерии приёмки

- `HuggingFace.authSecretRef` больше не даёт controlled failure и приводит к
  worker Pod с controller-owned projected Secret
- `HTTP.authSecretRef` больше не даёт controlled failure и приводит к worker
  Pod с controller-owned projected Secret
- для namespaced `Model` пустой `authSecretRef.namespace` корректно резолвится в
  namespace объекта
- для `ClusterModel` используется explicit namespace из CRD contract
- worker Pod не читает исходный user Secret напрямую из чужого namespace
- projected Secret содержит только минимально нужные ключи для соответствующего
  source type
- projected Secret owned by publication operation и удаляется вместе с ним
- backend worker получает auth material через явный internal contract:
  `HF_TOKEN` для HuggingFace и auth directory/headers contract для HTTP
- adapter/reconciler files не разрастаются beyond current quality gates
- тесты покрывают happy path, malformed Secret payload, missing Secret, replay и
  fail-closed branch’и

## Риски

- можно случайно расширить controller RBAC больше, чем нужно
- можно начать копировать лишние secret keys и нарушить minimal disclosure
- можно смешать namespaced/cluster-scoped namespace resolution в одном месте и
  получить неочевидные auth failures
- можно протащить auth assembly обратно в reconciler вместо bounded adapter
  service
