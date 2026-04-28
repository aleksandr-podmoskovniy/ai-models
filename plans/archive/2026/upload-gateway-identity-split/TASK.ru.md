# Upload gateway identity split

## 1. Заголовок

Отделить upload gateway от controller identity.

## 2. Контекст

Upload gateway сейчас живёт sidecar-контейнером в `ai-models-controller`
Deployment и использует тот же ServiceAccount/ClusterRole, что controller.
Это нарушает least-privilege: публичный ingress path `/v1/upload` приводит в
Pod, где рядом работает controller с широкими cluster-wide правами.

## 3. Постановка задачи

Вынести upload gateway в отдельный Deployment/Service/ServiceAccount с
namespaced Role, а controller оставить только controller manager/webhook/metrics
identity.

## 4. Scope

- Добавить `ai-models-upload-gateway` templates.
- Перевести ingress backend на upload-gateway Service.
- Перевести controller `--upload-service-name` на upload-gateway Service.
- Убрать upload sidecar и upload port из controller Service.
- Дать upload gateway только необходимые права на session Secrets.

## 5. Non-goals

- Не менять upload HTTP API.
- Не менять `Model` / `ClusterModel` CRD.
- Не менять session Secret schema.
- Не менять object-storage credentials contract.
- Не менять user-facing RBAC aggregation.

## 6. Затрагиваемые области

- `templates/_helpers.tpl`
- `templates/controller/deployment.yaml`
- `templates/controller/service.yaml`
- новый `templates/upload-gateway/*`

## 7. Критерии приёмки

- Controller Pod больше не содержит container `upload-gateway`.
- Controller Service больше не экспонирует port `upload`.
- Upload ingress указывает на `ai-models-upload-gateway`.
- Upload gateway ServiceAccount не получает controller ClusterRole.
- Upload gateway Role ограничен namespaced Secret `get/update` в namespace
  модуля, без cluster-wide controller прав.
- `helm template` / repo helm validation проходит.

## 8. Риски

- Неверное имя Service в upload session status сломает `InClusterURL`.
- Слишком узкий RBAC может заблокировать update session Secret.
- Изменение templates требует проверки render, потому что это DKP-facing
  deployment surface.
