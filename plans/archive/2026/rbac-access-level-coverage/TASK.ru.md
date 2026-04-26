## 1. Заголовок

DKP RBAC coverage для `Model` / `ClusterModel` и module manage/use ролей

## 2. Контекст

В `ai-models` сейчас есть только service-account RBAC для controller, DMCR и
node-cache runtime:

- `templates/controller/rbac.yaml`;
- `templates/dmcr/rbac.yaml`;
- `templates/node-cache-runtime/rbac.yaml`.

Пользовательские Deckhouse роли (`User`, `PrivilegedUser`, `Editor`, `Admin`,
`ClusterEditor`, `ClusterAdmin`, `SuperAdmin`) не получают явных module-local
permissions на публичные `ai.deckhouse.io` resources. Значит модуль работает
только там, где пользователь уже имеет сверхширокий доступ.

Reference из Deckhouse:

- source repo: `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse`;
- source revision: `f13d67cf51ecd6127a7bdd03c25e1ff7060c2da2`;
- legacy `user-authz` module fragments:
  `modules/*/templates/user-authz-cluster-roles.yaml`;
- `rbacv2/use/*` для прикладного использования module capabilities;
- `rbacv2/manage/*` для управления module/subsystem permissions;
- global role aggregation через labels `rbac.deckhouse.io/*`.

Read-only review выявил два API/security blockers, которые нельзя скрыть RBAC
шаблонами:

- `ai.deckhouse.io/cleanup-handle` хранится на публичном объекте и содержит
  backend/upload-staging details;
- `ClusterModel.spec.source.authSecretRef.namespace` допускает cross-namespace
  Secret consumption, поэтому write для `ClusterEditor` пока небезопасен.

## 3. Постановка задачи

Спроектировать DKP-compatible RBAC coverage для публичных resources модуля и
реализовывать grants только после hardening публичных утечек:

- дать read доступ к catalog objects не только `SuperAdmin`;
- дать namespaced application editing для `Model`;
- дать conservative cluster-wide editing для `ClusterModel` только там, где это
  безопасно;
- не открывать пользователям controller-owned/internal objects;
- не расширять доступ к Secret, exec, logs или port-forward локальными grants.

## 4. Scope

- `templates/user-authz-cluster-roles.yaml` или equivalent legacy user-authz
  surface;
- `templates/rbacv2/use/view.yaml`;
- `templates/rbacv2/use/edit.yaml`;
- `templates/rbacv2/manage/view.yaml`;
- `templates/rbacv2/manage/edit.yaml`;
- API hardening prerequisites:
  - убрать backend cleanup handle из public object metadata;
  - пересмотреть public `status.upload.tokenSecretRef`;
  - сузить или явно запретить `ClusterModel` cross-namespace auth Secret use;
  - стабилизировать public condition reasons без runtime-stage leakage;
- при необходимости docs/development notes про RBAC contract;
- Helm render/kubeconform/access-review evidence.

## 5. Non-goals

- не менять service-account RBAC controller/DMCR/node-cache как часть
  пользовательских ролей;
- не давать user-facing roles доступ к:
  - DMCR auth/TLS/CA Secrets;
  - upload-session state Secrets;
  - source-worker auth clones;
  - projected runtime pull Secrets;
  - cleanup jobs;
  - internal pods/deployments/services/ingress модуля;
- не добавлять module-local grants на `pods/log`, `pods/exec`,
  `pods/attach`, `pods/portforward`, `pods/proxy`;
- не давать end-user roles verbs на `models/status`,
  `clustermodels/status`, `models/finalizers`,
  `clustermodels/finalizers`;
- не ship-ить user-facing read/write grants до hardening публичных утечек:
  cleanup-handle, upload token ref и `ClusterModel` Secret semantics.

## 6. Затрагиваемые области

- DKP templates;
- public `Model` / `ClusterModel` access semantics;
- user-authz / rbacv2 integration;
- docs/development task evidence.

## 7. Критерии приёмки

- legacy user-authz fragments существуют и соответствуют Deckhouse naming:
  `d8:user-authz:ai-models:*`;
- rbacv2 `use` view/edit fragments существуют и aggregate-ятся в
  `kubernetes` use roles;
- rbacv2 `manage` view/edit fragments существуют и aggregate-ятся в выбранный
  subsystem как module manage permission;
- `User` / `PrivilegedUser` получают только read на `models` и
  `clustermodels`;
- `Editor` / `Admin` получают CRUD на namespaced `models`, но не на status /
  finalizers;
- `ClusterModel` write либо остаётся `ClusterAdmin+`, либо отдельным решением
  разрешается для `ClusterEditor` только после API hardening;
- `SuperAdmin` покрывается global platform role, без module-local wildcard;
- controller/DMCR/node-cache SA roles не aggregate-ятся для людей;
- Helm render и kubeconform проходят;
- access-review matrix покрывает allowed и denied paths для relevant
  access levels/personas;
- task review явно фиксирует deny paths: Secrets, exec, port-forward, internal
  runtime resources, status/finalizers.

## 8. Риски

- можно случайно выдать людям controller service-account privileges через
  aggregate labels;
- можно открыть `ClusterModel` write слишком рано и разрешить cross-namespace
  Secret consumption;
- можно оставить backend cleanup state видимым всем reader-ролям;
- можно смешать legacy user-authz и rbacv2 semantics без понятной матрицы.
