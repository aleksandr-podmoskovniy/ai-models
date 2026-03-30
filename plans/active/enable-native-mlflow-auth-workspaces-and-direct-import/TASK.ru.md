# TASK

## 1. Заголовок

Включить upstream-native MLflow auth/workspaces и перевести тяжёлый import на direct-to-S3 path.

## 2. Контекст

`ai-models` уже поднял рабочий phase-1 backend, но сейчас:
- аутентификация живёт только на ingress-уровне через Dex;
- raw backend service при прямом доступе не имеет собственной authz boundary;
- workspaces upstream MLflow не включены;
- import больших HF-моделей идёт через in-cluster Job, но artifact upload по-прежнему проксируется через MLflow server.

Пользователь явно требует:
- взять из upstream native MLflow auth + workspaces, а не ограничиваться ingress SSO;
- развивать import path через in-cluster Jobs и direct-to-S3 artifact uploads.

## 3. Постановка задачи

Нужно перестроить phase-1 backend integration так, чтобы:
- backend сам использовал upstream-native MLflow auth/workspaces contract;
- direct access к raw backend больше не был безграничным по смыслу;
- тяжёлые import Jobs писали artifacts напрямую в S3, без большого server-side proxy path;
- при этом решение оставалось максимально близким к upstream MLflow и не вводило лишний platform-specific custom auth layer.

## 4. Scope

В задачу входит:
- анализ и wiring upstream-native MLflow auth/workspaces в текущем module runtime;
- изменения values/OpenAPI/templates/runtime scripts, нужные для такого wiring;
- изменения import Job/runtime path для direct-to-S3 artifact access;
- обновление fixtures, semantic checks и docs под новый phase-1 contract.

## 5. Non-goals

- Не проектировать `Model` / `ClusterModel` и phase-2 catalog API.
- Не делать отдельный собственный auth service или custom OIDC layer поверх upstream MLflow.
- Не менять backend engine source beyond controlled runtime/config integration, если этого можно избежать.
- Не пытаться в этой же задаче довести весь multi-team RBAC UX до финального platform-grade состояния.

## 6. Затрагиваемые области

- `openapi/`
- `templates/backend/`
- `templates/auth/`
- `templates/module/`
- `images/backend/`
- `tools/`
- `fixtures/`
- `docs/`
- `plans/active/enable-native-mlflow-auth-workspaces-and-direct-import/`

## 7. Критерии приёмки

- backend launcher и runtime wiring используют upstream-native MLflow auth/workspaces knobs, а не только ingress auth;
- import Job path больше не зависит от proxied artifact uploads через backend server;
- новые values/OpenAPI contracts согласованы с templates и docs;
- render/semantic checks ловят ключевые ошибки нового wiring;
- `make verify` проходит.

## 8. Риски

- Upstream MLflow auth/workspaces может потребовать дополнительного bootstrap state и runtime env, который не был нужен ранее.
- Direct artifact access может поменять assumptions для UI/API flows и artifact resolution.
- Слишком агрессивная перестройка auth может поломать текущий login path через Dex, если смешать ingress SSO и backend-native auth некорректно.
