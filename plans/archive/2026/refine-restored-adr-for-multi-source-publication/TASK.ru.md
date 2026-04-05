# Refine Restored ADR For Multi-Source Publication

## 1. Контекст

В `internal-docs` восстановлен прежний ADR по каталогу моделей как baseline.

Его сильные стороны:

- каталог остаётся платформенной сущностью `Model` / `ClusterModel`;
- `ai-models` не превращается в inference API;
- внутренний backend остаётся internal component;
- public contract должен быть понятен без знания raw backend сущностей.

Но в текущем виде restored ADR слишком жёстко прибит к `artifact-first`,
`OCI-only` первой итерации и пока плохо выражает user path, который нам нужен:

- модель должна уметь приходить из разных источников;
- особенно важен HF-first path;
- источники первой полезной итерации должны соответствовать логике
  virtualization-подобного `source`: `HuggingFace`, `HTTP`, `local upload`;
- upload должен быть controller-owned по паттерну virtualization/DVCR, а не
  browser upload;
- после обработки должен появляться стабильный reference на сохранённый артефакт,
  который платформа реально хранит через свой backend/distribution path;
- дальше должен быть понятный flow: publication -> metadata enrichment ->
  `Ready`;
- потребление модели должно описываться через local materialization в PVC для
  runtime, а не через прямую работу runtime с raw backend storage;
- всё это нужно добавить без повторного ухода в source-oriented redesign, который
  ломает исходную идеологию ADR.

## 2. Постановка задачи

Аккуратно уточнить текущий ADR так, чтобы он:

- сохранил исходную идеологию каталога моделей;
- описывал multi-source ingestion по паттерну, похожему на virtualization
  images;
- описывал publication flow как controller-owned сохранение модели во
  внутреннем backend/distribution plane;
- добавлял observed reference на сохранённый published artifact;
- описывал использование модели через local PVC/materialization contract на
  стороне runtime;
- не превращал public contract в проекцию raw MLflow/UI/internal repo plumbing.

## 3. Scope

- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
- bundle under `plans/active/refine-restored-adr-for-multi-source-publication/*`

## 4. Non-goals

- Не переписывать ADR заново в source-oriented архитектуру.
- Не фиксировать все implementation details controller/runtime workers.
- Не менять код в `ai-models` в этом slice.
- Не документировать KubeRay materializer как уже закрытую фазу реализации.

## 5. Acceptance criteria

- В ADR остаётся ясная platform ideology каталога моделей.
- В ADR появляется multi-source ingestion model, похожая по духу на
  virtualization `dataSource`.
- Источники первой итерации в ADR соответствуют user flow: `HuggingFace`,
  `HTTP`, `Upload`.
- В ADR явно описан observed published artifact ref после сохранения модели.
- В ADR описан flow от source до `Ready` без лишних product semantics.
- В ADR описано потребление модели как подготовка локального пути в PVC для
  runtime.
- В ADR сохранена граница: public contract не зависит от raw backend сущностей.
- Документ не выглядит как смесь старого artifact-only и полного source-oriented
  redesign.

## 6. Risks

- Слишком слабая правка оставит ADR непрактичным для ожидаемого publication
  flow.
- Слишком сильная правка снова уведёт ADR в ту же drift-архитектуру, которую
  уже пришлось откатывать.
