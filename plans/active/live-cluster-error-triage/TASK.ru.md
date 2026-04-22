# Live cluster baseline triage, smoke and gap analysis

## 1. Заголовок

Разобрать текущие ошибки `ai-models` в живом кластере, подтвердить рабочий
publication/runtime baseline на стандартном кейсе `gemma 4`, проверить текущий
GC path для `DMCR`/S3 и зафиксировать оставшиеся узкие места без
архитектурного дрейфа

## 2. Контекст

После серии изменений по zero-trust ingest, progress/status, runtime delivery
и quality-gate cleanup пользователь сообщает, что в кластере у `ai-models`
есть реальные ошибки. Первичный live failure уже локализован и исправлен через
module override, но этого недостаточно: теперь нужно подтвердить, что baseline
вообще проходит стандартный `gemma 4` flow через `DMCR`, понять, как именно
работает GC по S3/registry path, и собрать список remaining gaps относительно
ожидаемого phase-1 baseline и привычных DKP паттернов.

## 3. Постановка задачи

Нужно выполнить bounded operational baseline verification loop:

1. Снять текущее состояние `ai-models` в кластере:
   - поды;
   - события;
   - свежие логи controller/runtime;
   - проблемные `Model`/`ClusterModel`/workload объекты, если они есть;
   - наличие или отсутствие стандартного smoke-кейса для `gemma 4`.
2. Подтвердить стандартный publication/runtime path для `gemma 4`:
   - есть ли живой `Model`/`ClusterModel`;
   - опубликован ли OCI artifact в `DMCR`;
   - есть ли runtime delivery signal у workload.
3. Проверить фактическое поведение GC:
   - что удаляется из registry/S3;
   - когда это инициируется;
   - есть ли стартовый sweep лишних объектов или только controller-driven GC.
4. Сопоставить текущий baseline и helper/runtime/GC patterns с
   `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`.
5. Зафиксировать remaining gaps и узкие места, которые реально мешают
   двигаться по phase-1 цели.

## 4. Scope

- `plans/active/live-cluster-error-triage/*`
- live cluster inspection через `kubectl`
- кодовые области по фактической причине сбоя:
  - controller bootstrap/controllers
  - publication/runtime adapters
  - workload delivery/runtime delivery
  - dmcr garbage collection
  - related docs/notes только если это потребуется по месту
 - comparative read-only inspection репозитория `virtualization`

## 5. Non-goals

- не делать blanket cleanup всех старых открытых workstreams;
- не переписывать архитектуру целиком без доказанного живого сигнала;
- не менять public API без прямой необходимости;
- не превращать сравнительный анализ с `virtualization` в copying exercise;
- не чинить теоретические риски, которые не подтверждаются кластером или
  repo-local design.

## 6. Затрагиваемые области

- `plans/active/live-cluster-error-triage/*`
- live cluster objects в namespace `d8-ai-models` и связанных smoke namespaces
- `images/controller/**`
- `images/dmcr/**`
- `templates/**`
- `docs/CONFIGURATION*.md`
- read-only comparison against `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`

## 7. Критерии приёмки

- зафиксирован текущий live status `dmcr`, controller и relevant smoke objects;
- стандартный `gemma 4` path либо подтверждён end-to-end, либо локализован
  точный разрыв с конкретным сигналом;
- для GC описано фактическое поведение: trigger, scope и границы cleanup;
- сравнительный анализ с `virtualization` сведён к конкретным reusable
  паттернам и реальным расхождениям, а не к общим впечатлениям;
- bundle содержит actionable список remaining gaps, который можно брать в
  следующий implementation slice без повторного расследования.

## 8. Риски

- можно принять исторический smoke residue за текущий baseline;
- можно переоценить `virtualization` как прямой template, хотя boundary у
  модулей разная;
- можно смешать operational triage с roadmap planning и снова расползти scope.
