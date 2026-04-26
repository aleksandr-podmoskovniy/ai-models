## 1. Current phase

Этап 1, corrective operational hardening и baseline verification внутри
publication/runtime path.

## 2. Orchestration

`solo`

Причина:

- нужен быстрый read-write operational loop по живому кластеру без
  архитектурного распараллеливания;
- сравнительный анализ с `virtualization` здесь read-only и не требует
  delegation сам по себе;
- если baseline verification вскроет multi-boundary redesign, это станет уже
  следующим bundle или continuation slice.

## 3. Slices

### Slice 1. Снять живое состояние baseline

Цель:

- получить текущее состояние controller, `DMCR`, smoke objects и runtime path,
  а не общий рассказ про ошибки.

Проверки:

- `kubectl get pods,events,model,clustermodel,...`
- `kubectl logs ...`
- при наличии smoke workload проверить delivery signal

Артефакт результата:

- зафиксирован текущий live baseline status и конкретные рабочие/нерабочие
  точки.

### Slice 2. Проверить стандартный кейс `gemma 4`

Цель:

- подтвердить publication/runtime path на стандартном кейсе и понять, где
  именно разрыв, если end-to-end smoke не замыкается.

Проверки:

- `kubectl get/describe/logs` по relevant `Model`/`ClusterModel`/workload
- inspect `DMCR` state и artifact references

Артефакт результата:

- зафиксирован либо working smoke path, либо точный failing step.

### Slice 3. Проверить GC path для registry/S3

Цель:

- понять, что именно у нас сегодня делает cleanup, когда он запускается и
  проверяет ли модуль лишние объекты после старта.

Проверки:

- read-only code inspection `images/dmcr/**`, `images/controller/**`,
  `templates/**`, docs
- при наличии live signal проверить соответствующие логи/cronhook/job paths

Артефакт результата:

- GC contract описан по фактическому коду и live behavior.

### Slice 4. Сопоставить паттерны с `virtualization`

Цель:

- понять, где `ai-models` уже следует DKP module patterns, а где остаются
  несогласованные или ещё сыроватые поверхности.

Проверки:

- read-only comparison against `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`
- зафиксировать только concrete reusable patterns и deviations

Артефакт результата:

- список соответствий, расхождений и pragmatic next fixes.

### Slice 5. Собрать actionable gaps

Цель:

- превратить расследование в короткий список следующих implementation targets.

Проверки:

- bundle notes консистентны с live observations и repo code

Артефакт результата:

- сформирован список узких мест и рекомендуемый следующий шаг.

### Slice 6. Повторный live log pass по DMCR после RBAC hardening

Цель:

- проверить свежие ошибки `d8-ai-models` в кластере и исправить только
  подтверждённые runtime/template defects.

Проверки:

- `kubectl get pods -n d8-ai-models`;
- `kubectl logs ... -c dmcr-garbage-collection --previous`;
- `kubectl auth can-i ... --as=system:serviceaccount:d8-ai-models:dmcr`;
- `cd images/dmcr && go test -count=1 ./internal/directupload`;
- `make helm-template`;
- `make kubeconform`;
- rendered RBAC/probe checks.

Артефакт результата:

- DMCR service-account RBAC синхронизирован с GC runtime behavior;
- direct-upload probe log noise убран через HTTPS health endpoint;
- внешние cluster-wide проблемы отделены от module defects.

### Slice 7. Fresh tiny model publication smoke after module update

Цель:

- создать маленький namespaced `Model`, пройти source resolution,
  publication worker, upload/sealing в `DMCR` и проверить public status/artifact
  signal по каждому этапу.

Проверки:

- preflight `kubectl get pods -n d8-ai-models`;
- `kubectl apply` маленького `Model` в `ai-models-smoke`;
- watch `Model.status.phase`, `conditions`, `artifact`;
- inspect controller logs;
- inspect publication worker pod/logs;
- inspect `dmcr` logs for registry write/read and storage health;
- verify no GC/request storm and no fresh `FailedMount` side effects.

Артефакт результата:

- зафиксирован end-to-end результат: `Ready` с `status.artifact.uri` или
  точный failing stage с логами.

Результат:

- tiny namespaced `Model` in `models.ai.deckhouse.io/v1alpha1` reached
  `Ready`;
- source fetch, publish worker, DMCR registry readback and workload
  `MaterializeBridge` delivery are confirmed end-to-end;
- no fresh DMCR S3/time-skew/timeout/probe-error storm was observed during
  the smoke window;
- legacy `models.ai-models.deckhouse.io` CRD remains installed but has no
  smoke object.

## 4. Rollback point

После Slice 1 можно остановиться с чистым live diagnosis без дополнительных
code changes.

## 5. Final validation

- повторная проверка в кластере по live objects
- `git diff --check` по bundle notes
- узкие локальные проверки только если в ходе работы понадобятся code changes
