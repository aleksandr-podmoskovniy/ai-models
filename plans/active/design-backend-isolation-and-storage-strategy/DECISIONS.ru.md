# DECISIONS

## 1. Backend isolation через MLflow workspaces не включать half-way

- Upstream MLflow workspaces действительно дают logical separation и
  workspace-scoped permissions, но это работает в связке с MLflow auth.
- В текущем `ai-models` backend auth живёт только на ingress-уровне через Dex,
  а app-native auth / permission sync внутрь backend не включены.
- Поэтому half-way включение workspaces сейчас даст организационный surface, но
  не решит корректно backend isolation для прямого доступа к внутреннему service.
- Для phase 1 правильный baseline:
  - держать backend service внутренним;
  - ограничивать прямой доступ к service/port-forward через Kubernetes RBAC;
  - не обещать tenant isolation внутри raw backend UI.
- Если понадобится shared multi-team backend с workspace isolation, это
  отдельная implementation задача: включение MLflow workspaces + native authz +
  mapping пользователей/групп.

## 2. Large HF import path

- Для больших HF-моделей phase-1 import должен идти внутри кластера, а не через
  ноутбук оператора.
- Полностью “без локального spool” остаться в upstream MLflow log_model flow
  нельзя: local checkpoint path является нормальной частью его artifact
  serialization semantics.
- Улучшать нужно не через custom direct-to-S3 bypass любой ценой, а через:
  - in-cluster import worker / Job;
  - быстрый HF download backend (`hf_xet`);
  - selective file download, если позволяет формат модели;
  - затем в phase 2 reuse того же image-owned import entrypoint из controller Job.

## 3. Роль KServe

- KServe — это serving plane / consumer моделей, а не замена registry.
- Он умеет читать модели из S3, GCS, Azure Blob, HTTP(S), Git, PVC, HF Hub и OCI.
- Для больших моделей особенно релевантен OCI / Modelcar path, потому что он
  снижает startup cost и позволяет использовать node-local image cache.
- Правильная долгосрочная связка:
  - `ai-models` владеет registry/catalog и publish flow;
  - KServe потребляет опубликованные артефакты из agreed storage/packaging form.
