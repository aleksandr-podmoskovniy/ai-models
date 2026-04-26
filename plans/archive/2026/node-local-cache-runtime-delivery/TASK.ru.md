## 1. Заголовок

Доведение доставки моделей до промышленной одноцелевой схемы

## 2. Контекст

Для `ai-models` уже зафиксирован канонический путь публикации:

- модель публикуется controller-owned способом;
- результатом публикации становится неизменяемый артефакт во внутреннем
  `DMCR`;
- прикладной запуск не должен зависеть от исторического backend shell.

По части runtime-доставки сейчас есть два слоя, которые нельзя больше путать:

- текущий живой переходный путь: контроллер меняет шаблон запуска и
  раскладывает модель в `/data/modelcache` через `materialize-artifact`;
- целевой узловой слой: managed local storage, stable per-node runtime
  `Pod/PVC`, общий узловой digest store и общий mount модели в workload.

Проблема не в отсутствии задела, а в дрейфе постановки:

- текущий запасной путь и целевой быстрый путь описывались как будто это уже
  один и тот же контракт;
- bundle разросся и перестал быть компактной рабочей поверхностью;
- в обсуждениях смешались storage substrate, узловой кэш и доставка модели в
  прикладной объект.

Нужен один канонический active bundle, который жёстко фиксирует:

- что уже считается рабочим текущим состоянием;
- какой именно единственный целевой путь доводим;
- какими срезами идём к финальной схеме без возврата к старой каше.

## 3. Постановка задачи

Нужно довести runtime-доставку модели до простой и объяснимой одноцелевой
схемы.

Целевой смысл такой:

1. На ноде работает controller-owned узловой runtime plane.
2. Опубликованный артефакт подтягивается из `DMCR` в узловой общий кэш.
3. Прикладной объект получает общий mount этой модели из node-local cache
   plane.
4. Один и тот же digest переиспользуется несколькими workload'ами на одной
   ноде без отдельной полной раскладки в их собственные тома.

Текущий per-workload `materialize-artifact` путь допускается только как
временное текущее состояние до cutover и не должен описываться как второй
долгоживущий product mode.

При этом должны сохраняться четыре инварианта:

- канонический источник опубликованной модели — `DMCR`;
- прикладной код видит один и тот же стабильный runtime-контракт без знания
  внутренней topology;
- публичный `Model` / `ClusterModel` контракт не засоряется storage-specific
  деталями;
- смена внутренней runtime/distribution topology не должна ломать
  workload-facing контракт.

## 4. Scope

- переписать и зафиксировать канонический active bundle для этого workstream;
- явно разделить:
  - публикацию модели;
  - доставку модели в прикладной объект;
  - узловой общий кэш и mount service;
  - текущее переходное materialize-состояние;
- довести основной быстрый режим доставки через узловой общий кэш и общий
  mount;
- определить и реализовать cutover к единственной целевой topology;
- убрать остаточный дрифт терминов, полумёртвых seams и misleading docs вокруг
  `materialize-artifact`, узлового runtime plane и workload delivery;
- дотянуть недостающие сигналы наблюдаемости, тесты и эксплуатационную
  документацию по этой схеме.

## 5. Non-goals

- не трогать `vLLM`, `KubeRay` и исследовательские стендовые документы;
- не менять канонический publication flow во внутренний `DMCR`;
- не тащить storage topology в публичный `Model.spec` или `ClusterModel.spec`;
- не обещать в этом bundle уже готовое "прямое хранилище" на ноде;
- не открывать заново отдельный workstream по `DMZ` registry;
- не строить новый зеркальный per-node intent контракт, если достаточно live
  cluster truth;
- не закреплять per-workload materialization как второй постоянный supported
  mode после cutover.

## 6. Затрагиваемые области

- `plans/active/node-local-cache-runtime-delivery/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/controllers/nodecacheruntime/*`
- `images/controller/internal/adapters/k8s/nodecacheruntime/*`
- `images/controller/internal/nodecache/*`
- `images/controller/internal/monitoring/runtimehealth/*`
- `images/controller/internal/support/resourcenames/*`
- `images/controller/cmd/ai-models-artifact-runtime/*`
- `images/controller/cmd/ai-models-controller/*`
- `openapi/config-values.yaml`
- `openapi/values.yaml`
- `templates/_helpers.tpl`
- `templates/controller/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- В active bundle есть одна каноническая формулировка текущего состояния и
  одна каноническая формулировка цели; bundle больше не описывает целевую
  схему как двурежимную.
- Единственная целевая topology реализована как workload-facing использование
  узлового общего кэша, а не как повторная полная раскладка модели в каждый
  прикладной объект.
- Один и тот же опубликованный артефакт может переиспользоваться несколькими
  прикладными объектами на одной ноде без отдельной полной перезаливки в их
  собственные тома.
- Текущее переходное materialize-состояние не описывается как равноправный
  final mode и имеет явный план удаления после cutover.
- Прикладной код видит один и тот же стабильный контракт через
  `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST` и
  `AI_MODELS_MODEL_FAMILY`, без знания внутреннего layout узлового кэша.
- Узловой runtime plane остаётся controller-owned per-node `Pod/PVC`, а
  shared storage identity не теряется при restart runtime pod'а.
- Runtime plane продолжает опираться на live managed pod truth по текущей
  ноде; новый зеркальный per-node intent plane не появляется.
- После фиксации target picture документация и структура репозитория больше не
  утверждают, что целевая схема включает долгоживущий переходный режим.
- Тесты систематически покрывают:
  - переиспользование одной модели на одной ноде;
  - cutover к единственной topology;
  - отсутствие contract drift при изменении внутренней доставки;
  - базовую устойчивость узлового runtime plane.
- Перед завершением проходит `make verify`.

## 8. Риски

- легко снова смешать storage substrate и workload-facing semantics в одном
  пакете;
- можно случайно выдать текущий подготовительный узловой слой за уже готовую
  пользовательскую функцию;
- можно оставить временный materialize bridge жить слишком долго и снова
  превратить его во второй supported mode;
- можно начать протаскивать storage-specific knobs в публичный model contract;
- можно повторно развести bundle и live code по разным словарям и снова
  потерять каноническую картину.
