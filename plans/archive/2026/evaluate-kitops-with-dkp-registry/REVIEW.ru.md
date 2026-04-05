# Review

## Findings

- `bundle-registry` для задачи не подходит: он реализует OCI/Docker Registry API поверх Deckhouse chunked bundle archives и предназначен для bundle distribution, а не для хранения/pull ModelKit artifacts.
- `payload-registry` технически выглядит подходящим target для `KitOps`: это user-facing registry на `docker distribution` с OCI/Docker API, Ingress, GC, auth proxy и DKP-native authz.
- Самая сильная сторона `payload-registry` для вашего кейса — auth/RBAC не через внешние bucket policies, а через Kubernetes tokens и `PayloadRepositoryAccess`, то есть registry pull/push можно привязать к DKP identities и namespace policy.
- Главная operational граница `payload-registry`: корневой путь registry привязан к Kubernetes namespace, а при удалении namespace содержимое репозиториев под этим prefix удаляется. Для модели как долгоживущего cluster asset это опасно, если не выделить отдельный “stable serving” namespace/ownership model.

## Missing checks

- Не подтверждён live push/pull реального ModelKit в `payload-registry`.
- Не проверен отдельный path для `KubeRay`; docs KitOps прямо покрывают `KServe`, а для `Ray` нужно отдельное operational решение.

## Residual risks

- Хотя `payload-registry` OCI-совместим, проект/UX/docs у него image-centric; возможны мелкие operational шероховатости с ModelKit media types или tooling, которые выявятся только на smoke push/pull.
- Если использовать namespace deletion semantics без отдельной governance-модели, serving artifacts могут удаляться вместе с namespace, что плохо сочетается с `ClusterModel`-подобным долгоживущим contract.
