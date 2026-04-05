# Implement Phase-2 Registry Path Conventions And Access Manager

## 1. Контекст

В `ai-models` уже появились:

- public DKP API types `Model` / `ClusterModel`;
- schema-level validation/defaulting/immutability;
- design bundle, который фиксирует:
  - canonical publish plane через `payload-registry`;
  - stable module-owned prefixes для published artifacts;
  - default same-namespace access для `Model`;
  - explicit access policy для `ClusterModel`.

Следующий implementation order по design bundle — registry namespace/path
conventions и access manager. Это первый controller-side slice phase 2: до него
`images/controller/` ещё остаётся documentation-only.

Пользователь отдельно просит смотреть на folder structure и module layout по
аналогии с `virtualization` и `gpu-control-plane`, а не придумывать произвольный
operator-style shell.

## 2. Постановка задачи

Реализовать первый controller-side slice для phase 2:

- завести module-local Go module под `images/controller/`;
- завести minimal executable/controller artifact layout;
- реализовать pure library для registry path conventions;
- реализовать initial access manager abstraction и payload-registry-oriented
  planning layer без live reconcile side effects;
- зафиксировать config/types/tests так, чтобы следующие slices могли поверх
  этого добавлять upload session lifecycle и publish workers.

## 3. Scope

- `images/controller/*`
- `plans/active/implement-model-catalog-registry-access/*`
- repo-local verify wiring, если оно нужно только для нового controller module

Внутри slice:
- module-local `go.mod`;
- `cmd/ai-models-controller`;
- bounded `internal/*` layout для:
  - `app` bootstrap shell;
  - registry path conventions;
  - access subject expansion / planning;
  - payload-registry access intent rendering;
  - minimal config / wiring / logging helpers;
- unit tests на path and access semantics.

## 4. Non-goals

- Не реализовывать ещё полный reconciler с watches и manager startup logic.
- Не внедрять upload session lifecycle.
- Не создавать live `PayloadRepositoryAccess` objects в кластере.
- Не добавлять worker runtime для HF/upload promotion.
- Не подключать templates/deploy manifests для controller image.
- Не менять public API types в `api/`, кроме крайних случаев compile integration.

## 5. Затрагиваемые области

- `images/controller/*`
- `plans/active/implement-model-catalog-registry-access/*`

Reference only:
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/images/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/gpu-control-plane/images/*`

## 6. Критерии приёмки

- `images/controller/` перестал быть documentation-only и получил module-local
  layout без operator-repo drift.
- В `images/controller/` появился ровно один `go.mod` на корне module boundary,
  а не вложенный submodule.
- Registry path conventions реализованы как testable library, а не размазаны по
  ad-hoc string concatenation.
- Есть явная модель для access planning:
  - namespaced default access для `Model`;
  - explicit access для `ClusterModel`;
  - `spec.access` у `Model` только расширяет same-namespace default, а не
    сужает его;
  - stable module-owned published prefixes;
  - staging vs catalog path split.
- Код укладывается в repo patterns, подсмотренные в `virtualization` /
  `gpu-control-plane`: module-local go.mod, thin `cmd/*`, bounded `internal/*`
  packages без преждевременного exported `pkg/*` surface.
- Узкие проверки проходят.

## 7. Риски

- Если сразу начать строить full controller manager, slice разрастётся и
  смешает несколько phase-2 задач.
- Если path/access logic не сделать отдельной library, дальше reconcile code
  быстро станет patchwork.
- Если directory layout в `images/controller/` будет выбрано неаккуратно,
  repo drift появится ещё до первой рабочей reconcile loop.
