# DECISIONS

## 1. Ingress-only SSO как у `istio` не считать достаточной схемой

- Референс `istio`/Kiali в Deckhouse использует стандартный UI gate:
  `Ingress external-auth + DexAuthenticator + allowedGroups`.
- Это правильно для web UI, где само приложение может жить в header-based auth
  или вообще не владеть собственной user identity.
- Но для `ai-models` этого недостаточно, потому что нам нужна:
  - user identity внутри самого backend;
  - app-native authz;
  - workspace membership;
  - защита от прямого доступа к внутреннему service.
- Поэтому pattern `istio` полезен только как reference на cluster-level SSO gate,
  но не как целевая модель backend isolation.

## 2. Целевая схема должна быть: Dex SSO -> native MLflow authz/workspaces

- Upstream MLflow workspaces действительно дают logical separation и
  workspace-scoped permissions, но это работает в связке с MLflow auth.
- Поэтому правильная целевая схема:
  - login через Dex в сам MLflow;
  - native MLflow authz/workspaces включены;
  - machine actors отделены от browser users;
  - namespace/group intent sync'ится в workspace membership отдельным provisioner.
- Ingress-only SSO без native MLflow authz не считается достаточной backend
  isolation boundary.

## 3. Namespace/group -> workspace sync делать только от явного DKP-owned intent

- Не надо пытаться выводить workspace membership из случайных `RoleBinding`,
  `AuthorizationRule` или другого namespace RBAC "по месту".
- Namespace даёт scope и ownership boundary, но не даёт сам по себе полной и
  безопасной модели access policy для MLflow.
- Правильный sync source:
  - namespace participation в `ai-models`;
  - явный список групп для viewer/editor/admin-like access;
  - затем internal projection этого intent в MLflow workspace.
- В phase 2 внешний UX должен жить в DKP API, а workspace остаётся внутренней
  backend сущностью.

## 4. Large HF import path

- Для больших HF-моделей phase-1 import должен идти внутри кластера, а не через
  ноутбук оператора.
- Полностью “без локального spool” остаться в upstream MLflow artifact flow
  нельзя: локальный download/cache на import worker является нормальной частью
  serialization/upload semantics.
- Улучшать нужно не через custom direct-to-S3 bypass любой ценой, а через:
  - in-cluster import worker / Job;
  - быстрый HF download backend (`hf_xet`);
  - selective file download, если позволяет формат модели;
  - direct artifact client upload в S3 вместо proxy через MLflow server;
  - затем в phase 2 reuse того же image-owned import entrypoint из controller Job.

## 5. Роль KServe

- KServe — это serving plane / consumer моделей, а не замена registry.
- Он умеет читать модели из S3, GCS, Azure Blob, HTTP(S), Git, PVC, HF Hub и OCI.
- Для больших моделей особенно релевантен OCI / Modelcar path, потому что он
  снижает startup cost и позволяет использовать node-local image cache.
- Правильная долгосрочная связка:
  - `ai-models` владеет registry/catalog и publish flow;
  - KServe потребляет опубликованные артефакты из agreed storage/packaging form.

## 6. Граница "через MLflow API" vs "локальный bootstrap helper"

- Не все backend-side операции должны идти через MLflow API.
- Правильное разделение такое:
  - pre-start/bootstrap операции, от которых зависит сам запуск сервера,
    не могут опираться на HTTP API этого же сервера;
  - steady-state provisioning и sync после старта сервера должны идти через
    поддерживаемый MLflow Python/REST API.
- Для `ai-models` это означает:
  - `db-upgrade` остаётся локальной pre-start операцией;
  - рендер auth config / runtime env остаётся локальным bootstrap layer;
  - future provisioner для users / workspace permissions / namespace-group sync
    должен использовать MLflow API, а не прямые записи в auth DB.
- Следствие для cleanup:
  - отдельными runtime operations оставлять только реальные операции
    (`db-upgrade`, `hf-import`, `bootstrap-oidc-auth` на переходный период);
  - мелкие `render-*` helpers консолидировать в один runtime helper / package,
    а не плодить дальше одноразовые entrypoints.
