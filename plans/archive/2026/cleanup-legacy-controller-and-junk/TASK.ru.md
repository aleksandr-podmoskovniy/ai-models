# Cleanup Legacy Controller And Junk

## Контекст

После нескольких corrective и implementation slices в рабочем дереве остались:

- legacy phase-2 пакеты под `images/controller/internal/*`, которые больше не
  участвуют в текущем live path;
- старые worker/runtime helpers, вытеснённые новым `backend artifact plane`;
- явный generated junk, который не должен жить в модуле.

Параллельно current live path уже стабилизирован вокруг:

- `artifactbackend`
- `sourcepublishjob`
- `publicationoperation`
- `modelpublish`
- `modelcleanup`
- `publication`
- `runtimedelivery`
- `app`

Значит cleanup можно делать уже сейчас, но только по reachability-аудиту, а не
по названию каталогов.

## Постановка задачи

Удалить из репозитория мёртвые legacy каталоги и мусор, не ломая текущий
working phase-2 path `HuggingFace|HTTP -> backend artifact -> status`.

## Scope

- Оформить отдельный cleanup bundle.
- По read-only audit определить, какие каталоги под
  `images/controller/internal/*` реально больше не используются и не должны
  оставаться в active tree.
- Удалить только те legacy пакеты, которые не импортируются и не нужны
  следующему implementation slice.
- Удалить явный generated junk, не являющийся частью module workflow.
- Удалить low-risk completed incident/fix bundles, которые до сих пор висят в
  `plans/active/`, но не несут долгой архитектурной ценности и подпадают под
  правило мелких одноразовых bundles из `plans/README`.
- Обновить только те docs и active bundle refs, которые после удаления начнут
  указывать на уже несуществующие active surfaces.

## Non-goals

- Не чистить вслепую весь `plans/active/`; исторические bundles не удалять без
  отдельного решения.
- Не удалять design/reference bundles и содержательные implementation bundles,
  которые ещё используются как engineering context.
- Не рефакторить текущий live path.
- Не трогать public API shape, CRD и status contract.
- Не делать новый implementation slice по upload/auth/runtime agent.

## Затрагиваемые области

- `images/controller/internal/*`
- `images/backend/scripts/*` только если аудит покажет dead worker helper
- `.VSCodeCounter/*`
- точечные low-risk каталоги в `plans/active/*`
- `images/controller/README.md`
- `plans/active/rebaseline-publication-plane-to-backend-artifact-plane/*`
- при необходимости другие docs, если они ссылаются на удалённые active
  surfaces

## Критерии приёмки

- В `images/controller/internal` не остаются legacy пакеты, которые не
  используются current runtime, tests или следующими agreed slices.
- Из репозитория удалён явный generated junk, не являющийся частью module
  workflow.
- README/controller docs не перечисляют удалённые active surfaces как живые.
- `go test ./...` в `images/controller` проходит после cleanup.
- Repo-level проверки проходят.

## Риски

- Самый большой риск — удалить пакет, который сейчас не импортируется напрямую,
  но ещё нужен следующему bounded slice.
- Второй риск — смешать cleanup active code с удалением исторического контекста
  из `plans/active/` и потерять engineering trail.
