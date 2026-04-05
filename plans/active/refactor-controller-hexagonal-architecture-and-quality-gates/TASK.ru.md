# Refactor Controller To Hexagonal Architecture And Add Quality Gates

## 1. Контекст

Текущий phase-2 controller уже умеет базовый lifecycle:

- source-first publication;
- upload session;
- `ModelPack` / OCI publication through the current implementation adapter;
- cleanup published artifact.

Но реализация drift'нула в fat orchestration:

- [reconciler.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/modelpublish/reconciler.go) — 536 LOC на старте bundle;
- [reconciler.go](/Users/myskat_90/flant/aleksandr-podmoskovniy/ai-models/images/controller/internal/publicationoperation/reconciler.go) — 409 LOC на старте bundle;
- `images/controller/internal/uploadsession/session.go` — 606 LOC на старте bundle, позже разрезан в smaller adapter files.

Сейчас в repo нет жёстких автоматических ограничителей, которые бы не дали
протащить:

- fat reconcilers;
- чрезмерный LOC на файл;
- высокую cyclomatic complexity;
- слабые orchestration-only tests без branch/state matrix.

Перед следующим feature workstream нужно сделать corrective plan, иначе новый
runtime/materializer/auth code будет наращиваться на плохом фундаменте.

## 2. Постановка задачи

Использовать один canonical bundle и выполнить corrective refactor controller
runtime slice-by-slice:

1. зафиксировать target package layout в духе hexagonal architecture;
2. добавить quality gates в `make verify`;
3. вынести use cases, ports, adapters и ownership из fat adapter code;
4. резать текущие плохие файлы bounded slices, а не big-bang переписыванием;
5. тестировать через branch/state matrix вместо file-oriented happy-path
   testing;
6. закончить corrective cuts до возвращения к следующим feature slices.
7. дополнительно вычистить compatibility shims и дублирующие controller tests,
   которые остались после предыдущих bounded cuts и больше не дают новой
   архитектурной или поведенческой проверки.
8. встроить в repo GPU-control-plane-style deadcode/coverage verification
   pattern и использовать его как объективный сигнал для дальнейшего
   controller reduction, а не только manual review.
9. aggressively reduce controller tree volume; текущий working target для
   production controller code — уйти с ~6.7k non-test Go LOC как минимум к
   ~3.4k в рамках corrective workstream, без нарушения release path.
10. перенести в `ai-models` практики из `gpu-control-plane` по:
   - стабильному сбору coverage artifacts;
   - отдельному `deadcode`-прогону в verify shell;
   - удалению реально мёртвых функций и швов по результатам этого прогона.
11. целиться в агрессивное сокращение controller tree, а не только в
   косметический cleanup; текущая reduction target для workstream — убрать
   как минимум половину очевидного patchwork/dead-wrapper/dead-test слоя
   относительно текущего phase-2 controller shape.
12. выпрямить package map remaining controller runtime так, чтобы:
   - concrete reconcilers жили под единым `internal/controllers/*`;
   - concrete K8s resource/service adapters жили под
     `internal/adapters/k8s/*`;
   - shared helper code для `Model` / `ClusterModel` и resource naming жило в
     `internal/support/*`, а не дублировалось по пакетам.

## 3. Scope

- `images/controller/internal/*`
- `images/controller/cmd/ai-models-controller/*`
- `Makefile`
- `tools/*`
- текущий planning bundle
- `artifacts/coverage/*`
- `images/controller/internal/controllers/*`
- `images/controller/internal/adapters/k8s/*`
- `images/controller/internal/support/*`

## 4. Non-goals

- Не продолжать новые feature slices поверх текущего fat controller до
  завершения corrective cuts.
- Не менять в этом bundle CRD/public API.
- Не внедрять прямо сейчас новые runtime components для materializer/auth.
- Не делать big-bang package move всего controller runtime за один шаг.
- Не удалять release-path lifecycle code без доказательства через deadcode,
  import graph и package-local tests.
- Не оставлять после rewrite parallel old-vs-new package trees c одинаковой
  ответственностью.

## 5. Затрагиваемые области

- `plans/active/refactor-controller-hexagonal-architecture-and-quality-gates/*`
- references:
  - `images/controller/internal/*`
  - `images/controller/cmd/ai-models-controller/*`
  - `Makefile`
  - `tools/*`

## 6. Критерии приёмки

- Есть отдельный planning bundle с:
  - target package layout;
  - use case map;
  - port/adapter map;
  - first-cut file split plan;
  - quality gates for `make verify`;
  - test strategy and branch matrix requirements.
- В плане явно зафиксированы hard acceptance criteria для будущих slices:
  - use case / port / adapter split;
  - max LOC per file;
  - max cyclomatic complexity per function;
  - thin reconciler rule;
  - package-level coverage thresholds;
  - lifecycle branch-matrix artifact.
- В repo добавлен automated deadcode check по controller module, wired в
  `Makefile` / `verify`, и он реально прогоняется на текущем controller tree.
- Root test shell сохраняет coverage profiles по module targets в стиле
  `gpu-control-plane`, а не только печатает агрегированный `go test`.
- Есть phased order:
  - сначала refactor `modelpublish` / `publicationoperation`;
  - затем `uploadsession` / `modelcleanup`;
  - параллельно add quality gates;
  - только потом новые features (`authSecretRef`, PVC materializer, etc.).
- После corrective cuts controller tree не должен держать:
  - file-only compatibility shims, которые можно убрать без нарушения
    public/domain/port boundaries;
  - duplicate tests, которые повторяют уже покрытый codec/mutation/service path
    и не добавляют новых negative/replay веток.
  - dead functions/methods, которые подтверждены automated deadcode run для
    current GOOS/GOARCH/test configuration и не имеют justified keep reason.
- После current package-map rewrite оставшиеся adapter packages должны иметь
  однозначный слой и ownership:
  - `controllers/*` для reconcilers;
  - `adapters/k8s/*` для concrete Pod/Service/Secret/Job builders and CRUD;
  - `support/*` только для реально shared helpers, а не для business logic.
- В репозитории есть явный shell для:
  - `deadcode` по controller packages;
  - сбору coverage artifacts в `artifacts/coverage` по bounded controller
    package groups.
- `make verify` или вложенный verify target реально запускает deadcode.
- По результатам deadcode/run cleanup удалены доказуемо мёртвые функции, файлы
  и compatibility seams.

## 7. Риски

- Самый опасный риск — описать красивую target architecture, но не привязать её
  к конкретным текущим файлам и verify hook points.
- Второй риск — сделать слишком большой “big bang refactor” вместо bounded
  slices.
- Третий риск — добавить quality gates, которые невозможно стабильно гонять в
  репозитории без понятного toolchain path.
