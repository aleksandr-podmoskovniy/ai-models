# TARGET ARCHITECTURE

## Цель

Получить не "видимость SSO", а реальную совместимую схему:
- пользователь входит в `ai-models` по Deckhouse SSO;
- backend знает identity пользователя внутри самого MLflow;
- backend изолирует данные через native MLflow workspaces и permissions;
- machine paths модуля не ломаются;
- импорт больших моделей не идёт через ноутбук и не проксируется через MLflow server;
- future DKP API (`Model` / `ClusterModel`) может reuse'ить тот же runtime.

## 1. Identity boundary

Источник истины по внешней identity остаётся cluster-wide:
- `user-authn`;
- `DexProvider`;
- Deckhouse группы и их membership.

`ai-models` не должен становиться владельцем внешнего OIDC provider.

Ответственность `ai-models`:
- создать свой `DexClient`;
- использовать Dex как OIDC IdP для самого MLflow;
- принимать `groups` claim как вход для app-level authz.

## 2. Browser SSO path

Для browser users правильный путь:

`browser -> ai-models ingress -> MLflow OIDC app -> Dex -> IdP -> Dex -> MLflow`

Важно:
- основной login должен происходить в самом MLflow через OIDC, а не через ingress-only `DexAuthenticator`;
- ingress-level external-auth паттерн как в `istio` пригоден только как UI gate, но не даёт app-native identity внутри backend;
- raw service/backend не должен превращаться в "обходную дверь", где нет того же auth boundary.

## 3. Native MLflow auth + workspaces

Native MLflow auth/workspaces должны оставаться включёнными.

Роли слоёв:
- OIDC отвечает за login и user identity;
- native MLflow authz отвечает за users, service accounts, workspace membership, permissions;
- workspaces отвечают за logical isolation данных внутри shared backend.

Это означает:
- shared backend допустим только при включённой app-level authz модели;
- ingress-only SSO без native MLflow authz не считается достаточной backend isolation boundary;
- browser user и direct API access должны проходить через один и тот же app-native auth layer.

## 4. Machine actors

Нужен отдельный внутренний machine path:
- bootstrap service account / internal admin для module-owned jobs;
- machine token/basic credentials только для внутренних use cases;
- эти учётки не являются пользовательским UX и не заменяют SSO.

Использование:
- import jobs;
- smoke jobs;
- внутренние probes/automation, если им действительно нужен backend API.

Не делать:
- не использовать общий browser SSO flow для import jobs;
- не отдавать machine credentials пользователям;
- не смешивать machine bootstrap с user-facing RBAC.

## 5. Sync `namespace/group -> workspace`

Правильный target не должен строиться на эвристике "прочитать все RoleBinding в namespace и угадать workspace policy".

Нужна явная owned mapping model:
- namespace задаёт scope владения;
- группы задаются явно как access policy для ai-models;
- workspace является внутренней проекцией этой policy в MLflow.

Что это значит practically:
- namespace сам по себе не является достаточным источником truth для MLflow membership;
- sync controller должен читать DKP-owned intent, а не скрапить произвольные app/team RBAC rules;
- в phase 1 допустим internal-only provisioner contract;
- в phase 2 внешний UX должен перейти на DKP API, а не на raw MLflow entities.

Честный промежуточный вариант:
- для selected namespaces ввести internal contract "namespace participates in ai-models";
- рядом держать явный список `viewer/editor/admin` групп для этого namespace/workspace;
- sync controller создаёт/обновляет workspace, membership и default permissions через MLflow APIs.

## 6. Relation to future CRDs

Future `Model` / `ClusterModel` не должны торчать наружу как raw workspaces.

Правильная граница:
- workspace остаётся внутренней backend entity;
- DKP API описывает платформенные объекты публикации моделей;
- controller under `images/controller/` делает sync в backend и использует уже готовую identity/workspace substrate.

То есть target architecture сейчас должна подготовить:
- SSO;
- user identity inside backend;
- workspace isolation;
- machine import path;
- sync seams,

но не делать workspace публичным платформенным контрактом.

## 7. Large HF import path

Для больших HF-моделей правильный path:

`HF -> in-cluster import Job -> direct artifact client upload to S3 -> MLflow metadata`

Принципы:
- данные не должны идти через ноутбук оператора;
- для больших uploads не надо гонять artifacts через MLflow proxy (`--no-serve-artifacts`);
- импортёр внутри кластера может иметь локальный spool/cache на ephemeral volume или PVC cache, это нормально;
- "вообще без промежуточного cache" не является реалистичной или обязательной целью.

Оптимизации:
- `snapshot_download()` с конкурентной загрузкой;
- `hf_xet`/ускоренный HF backend;
- selective file patterns, если позволяет формат модели;
- отдельный import image-owned entrypoint, который потом reuse'ит controller.

## 8. Relation to KServe

KServe не заменяет registry.

Роли:
- `ai-models` хранит catalog/metadata/publish state;
- S3/OCI содержат publishable artifacts;
- KServe читает agreed artifact form как serving plane.

Для больших моделей релевантные целевые варианты:
- `s3://...` для baseline serving;
- `oci://...` / modelcar как later optimization, если это улучшает rollout и cache locality.

## 9. Что делать по этапам

### Phase 1 feasible

- включить app-native MLflow OIDC login against Dex;
- оставить native MLflow workspaces/authz;
- завести internal machine account path;
- использовать in-cluster direct-to-S3 import jobs;
- считать raw backend internal admin surface, а не platform UX.

### Phase 1.5 / hardening slice

- добавить internal provisioner/sync для `namespace + explicit groups -> workspace`;
- ограничить прямой доступ к raw service через Kubernetes RBAC и network boundaries;
- оформить наблюдаемость и recovery для sync/import jobs.

### Phase 2 target

- вынести user-facing UX в DKP API (`Model` / `ClusterModel`);
- reuse'ить уже существующую backend auth/workspace substrate;
- не требовать обычному пользователю прямой работы в raw MLflow UI.

## 10. Что не делать

- не считать ingress-only SSO достаточной изоляцией backend;
- не скрапить чужие `RoleBinding`/`AuthorizationRule` как единственный источник truth для workspace membership;
- не тащить raw MLflow workspace как публичный DKP API;
- не делать "zero-cache" import ценой собственного кастомного artifact protocol поверх upstream.
