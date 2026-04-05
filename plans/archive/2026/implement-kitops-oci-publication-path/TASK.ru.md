# Implement KitOps OCI Publication Path

## 1. Контекст

Текущий phase-2 controller уже умеет live source-driven publication для:

- `HuggingFace`;
- narrow `HTTP` archive source;
- `Upload(HuggingFaceDirectory)`.

Но этот live path всё ещё публикует artifact в current object-storage-backed
backend plane. Это уже расходится с зафиксированным направлением:

- publication должен идти через `KitOps` packaging;
- published artifact должен быть `OCI`-адресуемым;
- `status.artifact.kind` для live path должен стать `OCI`, а не
  `ObjectStorage`;
- worker image должна реально содержать `kit` CLI, а не только ссылку в docs.

## 2. Постановка задачи

Перевести current live publication path с object-storage-backed saved artifact на
`KitOps` / OCI path, сохранив source-first UX и текущую controller архитектуру:

1. controller по-прежнему принимает `spec.source`;
2. worker pod скачивает / принимает source;
3. backend runtime пакует source через `KitOps` в `ModelPack`;
4. worker публикует artifact в configured OCI registry;
5. public `status.artifact` получает OCI ref + digest + mediaType;
6. `modelpublish` и `publicationoperation` остаются bounded owners без drift
   назад к fat reconciler / batch semantics.

## 3. Scope

- добавить pinned `kit` CLI в backend runtime image;
- завести module configuration для publication OCI registry and credentials;
- заменить controller worker destination wiring с S3 root на OCI repo prefix;
- заменить backend `source-publish` flow с object storage upload на
  `kit init/pack/push/inspect`;
- перевести current live `HuggingFace`, `HTTP`, `Upload(HuggingFaceDirectory)`
  result payloads на `artifact.kind=OCI`;
- обновить tests, docs и active bundle под новый live baseline.

## 4. Non-goals

- Не реализовывать в этом slice runtime materializer / PVC agent.
- Не реализовывать в этом slice registry cleanup execution.
- Не реализовывать `HTTP/HF authSecretRef` projection.
- Не обещать в этом slice live `Upload(ModelKit)` без чёткой ingest semantics.
- Не менять public API shape beyond current `status.artifact.kind=OCI`.

## 5. Затрагиваемые области

- `openapi/*`
- `fixtures/module-values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `templates/module/*`
- `images/controller/cmd/ai-models-controller/*`
- `images/controller/internal/artifactbackend/*`
- `images/controller/internal/sourcepublishpod/*`
- `images/controller/internal/uploadsession/*`
- `images/controller/internal/publicationoperation/*`
- `images/controller/internal/modelpublish/*`
- `images/backend/Dockerfile.local`
- `images/backend/werf.inc.yaml`
- `images/backend/scripts/*source-publish*`
- `images/backend/scripts/*upload-session*`
- `images/backend/scripts/smoke-runtime.sh`
- `docs/CONFIGURATION*`
- `images/controller/README.md`
- `plans/active/implement-kitops-oci-publication-path/*`

## 6. Критерии приёмки

- Backend runtime image устанавливает pinned `kit` CLI и runtime smoke это
  проверяет.
- Controller worker pods получают OCI publication destination и registry auth
  через module-owned config/Secret wiring.
- `ai-models-backend-source-publish` делает `KitOps` pack/push/inspect и пишет
  structured worker result с `artifact.kind=OCI`.
- Current live `HuggingFace`, `HTTP`, `Upload(HuggingFaceDirectory)` paths
  сохраняют working lifecycle через `publicationoperation` и `modelpublish`.
- Current live status/tests/docs больше не описывают object-storage-backed
  publication как baseline.
- Узкие tests и repo-level checks проходят.

## 7. Риски

- Самый опасный риск — partially migrate worker path: controller уже будет
  строить OCI refs, а backend image ещё не будет иметь working `kit`.
- Нельзя ломать phase-1 backend storage wiring: `artifacts` S3 settings остаются
  нужны внутреннему backend runtime, даже если phase-2 publication уходит в OCI.
- Если одновременно попытаться сделать `KitOps/OCI`, registry cleanup и
  runtime materialization, задача потеряет bounded scope и rollback point.
