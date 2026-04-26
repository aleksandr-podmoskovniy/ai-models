# Notes

## Что зафиксировано

- `runtime delivery applied` spam рождался не только из-за coarse-grained
  `Info`/`Event`, но и из-за reconcile path, который раньше опирался только на
  cached object и патчил workload даже в cases, где live pod template уже был в
  нужном состоянии.
- Для controller-facing noise reduction добавлен отдельный
  `deliverySignalState`, который смотрит только на workload-facing runtime
  contract:
  - resolved digest;
  - resolved artifact URI/family;
  - runtime model path;
  - delivery mode/reason.
- Перед patch controller теперь:
  - читает live workload через `APIReader`;
  - пропускает patch, если live pod template уже совпадает с desired template;
  - подавляет `ModelDeliveryApplied` event и `info`-лог, если patch нужен только
    для internal drift, а workload-facing delivery contract не изменился.
- Поверх этого добавлены direct regression tests не только на `Event`, но и на
  сам log contract:
  - stale reconcile больше не увеличивает число `runtime delivery applied`;
  - meaningful digest transition пишет ровно один
    `runtime delivery changed` и несёт `previousDigest`.
- Отдельный focused test на `deliverySignalStateFromTemplate` теперь фиксирует,
  какие именно workload-facing поля участвуют в semantic dedupe.

## Что осталось по сигналу

- first apply по-прежнему пишет `runtime delivery applied`;
- meaningful transition пишет `runtime delivery changed` с `previous*` полями;
- stale duplicate reconcile и internal template drift больше не создают
  повторный `ModelDeliveryApplied`.

## Проверки

- `cd images/controller && go test ./internal/controllers/workloaddelivery`
- `cd images/controller && go test ./internal/adapters/k8s/modeldelivery`
- `cd images/controller && go test ./internal/controllers/workloaddelivery ./internal/adapters/k8s/modeldelivery -count=1`
- `git diff --check`
