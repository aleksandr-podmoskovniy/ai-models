# TASK: Controller structure and architecture alignment

## Context

`images/controller/STRUCTURE.ru.md` должен быть durable architecture map для
controller/runtime tree. Сейчас документ частично отстал от живого дерева:
появились новые boundaries (`modelsource`, `workloaddelivery`,
`backendprefix`), часть Ray/KubeRay-specific narrative уже не соответствует
текущему diff, а некоторые runtime helpers выглядят как потенциальные
anti-hexagonal leaks.

## Task

Сверить `images/controller` с фактическим package tree, выровнять
`STRUCTURE.ru.md` и начать безопасный refactor там, где граница очевидно
зашумлена, без изменения runtime behavior.

## Scope

- Актуализировать package map и boundary descriptions в
  `images/controller/STRUCTURE.ru.md`.
- Убрать stale KubeRay/RayCluster wording из controller architecture baseline.
- Зафиксировать `internal/workloaddelivery` как shared resolved-delivery
  contract, а не controller package.
- Зафиксировать `internal/domain/modelsource` и `internal/dataplane/backendprefix`.
- Сделать один безопасный code cleanup slice, если он уменьшает coupling.

## Non-goals

- Не переписывать весь `workloaddelivery` controller в этом slice.
- Не менять public `Model` / `ClusterModel` API.
- Не менять RBAC/templates, кроме если architecture review найдёт blocker.
- Не трогать unrelated dirty files и не откатывать существующие изменения.

## Acceptance Criteria

- `STRUCTURE.ru.md` соответствует фактическому package tree.
- Документ не обещает KubeRay/Ray-specific integration как live baseline.
- Новые boundaries объяснены через ownership/runtime contract/reuse.
- Первый refactor не меняет поведение и подтверждается узкими Go tests.
- Read-only architect review captured in `PLAN.ru.md`.

## Risks

- В worktree уже есть большой несвязанный diff; нельзя смешать этот slice с
  чужими изменениями.
- Некоторые active bundles всё ещё executable; нельзя механически архивировать
  их в рамках этой задачи.
