# Implement HTTP Live Publication Reconcile

## 1. Контекст

После corrective slice controller уже работает по bounded architecture:

- `modelpublish` владеет только public lifecycle/status;
- `publicationoperation` владеет durable execution boundary;
- `HuggingFace -> mlflow/object storage` уже имеет live path;
- public API уже допускает `spec.source.type=HTTP`.

Следующий практический шаг — добавить второй live source без возврата к fat
reconciler architecture.

## 2. Постановка задачи

Реализовать live publication path для `Model` / `ClusterModel` c
`spec.source.type=HTTP` на уже существующей operation-based architecture:

- controller остаётся thin lifecycle owner;
- `publicationoperation` создаёт и отслеживает controller-owned worker Job;
- backend worker скачивает артефакт по HTTP(S), публикует его в managed backend
  `mlflow`, вычисляет базовую metadata/result payload и пишет durable result в
  operation ConfigMap;
- public status заполняется тем же путём, что и для `HuggingFace`.

## 3. Scope

- `images/backend/scripts/ai-models-backend-hf-import.py` или новый соседний
  runtime helper, если reuse окажется чище;
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelimportjob/*`
- affected tests under `images/controller/internal/*`
- bundle under `plans/active/implement-http-live-publication-reconcile/*`

## 4. Non-goals

- Не реализовывать сейчас полноценный live `Upload` flow.
- Не подделывать virtualization-style upload path без uploader/auth contract.
- Не реализовывать OCI packaging/push в payload-registry.
- Не делать runtime materializer/agent для PVC/local path в этом slice.
- Не расширять пока public API beyond what уже нужно для `HTTP` source.

## 5. Затрагиваемые области

- `images/backend/scripts/`
- `images/controller/internal/`
- `plans/active/implement-http-live-publication-reconcile/*`

## 6. Критерии приёмки

- `publicationoperation` поддерживает `spec.source.type=HTTP` как live path, а
  не immediate failure.
- Worker скачивает remote artifact по HTTP(S), публикует его в `mlflow` и
  возвращает structured durable result.
- `modelpublish` для `Model` / `ClusterModel` получает успешный result и пишет
  `status.source`, `status.artifact`, `status.resolved`, `phase`, `conditions`
  без special casing.
- HTTP auth/CA path поддержан в минимально корректном виде, который не
  протаскивает secret contents в public status.
- Узкие tests на controller packages и repo-level checks проходят.

## 7. Риски

- Легко зацементировать HF-specific naming/args/result shape и тем самым снова
  испортить controller layering.
- Нельзя quietly игнорировать `authSecretRef` и `caBundle`, если они уже есть в
  public API.
- Нельзя делать фальшивый `Upload` path по аналогии с HTTP: это другая
  архитектурная задача с controller-owned uploader/auth lifecycle.
