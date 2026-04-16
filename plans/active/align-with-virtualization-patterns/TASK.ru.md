# Сверка `ai-models` с production patterns модуля `virtualization`

## Контекст

`ai-models` уже зафиксировал в durable docs, что module shell, `openapi`,
`werf`, image/runtime boundaries и часть controller ownership discipline должны
следовать DKP-style паттернам `virtualization`. При этом нужно подтвердить не
только wording в docs, но и реальное совпадение design/implementation patterns
с production-grade reference repo `../virtualization`.

Запрос пользователя требует не декларативного сравнения "на глаз", а
целенаправленной сверки:

- module root и bundle shell;
- build / `werf` / shared helper layout;
- `openapi` split;
- `templates/` и runtime ownership boundaries;
- `images/` и executable code placement;
- workflow / verify entrypoints;
- controller/runtime implementation seams там, где `ai-models` уже заявляет
  virtualization-style ownership.

Если в этих слоях найдётся drift относительно production patterns
`virtualization`, его нужно либо выровнять, либо явно зафиксировать как
осознанное отличие, если прямое копирование паттерна здесь было бы неверным.

Текущий continuation этого же workstream дополнительно требует:

- синхронизировать `images/controller/STRUCTURE.ru.md` с live tree и актуальными
  controller/runtime boundaries;
- отдельно перепроверить controller/runtime implementation patterns против
  `../virtualization`, а не ограничиваться module-shell audit;
- если обнаружится не только doc drift, но и реальный implementation drift,
  выровнять код до production-grade pattern, а не оставлять cosmetic mismatch.

## Постановка задачи

Провести architecture/implementation audit `ai-models` against
`../virtualization`, составить decision-oriented drift matrix и привести
`ai-models` к production-grade virtualization-style patterns в тех местах, где
расхождение подтверждено и реально улучшает module shell или runtime
discipline.

## Scope

- сравнить `ai-models` и `virtualization` по module-oriented surfaces:
  - root `werf` shell и `.werf/*` reuse;
  - `build/base-images`, bundle assembly и release payload discipline;
  - `Chart.yaml`, `charts/`, `.helmignore`, `templates/`, `crds/`, `openapi/`;
  - `images/*` layout и placement executable/runtime code;
  - workflow / `make` / verify entrypoints;
  - controller/runtime ownership seams, где в `ai-models` уже заявлен
    virtualization-style contract;
- зафиксировать audit result в active bundle;
- внести bounded changes в code/docs/build layout, если найден production drift;
- обновить durable docs/evidence, если изменились engineering rules или
  reference alignment.

## Non-goals

- не копировать product-specific surface `virtualization`, который привязан к
  VM/DVCR/KubeVirt domain и не нужен `ai-models`;
- не смешивать эту задачу с redesign `Model` / `ClusterModel`;
- не делать новый public API только ради внешнего сходства с `virtualization`;
- не трогать upstream/backend patching beyond what is required for shell/layout
  alignment;
- не переписывать работающие runtime paths, если audit показывает лишь naming
  difference без architectural drift.

## Затрагиваемые области

- `plans/active/align-with-virtualization-patterns/*`
- `images/controller/STRUCTURE.ru.md`
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/CONFIGURATION*.md`
- root `werf.yaml`, `.werf/*`, `build/*`, `.github/workflows/*` при наличии drift
- `templates/*`, `openapi/*`, `images/*` в тех местах, где audit подтвердит
  расхождение с declared virtualization-style patterns

## Критерии приёмки

- есть bundle с audit findings и decision-oriented drift matrix против
  `../virtualization`;
- каждый обнаруженный drift классифицирован как:
  - `align now`;
  - `intentional difference`;
  - `defer with explicit reason`;
- если выбран `align now`, реализация действительно выровнена по
  production-grade module pattern, а не просто переименована cosmetically;
- module shell остаётся DKP module root и не превращается в отдельный operator
  repo;
- не появилось нового split-brain между docs и implementation:
  claimed virtualization-style patterns подтверждаются реальным tree/build/code;
- `images/controller/STRUCTURE.ru.md` отражает фактическое package tree,
  актуальные controller/runtime seams и отдельно фиксирует, какие совпадения с
  `virtualization` являются production pattern, а какие отличия intentional;
- все затронутые runtime/controller changes остаются defendable по ownership и
  replaceable-boundary логике;
- пройдены узкие проверки по slices и repo-level `make verify`.

## Риски

- легко перепутать reference pattern с product-specific implementation detail и
  скопировать лишнее;
- audit может выявить multi-area drift, который потребует staged repair вместо
  одного механического diff;
- прямое выравнивание некоторых surfaces может конфликтовать с текущим phase-2
  shape `ai-models`, если не отделять shell discipline от domain behavior.
