# Убрать backend DB upgrade wrapper из ConfigMap в image-owned runtime script

## Контекст

Текущий fix для empty database startup path уже корректен по semantics, но
реализован inline Python-блоком внутри `templates/backend/configmap.yaml`. Это
работает, но оставляет executable runtime logic в module manifests вместо
`images/backend`.

## Постановка задачи

Нужно перенести DB init/upgrade wrapper в image-owned runtime script под
`images/backend`, а в ConfigMap оставить только тонкий вызов этого entrypoint.

## Scope

- runtime script для backend image;
- wiring в `images/backend/werf.inc.yaml` и локальном `Dockerfile.local`;
- упрощение `templates/backend/configmap.yaml`;
- обновление repo-local semantic validation.

## Non-goals

- не менять сам semantics init/upgrade path;
- не перепаковывать остальные runtime scripts без необходимости;
- не трогать phase-2 API и controller design.

## Затрагиваемые области

- `images/backend/*`
- `templates/backend/configmap.yaml`
- `tools/helm-tests/validate-renders.py`
- `plans/active/move-backend-db-upgrade-wrapper-into-image/*`

## Критерии приёмки

- DB init/upgrade logic больше не живёт inline в ConfigMap;
- final backend image содержит отдельный runtime wrapper для DB init/upgrade;
- рендер backend manifests использует этот wrapper;
- `make verify` проходит.

## Риски

- можно случайно развести локальный `Dockerfile.local` и `werf` runtime image;
- можно потерять текущий empty-DB safe behavior, если wrapper будет перенесён
  неэквивалентно.
