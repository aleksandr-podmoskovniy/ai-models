# Reconcile Chat Decisions And Cleanup Phase-2 Runtime

## Контекст

В репозитории уже накопилось несколько параллельных phase-2 workstreams:

- corrective hexagonal refactor controller;
- runtime materialization baseline;
- end-to-end `ModelPack` lifecycle design;
- отдельные docs/skill updates.

При этом в чате были дополнительно проговорены durable решения, которые ещё не
везде зафиксированы одинаково:

1. published/runtime contract всегда идёт через `OCI from registry`;
2. backend storage под `DVCR` скрыт и не влияет на runtime/public contract;
3. published artifact contract — `ModelPack`, а не vendor-specific `KitOps`;
4. concrete implementations (`KitOps`, `Modctl`, future module-owned impl) —
   это adapters;
5. runtime path всегда идёт через init-container / materializer и отдаёт
   рантайму только локальный путь;
6. phase-1 `mlflow` остаётся рядом и не трогается;
7. проект нужно дочистить от старых plans/docs/files, которые уже не совпадают
   с текущим курсом.

Сейчас есть риск снова получить patchwork: часть файлов и bundles уже aligned,
часть ещё звучит старой моделью или дублирует активные workstreams.

## Постановка задачи

Сделать один corrective workstream, который:

1. проверит текущий репозиторий против реальных договорённостей из чата;
2. выровняет runtime/materialization docs, планы и repo memory под эти
   договорённости;
3. жёстко почистит stale active bundles, legacy wording и ненужные seams;
4. продолжит hard refactor структуры phase-2 runtime/controller без возврата к
   монолиту;
5. подготовит repo к следующему concrete implementation slice без старого
   мусора и contradictory guidance.

## Scope

- `plans/active/*`
- `plans/archive/*` where archival is needed
- `.agents/skills/*`
- `images/controller/**/*`
- `docs/CONFIGURATION*`
- `docs/development/*` only if repo workflow docs need sync
- `api/*` and `crds/*` only if drift against current runtime/public contract is
  discovered

## Non-goals

- Не менять phase-1 `mlflow` runtime/deployment logic.
- Не вводить новый public API для `ai-inference` deployment mutation в этом же
  corrective bundle.
- Не переписывать сразу current `ModelPack` implementation adapter.
- Не делать node-local cache plane в этом workstream.
- Не удалять исторические archive bundles без явного archive placement.

## Затрагиваемые области

- `plans/active/`
- `plans/archive/`
- `.agents/skills/`
- `images/controller/`
- `docs/`
- potentially `api/` and `crds/` if wording/contract drift is found

## Критерии приёмки

- Есть явный inventory того, что было agreed in chat и как это отражено в repo.
- Runtime/materialization guidance во всех текущих active docs совпадает:
  - `OCI from registry`;
  - hidden backend under `DVCR`;
  - `ModelPack` as contract;
  - concrete implementations as adapters;
  - init-container/materializer gives runtime only local path.
- В `plans/active` больше нет overlapping bundles, которые описывают один и тот
  же workstream разными словами; stale bundles архивированы или явно помечены.
- Legacy wording и dead seams удалены из runtime/controller docs и repo memory.
- Следующий implementation path after cleanup становится однозначным и не
  конфликтует с hexagonal architecture discipline.

## Риски

- Случайно удалить bundle или doc, который ещё содержит уникальный сигнал.
- Недочистить active bundles и оставить скрытый duplicate source of truth.
- Увлечься только docs cleanup и не закрыть structural runtime drift.
- Попробовать делать новый feature slice до завершения corrective cleanup.
