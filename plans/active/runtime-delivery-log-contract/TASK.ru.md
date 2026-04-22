# Runtime delivery log contract cleanup

## 1. Заголовок

Сделать `workloaddelivery`-логи stateful и информативными вместо repeated
reconcile noise

## 2. Контекст

В живом кластере `ai-models-controller` сейчас пишет много одинаковых строк
вида:

- `runtime delivery applied`

для одного и того же workload/digest/model path. Эти строки приходят пачками
при обычном reconcile churn и не помогают оператору понять:

- впервые ли применена доставка;
- реально ли сменился runtime contract;
- идёт ли только внутренний reconcile вокруг projected secrets / template
  normalization.

Пользователь прямо указывает, что такой сигнал не соответствует ожидаемому
quality bar и просит привести его к более defendable pattern, как в
`virtualization`: лог должен отражать meaningful state change, а не каждый
внутренний reconcile.

## 3. Постановка задачи

Нужно поправить controller-owned log contract в
`images/controller/internal/controllers/workloaddelivery`:

1. Локализовать, почему текущий reconcile пишет повторяющиеся
   `runtime delivery applied` строки.
2. Разделить:
   - semantic runtime-delivery state change;
   - внутренний reconcile / housekeeping patch.
3. Оставить `info`-лог только для meaningful delivery transition.
4. Не оставлять рядом столь же шумный `Event` для того же repeated case.
5. Зафиксировать в тестах, что repeated reconcile с тем же workload-facing
   runtime state не порождает новый user-facing delivery signal.

## 4. Scope

- `plans/active/runtime-delivery-log-contract/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- при необходимости `images/controller/internal/adapters/k8s/modeldelivery/*`
- docs/evidence только если по факту нужен durable note

## 5. Non-goals

- не redesign всей runtime topology;
- не менять public API `Model` / `ClusterModel`;
- не переписывать весь logging style controller целиком;
- не решать сейчас node-cache metrics/alerts/dashboards;
- не менять live runtime semantics доставки модели в workload.

## 6. Затрагиваемые области

- controller runtime delivery reconcile path;
- controller log/event signal around workload delivery apply;
- focused controller tests for log/event dedupe semantics.

## 7. Критерии приёмки

- repeated reconcile with unchanged workload-facing delivery semantics больше не
  пишет repeated `info`-лог про applied delivery;
- repeated reconcile with unchanged delivery semantics не создаёт repeated
  `ModelDeliveryApplied` event;
- `info`-лог остаётся только для meaningful delivery transition:
  first apply, digest/mode/reason/artifact contract change или explicit removal;
- поведение покрыто focused tests в `workloaddelivery`;
- проходят релевантные узкие проверки по затронутым пакетам.

## 8. Риски

- можно спрятать слишком много сигнала и потерять useful operational log;
- можно дедуплицировать по внутренним полям, а не по workload-facing contract;
- можно оставить repeated Kubernetes Event, даже если лог уже убран.
