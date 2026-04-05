# Implement Phase-2 Model And ClusterModel API Types

## 1. Контекст

В репозитории уже зафиксирован phase-2 design bundle для public catalog API:
`Model`, `ClusterModel`, controller-owned publish flow и separation between
public contract and internal backend. При этом `api/` в модуле пока остаётся
documentation-only и не содержит DKP-native типов для phase 2.

Следующий практический шаг — начать реализацию с самого узкого slice:
создать базовый API scaffolding и первые public types под `api/`, не таща
раньше времени controller runtime, registry access manager и publish workers.

## 2. Постановка задачи

Реализовать первый implementation slice phase 2:

- завести Go module под `api/`;
- завести group/version layout для public DKP API;
- реализовать initial `Model` и `ClusterModel` types;
- вынести общие public enum/struct types для `spec` и `status`;
- добавить generated deepcopy artifacts;
- зафиксировать минимальный generation workflow для дальнейших phase-2 slices.

## 3. Scope

- `api/` как корень public DKP API contract.
- `Model` и `ClusterModel` initial type definitions.
- Shared public types:
  - source kinds;
  - package/publish/runtime hint types;
  - access policy shape;
  - status phase;
  - upload/artifact/metadata/backend/access status blocks.
- `register.go`, `doc.go`, `group` package wiring.
- Generated `zz_generated.deepcopy.go`.
- Repo-local commands/scripts, если они нужны для воспроизводимого codegen.

## 4. Non-goals

- Не реализовывать контроллеры под `images/controller/`.
- Не делать publish worker runtime.
- Не делать reconcile logic, sync с MLflow или registry side effects.
- Не вводить webhook/server-side validation runtime.
- Не генерировать CRD manifests и не подключать их в module templates в этом
  slice.
- Не финализировать immutability enforcement beyond API comments/markers.

## 5. Затрагиваемые области

- `api/*`
- `Makefile`, только если нужен reproducible generate entrypoint
- `plans/active/implement-model-catalog-api-types/*`

Reference only:
- `plans/active/design-model-catalog-controller-and-publish-architecture/*`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/api/*`

## 6. Критерии приёмки

- В `api/` появился самостоятельный Go module для public DKP API.
- Есть versioned API package с `Model` и `ClusterModel`.
- `Model` и `ClusterModel` semantically aligned и различаются только scope /
  access semantics, а не случайным shape drift.
- `spec` и `status` разделены по ownership согласно design bundle.
- `status` не протекает raw MLflow сущностями и internal registry objects.
- Generated deepcopy files воспроизводимы локально.
- Узкая проверка для slice проходит.

## 7. Риски

- Слишком ранняя детализация types может закрепить неудачные names или shape до
  появления controller logic.
- Если сразу смешать API types и runtime/controller concerns, репозиторий начнёт
  сползать в operator-style layout, что противоречит repo rules.
- Неправильный initial version/group усложнит будущие CRD и client generation.
